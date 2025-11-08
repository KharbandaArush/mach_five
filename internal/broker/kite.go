package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mach_five/trading-system/internal/config"
	"github.com/mach_five/trading-system/internal/logger"
	"github.com/mach_five/trading-system/internal/models"
)

// KiteBroker implements broker interface for Zerodha Kite Connect API
type KiteBroker struct {
	config        *config.Config
	logger        *logger.Logger
	apiKey        string
	accessToken   string
	refreshToken  string
	baseURL       string
	httpClient    *http.Client
	marketHours   *MarketHours
	tokenMutex    sync.RWMutex // Protects accessToken and refreshToken
	tokenExpiry   time.Time    // When the current access token expires
}

// KiteOrderRequest represents the order request to Kite API
type KiteOrderRequest struct {
	Exchange        string `json:"exchange"`
	Tradingsymbol   string `json:"tradingsymbol"`
	TransactionType string `json:"transaction_type"` // BUY or SELL
	OrderType       string `json:"order_type"`      // MARKET, LIMIT, SL, SL-M
	Variety         string `json:"variety,omitempty"` // regular, amo, co, bo, icebox
	Quantity        int    `json:"quantity"`
	Price           float64 `json:"price,omitempty"` // Required for LIMIT orders
	Product         string `json:"product"`        // MIS, CNC, NRML
	Validity        string `json:"validity"`        // DAY, IOC
}

// KiteOrderResponse represents the response from Kite API
type KiteOrderResponse struct {
	Status string `json:"status"`
	Data   struct {
		OrderID string `json:"order_id"`
	} `json:"data"`
	Message string `json:"message"`
}

// KiteTokenResponse represents the response from Kite token refresh API
type KiteTokenResponse struct {
	Status        string `json:"status"`
	Data          struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	} `json:"data"`
	Message string `json:"message"`
}

// NewKiteBroker creates a new Kite broker instance
func NewKiteBroker(cfg *config.Config, log *logger.Logger) (*KiteBroker, error) {
	if cfg.Broker.APIKey == "" {
		return nil, fmt.Errorf("Kite API key is required")
	}
	if cfg.Broker.APISecret == "" {
		return nil, fmt.Errorf("Kite access token is required (set as api_secret in config)")
	}

	baseURL := cfg.Broker.BaseURL
	if baseURL == "" {
		baseURL = "https://kite.zerodha.com" // Default to production
	}

	// Configure HTTP client with connection pooling for high-frequency requests (1ms polling)
	transport := &http.Transport{
		MaxIdleConns:        100,              // Maximum idle connections
		MaxIdleConnsPerHost: 10,              // Maximum idle connections per host
		IdleConnTimeout:     90 * time.Second, // How long idle connections are kept
		DisableKeepAlives:   false,           // Enable keep-alive connections
	}
	
	broker := &KiteBroker{
		config:       cfg,
		logger:       log,
		apiKey:       cfg.Broker.APIKey,
		accessToken:  cfg.Broker.APISecret, // Access token stored in APISecret field
		refreshToken: cfg.Broker.RefreshToken,
		baseURL:      baseURL,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
		marketHours: NewMarketHours(),
		tokenExpiry:  time.Now().Add(24 * time.Hour), // Default expiry (not used for auto-refresh)
	}

	// Token refresh is disabled - tokens must be manually updated via update-access-token.sh
	log.Info("📋 Using access token from broker-config.json")
	log.Info("   Token refresh is disabled - update manually if health checks fail")

	return broker, nil
}

// ExecuteOrder executes an order via Kite Connect API
func (k *KiteBroker) ExecuteOrder(ctx context.Context, order models.Order) (models.ExecutionResult, error) {
	// Use the AMO decision made at read time
	useAMO := order.IsAMO

	// Use exchange from order, or parse from symbol if not set
	var exchange, tradingsymbol string
	if order.Exchange != "" {
		// Use exchange from order
		exchange = order.Exchange
		tradingsymbol = strings.ToUpper(order.Symbol)
	} else {
		// Fallback to parsing from symbol format (for backward compatibility)
		exchange, tradingsymbol = k.parseSymbol(order.Symbol)
	}

	// Optimized logging - only log essential info, detailed logs only on DEBUG
	if useAMO {
		k.logger.Info("🌙 Placing AMO order: %s | %s:%s | %s %d @ %.2f", 
			order.ID, exchange, order.Symbol, order.Side, order.Quantity, order.Price)
		if k.logger.IsDebug() {
			nextOpen := k.marketHours.GetNextMarketOpenTime(order.ScheduledTime)
			k.logger.Debug("   📅 Scheduled: %s IST | ⏰ Executes: %s", 
				order.ScheduledTime.Format("2006-01-02 15:04:05 IST"),
				nextOpen.Format("2006-01-02 15:04:05 IST"))
		}
	} else {
		k.logger.Info("🌞 Placing order: %s | %s:%s | %s %d @ %.2f", 
			order.ID, exchange, order.Symbol, order.Side, order.Quantity, order.Price)
		if k.logger.IsDebug() {
			k.logger.Debug("   📅 Scheduled: %s IST", order.ScheduledTime.Format("2006-01-02 15:04:05 IST"))
		}
	}
	
	// Map order side
	transactionType := "BUY"
	if strings.ToUpper(order.Side) == "SELL" {
		transactionType = "SELL"
	}

	// Map order type
	orderType := "MARKET"
	if strings.ToUpper(order.OrderType) == "LIMIT" {
		orderType = "LIMIT"
	}

	// Build order request
	kiteOrder := KiteOrderRequest{
		Exchange:        exchange,
		Tradingsymbol:   tradingsymbol,
		TransactionType: transactionType,
		OrderType:       orderType,
		Quantity:        order.Quantity,
		Product:         "CNC", // CNC (Cash and Carry) for delivery-based trades
		Validity:        "DAY", // Default to DAY, can be configured
	}

	// Set variety to "amo" for After Market Orders
	if useAMO {
		kiteOrder.Variety = "amo"
	} else {
		kiteOrder.Variety = "regular"
	}

	// Add price for LIMIT orders
	if orderType == "LIMIT" {
		kiteOrder.Price = order.Price
	}

	// Make API request (use AMO endpoint if market is closed)
	var result *KiteOrderResponse
	var err error
	if useAMO {
		k.logger.Debug("🔀 Routing to AMO endpoint (order.IsAMO=true)")
		result, err = k.placeAMOOrder(ctx, kiteOrder)
	} else {
		k.logger.Debug("🔀 Routing to regular endpoint (order.IsAMO=false)")
		result, err = k.placeOrder(ctx, kiteOrder)
	}
	if err != nil {
		return models.ExecutionResult{
			OrderID:      order.ID,
			Success:      false,
			ExecutedAt:   time.Now(),
			ErrorMessage: err.Error(),
		}, err
	}

	// Check if order was successful
	if result.Status != "success" {
		errorMsg := result.Message
		if errorMsg == "" {
			errorMsg = "Order placement failed"
		}
		return models.ExecutionResult{
			OrderID:      order.ID,
			Success:      false,
			ExecutedAt:   time.Now(),
			ErrorMessage: errorMsg,
		}, fmt.Errorf("kite order failed: %s", errorMsg)
	}

	if useAMO {
		nextOpen := k.marketHours.GetNextMarketOpenTime(order.ScheduledTime)
		k.logger.Success("✅ Kite AMO order placed successfully")
		k.logger.Info("   📝 Kite Order ID: %s", result.Data.OrderID)
		k.logger.Info("   ⏰ Will execute at: %s", nextOpen.Format("2006-01-02 15:04:05 IST"))
	} else {
		k.logger.Success("✅ Kite order placed successfully")
		k.logger.Info("   📝 Kite Order ID: %s", result.Data.OrderID)
	}

	return models.ExecutionResult{
		OrderID:        order.ID,
		Success:        true,
		ExecutionID:    result.Data.OrderID,
		ExecutedAt:     time.Now(),
		ExecutedPrice:  order.Price, // For MARKET orders, this will be filled by Kite
		ExecutedQuantity: order.Quantity,
	}, nil
}

// placeOrder places an order via Kite Connect API
func (k *KiteBroker) placeOrder(ctx context.Context, orderReq KiteOrderRequest) (*KiteOrderResponse, error) {
	// Safety check: AMO orders should use placeAMOOrder, not placeOrder
	if orderReq.Variety == "amo" {
		k.logger.Warn("⚠️  AMO order detected in placeOrder - redirecting to placeAMOOrder")
		return k.placeAMOOrder(ctx, orderReq)
	}

	// Use api.kite.trade for regular orders (works with form-urlencoded)
	apiURL := "https://api.kite.trade/orders/regular"

	// Build form-urlencoded request body
	formData := url.Values{}
	formData.Set("exchange", orderReq.Exchange)
	formData.Set("tradingsymbol", orderReq.Tradingsymbol)
	formData.Set("transaction_type", orderReq.TransactionType)
	formData.Set("order_type", orderReq.OrderType)
	// Only set variety if it's not empty and not "amo" (amo should use AMO endpoint)
	if orderReq.Variety != "" && orderReq.Variety != "amo" {
		formData.Set("variety", orderReq.Variety)
	} else {
		formData.Set("variety", "regular")
	}
	formData.Set("quantity", fmt.Sprintf("%d", orderReq.Quantity))
	formData.Set("product", orderReq.Product)
	formData.Set("validity", orderReq.Validity)
	
	// Add price only for LIMIT orders
	if orderReq.OrderType == "LIMIT" && orderReq.Price > 0 {
		formData.Set("price", fmt.Sprintf("%.2f", orderReq.Price))
	}

	body := []byte(formData.Encode())

	// Create HTTP request with form data
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Get current access token (may trigger refresh)
	accessToken, err := k.getAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	// Set headers - use form-urlencoded for regular orders
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Kite-Version", "3")
	req.Header.Set("Authorization", fmt.Sprintf("token %s:%s", k.apiKey, accessToken))

	// Log request details only on DEBUG level (optimized for performance)
	if k.logger.IsDebug() {
		k.logRequestDetails(req, apiURL, body, "regular")
		k.logger.Debug("📤 Sending regular order request to Kite: %s", apiURL)
		k.logger.Debug("   Order request (form-data): %s", formData.Encode())
	}
	
	// Make request (optimized - no logging in hot path)
	resp, err := k.httpClient.Do(req)
	if err != nil {
		k.logger.Error("❌ Network error during order placement: %v", err)
		k.logger.Error("   URL: %s", apiURL)
		k.logger.Error("   Request Body: %s", formData.Encode())
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		errorMsg := string(respBody)
		
		// Log auth errors - token refresh is disabled, must be updated manually
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			k.logger.Error("❌ Authentication failed - access token may be expired")
			k.logger.Error("   Use update-access-token.sh to update the token manually")
		}
		
		k.logger.Error("❌ Kite order placement failed")
		k.logger.Error("   Status Code: %d", resp.StatusCode)
		k.logger.Error("   URL: %s", apiURL)
		k.logger.Error("   Request Body: %s", formData.Encode())
		k.logger.Error("   Response: %s", errorMsg)
		return nil, fmt.Errorf("kite API returned status %d: %s", resp.StatusCode, errorMsg)
	}

	// Parse response
	var kiteResp KiteOrderResponse
	if err := json.Unmarshal(respBody, &kiteResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &kiteResp, nil
}

// placeAMOOrder places an After Market Order via Kite Connect API
func (k *KiteBroker) placeAMOOrder(ctx context.Context, orderReq KiteOrderRequest) (*KiteOrderResponse, error) {
	// Kite AMO orders use the dedicated AMO endpoint at api.kite.trade
	// Use api.kite.trade for AMO orders (works with form-urlencoded)
	amoURL := "https://api.kite.trade/orders/amo"

	// Ensure validity is DAY for AMO orders
	orderReq.Validity = "DAY"

	// Build form-urlencoded request body
	formData := url.Values{}
	formData.Set("exchange", orderReq.Exchange)
	formData.Set("tradingsymbol", orderReq.Tradingsymbol)
	formData.Set("transaction_type", orderReq.TransactionType)
	formData.Set("order_type", orderReq.OrderType)
	formData.Set("variety", "amo")
	formData.Set("quantity", fmt.Sprintf("%d", orderReq.Quantity))
	formData.Set("product", orderReq.Product)
	formData.Set("validity", orderReq.Validity)
	
	// Add price only for LIMIT orders
	if orderReq.OrderType == "LIMIT" && orderReq.Price > 0 {
		formData.Set("price", fmt.Sprintf("%.2f", orderReq.Price))
	}

	body := []byte(formData.Encode())

	k.logger.Info("📋 AMO Order Request (form-data):")
	k.logger.Info("   %s", formData.Encode())

	// Create HTTP request with form data
	req, err := http.NewRequestWithContext(ctx, "POST", amoURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create AMO order request: %w", err)
	}

	// Get current access token (may trigger refresh)
	accessToken, err := k.getAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	// Set headers - use form-urlencoded for AMO orders
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Kite-Version", "3")
	req.Header.Set("Authorization", fmt.Sprintf("token %s:%s", k.apiKey, accessToken))

	// Log request details only on DEBUG level (optimized for performance)
	if k.logger.IsDebug() {
		k.logRequestDetails(req, amoURL, body, "AMO")
		k.logger.Debug("📤 Sending AMO order request to Kite: %s", amoURL)
	}
	
	// Make request (optimized - no logging in hot path)
	resp, err := k.httpClient.Do(req)
	if err != nil {
		k.logger.Error("❌ Network error during AMO order placement: %v", err)
		k.logger.Error("   URL: %s", amoURL)
		k.logger.Error("   Request Body: %s", formData.Encode())
		return nil, fmt.Errorf("failed to execute AMO order request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read AMO order response: %w", err)
	}

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		errorMsg := string(respBody)
		
		// Log auth errors - token refresh is disabled, must be updated manually
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			k.logger.Error("❌ Authentication failed - access token may be expired")
			k.logger.Error("   Use update-access-token.sh to update the token manually")
		}
		
		k.logger.Error("❌ Kite AMO order placement failed")
		k.logger.Error("   Status Code: %d", resp.StatusCode)
		k.logger.Error("   URL: %s", amoURL)
		k.logger.Error("   Request Body: %s", formData.Encode())
		k.logger.Error("   Response: %s", errorMsg)
		return nil, fmt.Errorf("kite AMO API returned status %d: %s", resp.StatusCode, errorMsg)
	}

	// Parse response
	var kiteResp KiteOrderResponse
	if err := json.Unmarshal(respBody, &kiteResp); err != nil {
		return nil, fmt.Errorf("failed to parse AMO order response: %w", err)
	}

	return &kiteResp, nil
}

// parseSymbol parses symbol to extract exchange and trading symbol
// Supports formats: "NSE:RELIANCE", "BSE:RELIANCE", "RELIANCE" (defaults to NSE)
func (k *KiteBroker) parseSymbol(symbol string) (string, string) {
	parts := strings.Split(symbol, ":")
	if len(parts) == 2 {
		// Format: "NSE:RELIANCE"
		return strings.ToUpper(parts[0]), strings.ToUpper(parts[1])
	}
	// Default to NSE if no exchange specified
	return "NSE", strings.ToUpper(symbol)
}

// HealthCheck checks Kite API health
func (k *KiteBroker) HealthCheck(ctx context.Context) error {
	// Check user profile as health check
	// Use api.kite.trade without /oms prefix (tested and working)
	url := "https://api.kite.trade/user/profile"
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		k.logger.Error("❌ Failed to create health check request: %v", err)
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	// Get current access token (may trigger refresh)
	accessToken, err := k.getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("token %s:%s", k.apiKey, accessToken))
	req.Header.Set("X-Kite-Version", "3")

	// Don't log request details for health checks (only log on failure)
	resp, err := k.httpClient.Do(req)
	if err != nil {
		k.logger.Error("❌ Health check network error: %v", err)
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body for detailed error logging
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		k.logger.Warn("⚠️  Failed to read health check response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		errorDetails := string(respBody)
		if errorDetails == "" {
			errorDetails = "No error message in response"
		}
		
		// Don't retry health checks - just log the error and return
		// Health checks should be simple and not cause cascading retries
		k.logger.Error("❌ Kite health check failed")
		k.logger.Error("   Status Code: %d", resp.StatusCode)
		k.logger.Error("   URL: %s", url)
		k.logger.Error("   Response: %s", errorDetails)
		k.logger.Error("   API Key: %s (first 10 chars)", k.apiKey[:min(10, len(k.apiKey))])
		k.tokenMutex.RLock()
		tokenPreview := k.accessToken[:min(10, len(k.accessToken))]
		k.tokenMutex.RUnlock()
		k.logger.Error("   Access Token: %s (first 10 chars)", tokenPreview)
		
		return fmt.Errorf("health check returned status %d: %s", resp.StatusCode, errorDetails)
	}

	// Health check passed - silently return (only log errors)
	return nil
}

// ValidateSymbol validates if a symbol exists in Zerodha
func (k *KiteBroker) ValidateSymbol(ctx context.Context, exchange, symbol string) (bool, error) {
	// Use Kite API quote endpoint to validate symbol
	// Format: https://api.kite.trade/quote/ltp?i=EXCHANGE:SYMBOL
	// Example: https://api.kite.trade/quote/ltp?i=NSE:RELIANCE
	
	// Normalize exchange and symbol
	exchange = strings.ToUpper(exchange)
	symbol = strings.ToUpper(symbol)
	
	// Build URL with instrument identifier
	instrumentID := fmt.Sprintf("%s:%s", exchange, symbol)
	quoteURL := fmt.Sprintf("https://api.kite.trade/quote/ltp?i=%s", url.QueryEscape(instrumentID))
	
	req, err := http.NewRequestWithContext(ctx, "GET", quoteURL, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create validation request: %w", err)
	}
	
	// Get current access token
	accessToken, err := k.getAccessToken(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get access token: %w", err)
	}
	
	req.Header.Set("Authorization", fmt.Sprintf("token %s:%s", k.apiKey, accessToken))
	req.Header.Set("X-Kite-Version", "3")
	
	// Make request
	resp, err := k.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to validate symbol: %w", err)
	}
	defer resp.Body.Close()
	
	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read validation response: %w", err)
	}
	
	// Check if symbol is valid (200 OK means symbol exists)
	if resp.StatusCode == http.StatusOK {
		// Parse response to ensure it's valid JSON with data
		var quoteResp map[string]interface{}
		if err := json.Unmarshal(respBody, &quoteResp); err != nil {
			return false, fmt.Errorf("invalid response format: %w", err)
		}
		
		// Check if status is success and data exists
		if status, ok := quoteResp["status"].(string); ok && status == "success" {
			if data, ok := quoteResp["data"].(map[string]interface{}); ok && len(data) > 0 {
				return true, nil
			}
		}
		return false, fmt.Errorf("symbol not found in response")
	}
	
	// 404 means symbol doesn't exist
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	
	// 403 means insufficient permissions - treat as validation unavailable (not invalid)
	// This allows orders through if we can't validate due to permissions
	if resp.StatusCode == http.StatusForbidden {
		errorMsg := string(respBody)
		return true, fmt.Errorf("validation unavailable (insufficient permissions): %s", errorMsg)
	}
	
	// Other errors - treat as validation error but don't fail the order
	errorMsg := string(respBody)
	return true, fmt.Errorf("validation failed with status %d: %s", resp.StatusCode, errorMsg)
}

// refreshAccessToken refreshes the access token using the refresh token
func (k *KiteBroker) refreshAccessToken(ctx context.Context) error {
	k.tokenMutex.Lock()
	defer k.tokenMutex.Unlock()

	if k.refreshToken == "" {
		return fmt.Errorf("refresh token not available")
	}

	k.logger.Info("🔄 Refreshing Kite access token...")

	// Kite Connect token refresh endpoint
	refreshURL := fmt.Sprintf("%s/session/refresh_token", k.baseURL)

	// Prepare form data
	data := url.Values{}
	data.Set("refresh_token", k.refreshToken)
	data.Set("api_key", k.apiKey)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", refreshURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create refresh request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Kite-Version", "3")

	// Log request details for token refresh
	k.logRequestDetails(req, refreshURL, []byte(data.Encode()), "token refresh")

	// Make request
	resp, err := k.httpClient.Do(req)
	if err != nil {
		k.logger.Error("❌ Network error during token refresh: %v", err)
		return fmt.Errorf("failed to refresh token: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read refresh response: %w", err)
	}

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		errorMsg := string(respBody)
		k.logger.Error("❌ Token refresh failed")
		k.logger.Error("   Status Code: %d", resp.StatusCode)
		k.logger.Error("   Response: %s", errorMsg)
		return fmt.Errorf("token refresh returned status %d: %s", resp.StatusCode, errorMsg)
	}

	// Parse response
	var tokenResp KiteTokenResponse
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return fmt.Errorf("failed to parse refresh response: %w", err)
	}

	if tokenResp.Status != "success" {
		return fmt.Errorf("token refresh failed: %s", tokenResp.Message)
	}

	// Update tokens
	oldToken := k.accessToken[:min(10, len(k.accessToken))]
	k.accessToken = tokenResp.Data.AccessToken
	if tokenResp.Data.RefreshToken != "" {
		k.refreshToken = tokenResp.Data.RefreshToken
	}
	k.tokenExpiry = time.Now().Add(24 * time.Hour) // Kite tokens typically last 24 hours

	k.logger.Success("✅ Token refreshed successfully")
	k.logger.Debug("   Old token: %s...", oldToken)
	k.logger.Debug("   New token: %s...", k.accessToken[:min(10, len(k.accessToken))])
	k.logger.Debug("   Expires at: %s", k.tokenExpiry.Format("2006-01-02 15:04:05 IST"))

	// Update config file if path is available
	if k.config.Broker.ConfigPath != "" {
		if err := k.updateConfigFile(); err != nil {
			k.logger.Warn("⚠️  Failed to update config file with new token: %v", err)
			k.logger.Warn("   Token is updated in memory but not persisted")
		}
	}

	return nil
}

// updateConfigFile updates the broker config file with new tokens
func (k *KiteBroker) updateConfigFile() error {
	// Read existing config
	data, err := os.ReadFile(k.config.Broker.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var fileConfig map[string]interface{}
	if err := json.Unmarshal(data, &fileConfig); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// Update tokens
	fileConfig["api_secret"] = k.accessToken
	if k.refreshToken != "" {
		fileConfig["refresh_token"] = k.refreshToken
	}

	// Write back
	updatedData, err := json.MarshalIndent(fileConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(k.config.Broker.ConfigPath, updatedData, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// getAccessToken returns the current access token from config
// Token refresh is disabled - tokens must be manually updated via update-access-token.sh
func (k *KiteBroker) getAccessToken(ctx context.Context) (string, error) {
	k.tokenMutex.RLock()
	defer k.tokenMutex.RUnlock()
	
	if k.accessToken == "" {
		return "", fmt.Errorf("access token not configured")
	}
	
	return k.accessToken, nil
}

// NOTE: Token refresh functions removed - token refresh is now manual only
// Use update-access-token.sh script to update tokens when health checks fail

// logRequestDetails logs complete request details including all headers
func (k *KiteBroker) logRequestDetails(req *http.Request, url string, body []byte, orderType string) {
	k.logger.Section(fmt.Sprintf("📋 HTTP Request Details (%s order)", orderType))
	
	// Log URL and method
	k.logger.Info("   Method: %s", req.Method)
	k.logger.Info("   URL: %s", url)
	
	// Log all headers
	k.logger.Info("   Headers:")
	for name, values := range req.Header {
		for _, value := range values {
			// Mask sensitive authorization header
			if name == "Authorization" {
				// Show first part (token) and mask the access token
				parts := strings.Split(value, ":")
				if len(parts) == 2 {
					maskedToken := maskString(parts[1], 4)
					k.logger.Info("      %s: %s:%s", name, parts[0], maskedToken)
				} else {
					maskedValue := maskString(value, 4)
					k.logger.Info("      %s: %s", name, maskedValue)
				}
			} else {
				k.logger.Info("      %s: %s", name, value)
			}
		}
	}
	
	// Log request body
	if len(body) > 0 {
		k.logger.Info("   Request Body:")
		k.logger.Info("      %s", string(body))
	}
}

// maskString masks a string showing only first and last few characters
func maskString(s string, visibleChars int) string {
	if len(s) <= visibleChars*2 {
		return strings.Repeat("*", len(s))
	}
	return s[:visibleChars] + strings.Repeat("*", len(s)-visibleChars*2) + s[len(s)-visibleChars:]
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}


