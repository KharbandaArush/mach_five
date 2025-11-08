package reader

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mach_five/trading-system/internal/cache"
	"github.com/mach_five/trading-system/internal/config"
	"github.com/mach_five/trading-system/internal/logger"
	"github.com/mach_five/trading-system/internal/models"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

// SheetsReader reads orders from Google Sheets
type SheetsReader struct {
	config  *config.Config
	cache   *cache.RedisCache
	logger  *logger.Logger
	service *sheets.Service
	sheetID string
}

// NewSheetsReader creates a new Google Sheets reader
func NewSheetsReader(cfg *config.Config, cache *cache.RedisCache, log *logger.Logger) (*SheetsReader, error) {
	ctx := context.Background()

	// Load credentials
	credentialsPath := cfg.GoogleSheets.CredentialsPath
	log.Debug("📂 Loading Google credentials from: %s", credentialsPath)
	
	credData, err := os.ReadFile(credentialsPath)
	if err != nil {
		log.Error("❌ Failed to read Google credentials file")
		log.Error("   Path: %s", credentialsPath)
		log.Error("   Error: %v", err)
		log.Error("   💡 Check if file exists and is readable")
		return nil, fmt.Errorf("failed to read credentials file: %w", err)
	}

	// Parse credentials
	log.Debug("🔐 Parsing Google credentials JSON")
	creds, err := google.CredentialsFromJSON(ctx, credData, sheets.SpreadsheetsReadonlyScope)
	if err != nil {
		log.Error("❌ Failed to parse Google credentials")
		log.Error("   Path: %s", credentialsPath)
		log.Error("   Error: %v", err)
		log.Error("   💡 Credentials file may be corrupted or invalid")
		log.Error("   💡 Re-download from Google Cloud Console if needed")
		return nil, fmt.Errorf("failed to parse credentials: %w", err)
	}

	// Create sheets service
	log.Debug("🔗 Creating Google Sheets API client")
	srv, err := sheets.NewService(ctx, option.WithCredentials(creds))
	if err != nil {
		log.Error("❌ Failed to create Google Sheets service")
		log.Error("   Error: %v", err)
		log.Error("   💡 Check network connectivity and API access")
		return nil, fmt.Errorf("failed to create sheets service: %w", err)
	}
	
	log.Debug("✅ Google Sheets service created successfully")

	// Validate sheet ID
	sheetID := cfg.GoogleSheets.SheetID
	if sheetID == "" {
		log.Error("❌ Google Sheet ID is not set")
		log.Error("   Set GOOGLE_SHEET_ID environment variable or in config")
		return nil, fmt.Errorf("Google Sheet ID is required")
	}
	
	log.Debug("📊 Using Google Sheet ID: %s", sheetID)

	return &SheetsReader{
		config:  cfg,
		cache:   cache,
		logger:  log,
		service: srv,
		sheetID: sheetID,
	}, nil
}

// Start starts the reader service (runs continuously)
func (r *SheetsReader) Start(ctx context.Context) error {
	r.logger.Info("Starting Google Sheets reader service")
	
	ticker := time.NewTicker(r.config.GoogleSheets.RefreshInterval)
	defer ticker.Stop()

	// Initial read
	if err := r.readAndCacheOrders(); err != nil {
		r.logger.Error("Initial read failed: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("Stopping Google Sheets reader service")
			return ctx.Err()
		case <-ticker.C:
			if err := r.readAndCacheOrders(); err != nil {
				r.logger.Error("Failed to read orders: %v", err)
				// Retry with 0 delay as per requirements
				time.Sleep(0)
				if err := r.readAndCacheOrders(); err != nil {
					r.logger.Error("Retry failed: %v", err)
				}
			}
		}
	}
}

// readAndCacheOrders reads orders from both sheets and caches them
func (r *SheetsReader) readAndCacheOrders() error {
	var allOrders []models.Order

	// Read buy orders from to_buy sheet
	r.logger.Debug("Reading buy orders from sheet: %s, range: %s", r.sheetID, r.config.GoogleSheets.BuyRange)
	buyOrders, err := r.readSheet(r.config.GoogleSheets.BuyRange, "Buy")
	if err != nil {
		r.logger.Error("❌ Failed to read buy orders: %v", err)
		r.logger.Error("   Sheet ID: %s", r.sheetID)
		r.logger.Error("   Range: %s", r.config.GoogleSheets.BuyRange)
		r.logger.Error("   Full error details logged above")
	} else {
		r.logger.Success("✅ Read %d buy orders from to_buy sheet", len(buyOrders))
		allOrders = append(allOrders, buyOrders...)
	}

	// Read sell orders from to_sell sheet
	r.logger.Debug("Reading sell orders from sheet: %s, range: %s", r.sheetID, r.config.GoogleSheets.SellRange)
	sellOrders, err := r.readSheet(r.config.GoogleSheets.SellRange, "Sell")
	if err != nil {
		r.logger.Error("❌ Failed to read sell orders: %v", err)
		r.logger.Error("   Sheet ID: %s", r.sheetID)
		r.logger.Error("   Range: %s", r.config.GoogleSheets.SellRange)
		r.logger.Error("   Full error details logged above")
	} else {
		r.logger.Success("✅ Read %d sell orders from to_sell sheet", len(sellOrders))
		allOrders = append(allOrders, sellOrders...)
	}

	// Log summary in table format
	r.logger.Section("📊 Order Reading Summary")
	r.logger.TableSimple("Orders Read from Google Sheets", map[string]string{
		"📈 Buy Orders":  fmt.Sprintf("%d", len(buyOrders)),
		"📉 Sell Orders": fmt.Sprintf("%d", len(sellOrders)),
		"📦 Total Orders": fmt.Sprintf("%d", len(allOrders)),
		"🕐 Timestamp":   time.Now().Format("2006-01-02 15:04:05 IST"),
	})

	// Cache all orders
	for _, order := range allOrders {
		expiryTime := order.ScheduledTime.Add(10 * time.Second)
		if err := r.cache.StoreOrder(order, expiryTime); err != nil {
			r.logger.Error("Failed to cache order %s: %v", order.ID, err)
			continue
		}
		amoStatus := "Regular"
		if order.IsAMO {
			amoStatus = "AMO"
		}
		r.logger.Debug("Cached order: %s, side: %s, exchange: %s, symbol: %s, scheduled: %s, type: %s, expiry: %s", 
			order.ID, order.Side, order.Exchange, order.Symbol, order.ScheduledTime.Format(time.RFC3339), amoStatus, expiryTime.Format(time.RFC3339))
	}

	return nil
}

// readSheet reads orders from a specific sheet range
func (r *SheetsReader) readSheet(rangeStr, side string) ([]models.Order, error) {
	r.logger.Debug("📖 Reading %s orders from sheet: %s, range: %s", side, r.sheetID, rangeStr)
	
	resp, err := r.service.Spreadsheets.Values.Get(r.sheetID, rangeStr).Do()
	if err != nil {
		r.logger.Error("❌ Failed to read %s orders from Google Sheets", side)
		r.logger.Error("   Sheet ID: %s", r.sheetID)
		r.logger.Error("   Range: %s", rangeStr)
		r.logger.Error("   Error: %v", err)
		
		// Try to extract more details from the error
		if strings.Contains(err.Error(), "404") {
			r.logger.Error("   💡 This is a 404 error - possible causes:")
			r.logger.Error("      - Sheet tab '%s' does not exist", strings.Split(rangeStr, "!")[0])
			r.logger.Error("      - Sheet ID is incorrect")
			r.logger.Error("      - Service account doesn't have access to the sheet")
		} else if strings.Contains(err.Error(), "403") {
			r.logger.Error("   💡 This is a 403 error - permission denied:")
			r.logger.Error("      - Service account needs access to the sheet")
			r.logger.Error("      - Check sharing permissions in Google Sheets")
		} else if strings.Contains(err.Error(), "401") {
			r.logger.Error("   💡 This is a 401 error - authentication failed:")
			r.logger.Error("      - Google credentials may be invalid or expired")
			r.logger.Error("      - Check google-credentials.json file")
		}
		
		return nil, fmt.Errorf("failed to read sheet range %s: %w", rangeStr, err)
	}

	if len(resp.Values) == 0 {
		r.logger.Debug("No data found in range %s", rangeStr)
		return []models.Order{}, nil
	}

	r.logger.Debug("Found %d rows in %s sheet", len(resp.Values), side)
	orders, err := r.parseRows(resp.Values, side)
	if err != nil {
		return nil, fmt.Errorf("failed to parse rows from %s: %w", rangeStr, err)
	}

	r.logger.Debug("Parsed %d valid orders from %d rows in %s sheet", len(orders), len(resp.Values), side)
	return orders, nil
}

// parseRows parses sheet rows into Order objects
// Column mapping (B through L):
// B: planned_buy_price (float) - Price
// C: product (string) - Product type
// D: Name (string) - Stock name
// E: bse_code (string) - BSE code
// F: symbol (string) - Trading symbol
// G: execute_date (string) - Date (YYYY-MM-DD)
// H: execute_time (string) - Time (HH:MM:SS or HH:MM)
// I: Money Needed (float) - Money required (used to calculate quantity if quantity column not present)
// J: Lots (int) - Number of orders to place
// K: exchange (string) - Exchange (NSE, BSE, etc.)
// L: quantity (int, optional) - Total quantity to distribute across lots
// Note: If lots > 1, total quantity (q) is distributed as: floor(q/n) base quantity,
//       with mod(q/n) orders getting floor(q/n) + 1 to ensure total quantity is used
func (r *SheetsReader) parseRows(rows [][]interface{}, side string) ([]models.Order, error) {
	var orders []models.Order
	// Get current time - we'll use IST for comparison
	istLocation, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		r.logger.Warn("Failed to load IST timezone: %v", err)
		istLocation = time.UTC
	}
	now := time.Now().In(istLocation)

	for i, row := range rows {
		// Need at least 10 columns (B through K, indexed 0-9)
		if len(row) < 10 {
			r.logger.Warn("Row %d has insufficient columns (%d), need at least 10 (B-K), skipping", i+3, len(row))
			continue
		}

		// Column B (index 0): planned_buy_price (or planned_sell_price for sell orders)
		priceStr := strings.TrimSpace(fmt.Sprintf("%v", row[0]))
		if priceStr == "" {
			r.logger.Debug("Row %d (%s): empty price, skipping", i+3, side)
			continue
		}
		// Skip if it's a header row (contains "price" text)
		if strings.Contains(strings.ToLower(priceStr), "price") {
			r.logger.Debug("Row %d (%s): appears to be header row, skipping", i+3, side)
			continue
		}
		price, err := strconv.ParseFloat(priceStr, 64)
		if err != nil {
			r.logger.Warn("Row %d (%s): invalid price '%s', skipping", i+3, side, priceStr)
			continue
		}

		// Column C (index 1): product - not used directly but logged
		product := strings.TrimSpace(fmt.Sprintf("%v", row[1]))

		// Column D (index 2): Name - not used directly but logged
		name := strings.TrimSpace(fmt.Sprintf("%v", row[2]))

		// Column E (index 3): bse_code - not used directly but logged
		bseCode := strings.TrimSpace(fmt.Sprintf("%v", row[3]))

		// Column F (index 4): symbol
		symbol := strings.TrimSpace(fmt.Sprintf("%v", row[4]))
		if symbol == "" {
			r.logger.Warn("Row %d: empty symbol, skipping", i+3)
			continue
		}

		// Column G (index 5): execute_date
		dateStr := strings.TrimSpace(fmt.Sprintf("%v", row[5]))
		// Try multiple date formats
		var date time.Time
		dateFormats := []string{"2006-01-02", "02-Jan-2006", "02-January-2006", "2006/01/02", "02/01/2006"}
		parsed := false
		for _, format := range dateFormats {
			if parsedDate, parseErr := time.Parse(format, dateStr); parseErr == nil {
				date = parsedDate
				parsed = true
				break
			}
		}
		if !parsed {
			r.logger.Warn("Row %d: invalid date '%s' (tried formats: YYYY-MM-DD, DD-Mon-YYYY, DD-Month-YYYY), skipping", i+3, dateStr)
			continue
		}

		// Column H (index 6): execute_time
		timeStr := strings.TrimSpace(fmt.Sprintf("%v", row[6]))
		var t time.Time
		// Try different time formats
		timeFormats := []string{"15:04:05", "15:04", "3:04 PM", "15:04:05 PM"}
		timeParsed := false
		for _, format := range timeFormats {
			if parsedTime, timeErr := time.Parse(format, timeStr); timeErr == nil {
				t = parsedTime
				timeParsed = true
				break
			}
		}
		if !timeParsed {
			r.logger.Warn("Row %d: invalid time '%s', skipping", i+3, timeStr)
			continue
		}

		// Column I (index 7): Money Needed - used to calculate total quantity if quantity column not present
		moneyNeededStr := strings.TrimSpace(fmt.Sprintf("%v", row[7]))
		moneyNeeded, _ := strconv.ParseFloat(moneyNeededStr, 64)

		// Column J (index 8): Lots (number of orders to place)
		lotsStr := strings.TrimSpace(fmt.Sprintf("%v", row[8]))
		lots, lotsErr := strconv.Atoi(lotsStr)
		if lotsErr != nil || lots <= 0 {
			r.logger.Warn("Row %d: invalid lots '%s', defaulting to 1", i+3, lotsStr)
			lots = 1
		}
		
		// Calculate total quantity (q)
		// If there's a quantity column (index 10), use it; otherwise calculate from Money Needed / Price
		var totalQuantity int
		if len(row) > 10 {
			// Column L (index 10): Quantity (if present)
			quantityStr := strings.TrimSpace(fmt.Sprintf("%v", row[10]))
			if quantityStr != "" {
				if qty, err := strconv.Atoi(quantityStr); err == nil && qty > 0 {
					totalQuantity = qty
				} else {
					// Invalid quantity, calculate from Money Needed / Price
					if price > 0 {
						totalQuantity = int(moneyNeeded / price)
					} else {
						totalQuantity = 1 // Default if price is 0
					}
				}
			} else {
				// No quantity column, calculate from Money Needed / Price
				if price > 0 {
					totalQuantity = int(moneyNeeded / price)
				} else {
					totalQuantity = 1 // Default if price is 0
				}
			}
		} else {
			// No quantity column, calculate from Money Needed / Price
			if price > 0 {
				totalQuantity = int(moneyNeeded / price)
			} else {
				totalQuantity = 1 // Default if price is 0
			}
		}
		
		// Ensure totalQuantity is at least 1
		if totalQuantity <= 0 {
			totalQuantity = 1
		}
		
		// Calculate quantity distribution across lots
		// floor(q/n) for base quantity, mod(q/n) orders get +1
		baseQuantity := totalQuantity / lots
		remainder := totalQuantity % lots

		// Column K (index 9): exchange
		exchange := strings.TrimSpace(fmt.Sprintf("%v", row[9]))
		if exchange == "" {
			r.logger.Debug("Row %d: empty exchange, defaulting to NSE", i+3)
			exchange = "NSE" // Default to NSE if not specified
		}
		// Normalize exchange to uppercase
		exchange = strings.ToUpper(exchange)

		// Load IST timezone (Asia/Kolkata)
		istLocation, err := time.LoadLocation("Asia/Kolkata")
		if err != nil {
			r.logger.Warn("Failed to load IST timezone, using UTC: %v", err)
			istLocation = time.UTC
		}

		// Combine date and time in IST timezone
		// Google Sheet times are in IST, so we interpret them as IST
		scheduledTime := time.Date(
			date.Year(), date.Month(), date.Day(),
			t.Hour(), t.Minute(), t.Second(), 0,
			istLocation,
		)

		// Get current time in IST for comparison
		nowIST := time.Now().In(istLocation)

		// Skip if scheduled time is in the past (in IST)
		if scheduledTime.Before(nowIST) {
			r.logger.Debug("Row %d: scheduled time %s IST is in the past, skipping", i+3, scheduledTime.Format("2006-01-02 15:04:05 IST"))
			continue
		}

		// Determine if this order should be placed as AMO based on scheduled time
		// Market hours: 9:00 AM - 3:30 PM IST (any day of the week)
		isAMO := r.shouldUseAMO(scheduledTime)

		// Create multiple orders based on lots value
		// Distribute totalQuantity across lots orders:
		// - baseQuantity = floor(totalQuantity / lots)
		// - remainder orders get baseQuantity + 1
		for orderNum := 1; orderNum <= lots; orderNum++ {
			// Calculate quantity for this order
			// First 'remainder' orders get baseQuantity + 1, rest get baseQuantity
			orderQuantity := baseQuantity
			if orderNum <= remainder {
				orderQuantity = baseQuantity + 1
			}
			
			// Generate unique order ID by appending order number
			orderID := models.GenerateOrderID(symbol, scheduledTime)
			if lots > 1 {
				// Append order number to make each order unique
				orderID = fmt.Sprintf("%s-%d", orderID, orderNum)
			}
			
			order := models.Order{
				ID:            orderID,
				Symbol:        symbol,
				Exchange:      exchange,
				Price:         price,
				Quantity:      orderQuantity,
				OrderType:     "LIMIT", // Default to LIMIT as we have a price
				Side:          side,
				ScheduledTime: scheduledTime,
				CreatedAt:     now,
				IsAMO:         isAMO,
			}

			if isAMO {
				r.logger.Debug("Row %d, Order %d/%d: Scheduled for %s IST (market closed) - marked as AMO", 
					i+3, orderNum, lots, scheduledTime.Format("2006-01-02 15:04:05 IST"))
			}

			r.logger.Debug("Parsed order %d/%d: %s, Exchange: %s, Symbol: %s, Name: %s, BSE: %s, Product: %s, Money: %.2f, Quantity: %d, Lots: %d, Total Qty: %d", 
				orderNum, lots, order.ID, exchange, symbol, name, bseCode, product, moneyNeeded, orderQuantity, lots, totalQuantity)

			orders = append(orders, order)
		}
		
		if lots > 1 {
			r.logger.Info("Row %d: Created %d orders (lots=%d, total qty=%d, base=%d, remainder=%d) for %s", 
				i+3, lots, lots, totalQuantity, baseQuantity, remainder, symbol)
		}
	}

	return orders, nil
}

// shouldUseAMO determines if an order should be placed as AMO based on scheduled time
// Market hours: 9:00 AM - 3:30 PM IST (any day of the week)
func (r *SheetsReader) shouldUseAMO(scheduledTime time.Time) bool {
	istLocation, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		r.logger.Warn("Failed to load IST timezone: %v", err)
		istLocation = time.UTC
	}
	istTime := scheduledTime.In(istLocation)

	// Get time in minutes from midnight
	hour := istTime.Hour()
	minute := istTime.Minute()
	minutesFromMidnight := hour*60 + minute

	// Market hours: 9:00 AM (540 minutes) to 3:30 PM (930 minutes)
	openTime := 9*60 + 0   // 9:00 AM = 540 minutes
	closeTime := 15*60 + 30 // 3:30 PM = 930 minutes

	// If outside market hours, use AMO
	return minutesFromMidnight < openTime || minutesFromMidnight >= closeTime
}

// HealthCheck checks if Google Sheets is accessible
func (r *SheetsReader) HealthCheck() error {
	_, err := r.service.Spreadsheets.Get(r.sheetID).Do()
	return err
}
