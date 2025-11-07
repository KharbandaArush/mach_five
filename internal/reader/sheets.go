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
	credData, err := os.ReadFile(cfg.GoogleSheets.CredentialsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read credentials file: %w", err)
	}

	// Parse credentials
	creds, err := google.CredentialsFromJSON(ctx, credData, sheets.SpreadsheetsReadonlyScope)
	if err != nil {
		return nil, fmt.Errorf("failed to parse credentials: %w", err)
	}

	// Create sheets service
	srv, err := sheets.NewService(ctx, option.WithCredentials(creds))
	if err != nil {
		return nil, fmt.Errorf("failed to create sheets service: %w", err)
	}

	return &SheetsReader{
		config:  cfg,
		cache:   cache,
		logger:  log,
		service: srv,
		sheetID: cfg.GoogleSheets.SheetID,
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
		r.logger.Error("Failed to read buy orders: %v", err)
	} else {
		r.logger.Info("Read %d buy orders", len(buyOrders))
		allOrders = append(allOrders, buyOrders...)
	}

	// Read sell orders from to_sell sheet
	r.logger.Debug("Reading sell orders from sheet: %s, range: %s", r.sheetID, r.config.GoogleSheets.SellRange)
	sellOrders, err := r.readSheet(r.config.GoogleSheets.SellRange, "Sell")
	if err != nil {
		r.logger.Error("Failed to read sell orders: %v", err)
	} else {
		r.logger.Info("Read %d sell orders", len(sellOrders))
		allOrders = append(allOrders, sellOrders...)
	}

	r.logger.Info("Total orders read: %d (Buy: %d, Sell: %d)", 
		len(allOrders), len(buyOrders), len(sellOrders))

	// Cache all orders
	for _, order := range allOrders {
		expiryTime := order.ScheduledTime.Add(10 * time.Second)
		if err := r.cache.StoreOrder(order, expiryTime); err != nil {
			r.logger.Error("Failed to cache order %s: %v", order.ID, err)
			continue
		}
		r.logger.Debug("Cached order: %s, side: %s, symbol: %s, scheduled: %s, expiry: %s", 
			order.ID, order.Side, order.Symbol, order.ScheduledTime.Format(time.RFC3339), expiryTime.Format(time.RFC3339))
	}

	return nil
}

// readSheet reads orders from a specific sheet range
func (r *SheetsReader) readSheet(rangeStr, side string) ([]models.Order, error) {
	resp, err := r.service.Spreadsheets.Values.Get(r.sheetID, rangeStr).Do()
	if err != nil {
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
// Column mapping (B through J):
// B: planned_buy_price (float) - Price
// C: product (string) - Product type
// D: Name (string) - Stock name
// E: bse_code (string) - BSE code
// F: symbol (string) - Trading symbol
// G: execute_date (string) - Date (YYYY-MM-DD)
// H: execute_time (string) - Time (HH:MM:SS or HH:MM)
// I: Money Needed (float) - Money required
// J: Lots (int) - Quantity
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
		// Need at least 9 columns (B through J, indexed 0-8)
		if len(row) < 9 {
			r.logger.Warn("Row %d has insufficient columns (%d), skipping", i+3, len(row))
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

		// Column I (index 7): Money Needed - not used directly but logged
		moneyNeededStr := strings.TrimSpace(fmt.Sprintf("%v", row[7]))
		moneyNeeded, _ := strconv.ParseFloat(moneyNeededStr, 64)

		// Column J (index 8): Lots (quantity)
		lotsStr := strings.TrimSpace(fmt.Sprintf("%v", row[8]))
		quantity, qtyErr := strconv.Atoi(lotsStr)
		if qtyErr != nil || quantity <= 0 {
			r.logger.Warn("Row %d: invalid lots '%s', defaulting to 1", i+3, lotsStr)
			quantity = 1
		}

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

		// Create order
		order := models.Order{
			ID:            models.GenerateOrderID(symbol, scheduledTime),
			Symbol:        symbol,
			Price:         price,
			Quantity:      quantity,
			OrderType:     "LIMIT", // Default to LIMIT as we have a price
			Side:          side,
			ScheduledTime: scheduledTime,
			CreatedAt:     now,
		}

		r.logger.Debug("Parsed order: %s, Name: %s, BSE: %s, Product: %s, Money: %.2f, Lots: %d", 
			order.ID, name, bseCode, product, moneyNeeded, quantity)

		orders = append(orders, order)
	}

	return orders, nil
}

// HealthCheck checks if Google Sheets is accessible
func (r *SheetsReader) HealthCheck() error {
	_, err := r.service.Spreadsheets.Get(r.sheetID).Do()
	return err
}
