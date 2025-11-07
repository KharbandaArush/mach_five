package broker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mach_five/trading-system/internal/config"
	"github.com/mach_five/trading-system/internal/logger"
	"github.com/mach_five/trading-system/internal/models"
)

// KiteBroker implements broker interface for Zerodha Kite Connect API
type KiteBroker struct {
	config      *config.Config
	logger      *logger.Logger
	apiKey      string
	accessToken string
	baseURL     string
	httpClient  *http.Client
	marketHours *MarketHours
}

// KiteOrderRequest represents the order request to Kite API
type KiteOrderRequest struct {
	Exchange        string `json:"exchange"`
	Tradingsymbol   string `json:"tradingsymbol"`
	TransactionType string `json:"transaction_type"` // BUY or SELL
	OrderType       string `json:"order_type"`      // MARKET, LIMIT, SL, SL-M
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

	return &KiteBroker{
		config:      cfg,
		logger:      log,
		apiKey:      cfg.Broker.APIKey,
		accessToken: cfg.Broker.APISecret, // Access token stored in APISecret field
		baseURL:     baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		marketHours: NewMarketHours(),
	}, nil
}

// ExecuteOrder executes an order via Kite Connect API
func (k *KiteBroker) ExecuteOrder(ctx context.Context, order models.Order) (models.ExecutionResult, error) {
	now := time.Now()
	useAMO := k.marketHours.ShouldUseAMO(now)

	if useAMO {
		nextOpen := k.marketHours.GetNextMarketOpenTime(now)
		k.logger.Info("Market is closed. Placing After Market Order (AMO) for order: %s. Will execute at: %s", 
			order.ID, nextOpen.Format("2006-01-02 15:04:05 IST"))
	} else {
		k.logger.Info("Market is open. Placing regular order: %s (Symbol: %s, Side: %s, Qty: %d)", 
			order.ID, order.Symbol, order.Side, order.Quantity)
	}

	// Parse symbol to get exchange and trading symbol
	// Expected format: "NSE:RELIANCE" or "BSE:RELIANCE" or just "RELIANCE" (defaults to NSE)
	exchange, tradingsymbol := k.parseSymbol(order.Symbol)
	
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
		Product:         "MIS", // Default to MIS (Intraday), can be configured
		Validity:        "DAY", // Default to DAY, can be configured
	}

	// Add price for LIMIT orders
	if orderType == "LIMIT" {
		kiteOrder.Price = order.Price
	}

	// Make API request (use AMO endpoint if market is closed)
	var result *KiteOrderResponse
	var err error
	if useAMO {
		result, err = k.placeAMOOrder(ctx, kiteOrder)
	} else {
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
		nextOpen := k.marketHours.GetNextMarketOpenTime(now)
		k.logger.Info("Kite AMO order placed successfully. Order ID: %s. Will execute at: %s", 
			result.Data.OrderID, nextOpen.Format("2006-01-02 15:04:05 IST"))
	} else {
		k.logger.Info("Kite order placed successfully. Order ID: %s", result.Data.OrderID)
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
	url := fmt.Sprintf("%s/oms/orders/regular", k.baseURL)

	// Marshal request body
	body, err := json.Marshal(orderReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal order request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Kite-Version", "3")
	req.Header.Set("Authorization", fmt.Sprintf("token %s:%s", k.apiKey, k.accessToken))

	// Make request
	resp, err := k.httpClient.Do(req)
	if err != nil {
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
		return nil, fmt.Errorf("kite API returned status %d: %s", resp.StatusCode, string(respBody))
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
	// Kite AMO orders use the same endpoint but with validity set to "DAY" and placed outside market hours
	// The API automatically recognizes it as AMO when placed outside market hours
	url := fmt.Sprintf("%s/oms/orders/regular", k.baseURL)

	// Ensure validity is DAY for AMO orders
	orderReq.Validity = "DAY"

	// Marshal request body
	body, err := json.Marshal(orderReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal AMO order request: %w", err)
	}

	k.logger.Debug("Placing AMO order: %s", string(body))

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create AMO order request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Kite-Version", "3")
	req.Header.Set("Authorization", fmt.Sprintf("token %s:%s", k.apiKey, k.accessToken))

	// Make request
	resp, err := k.httpClient.Do(req)
	if err != nil {
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
		return nil, fmt.Errorf("kite AMO API returned status %d: %s", resp.StatusCode, string(respBody))
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
	url := fmt.Sprintf("%s/oms/user/profile", k.baseURL)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("token %s:%s", k.apiKey, k.accessToken))
	req.Header.Set("X-Kite-Version", "3")

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return nil
}


