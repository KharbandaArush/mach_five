package broker

import (
	"time"
)

// MarketHours handles market timing logic
type MarketHours struct {
	OpenTime  time.Duration // 9:00 AM in minutes from midnight
	CloseTime time.Duration // 3:30 PM in minutes from midnight
}

// NewMarketHours creates a new MarketHours instance
func NewMarketHours() *MarketHours {
	return &MarketHours{
		OpenTime:  9*60 + 0,  // 9:00 AM = 540 minutes
		CloseTime: 15*60 + 30, // 3:30 PM = 930 minutes
	}
}

// IsMarketOpen checks if the market is currently open
// Market hours: 9:00 AM - 3:30 PM IST, Monday to Friday
func (m *MarketHours) IsMarketOpen(now time.Time) bool {
	// Convert to IST (UTC+5:30)
	istLocation, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		// Fallback to UTC if IST not available
		istLocation = time.UTC
	}
	istTime := now.In(istLocation)

	// Check if it's a weekday (Monday = 1, Friday = 5)
	weekday := istTime.Weekday()
	if weekday == time.Saturday || weekday == time.Sunday {
		return false
	}

	// Get time in minutes from midnight
	hour := istTime.Hour()
	minute := istTime.Minute()
	minutesFromMidnight := time.Duration(hour)*60 + time.Duration(minute)

	// Check if within market hours
	return minutesFromMidnight >= m.OpenTime && minutesFromMidnight < m.CloseTime
}

// ShouldUseAMO determines if an order should be placed as AMO
func (m *MarketHours) ShouldUseAMO(now time.Time) bool {
	return !m.IsMarketOpen(now)
}

// GetNextMarketOpenTime returns the next market open time
func (m *MarketHours) GetNextMarketOpenTime(now time.Time) time.Time {
	istLocation, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		istLocation = time.UTC
	}
	istTime := now.In(istLocation)

	// Get today's market open time
	year, month, day := istTime.Date()
	marketOpen := time.Date(year, month, day, 9, 0, 0, 0, istLocation)

	// If market already opened today and it's past close time, move to next day
	if istTime.After(marketOpen) && !m.IsMarketOpen(istTime) {
		marketOpen = marketOpen.Add(24 * time.Hour)
	}

	// Skip weekends
	for marketOpen.Weekday() == time.Saturday || marketOpen.Weekday() == time.Sunday {
		marketOpen = marketOpen.Add(24 * time.Hour)
	}

	return marketOpen
}

