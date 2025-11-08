package models

import (
	"encoding/json"
	"time"
)

// Order represents a trading order
type Order struct {
	ID            string    `json:"id"`
	Symbol        string    `json:"symbol"`
	Exchange      string    `json:"exchange"`   // Exchange (NSE, BSE, etc.)
	Price         float64   `json:"price"`
	Quantity      int       `json:"quantity"`
	OrderType     string    `json:"order_type"` // Market, Limit
	Side          string    `json:"side"`       // Buy, Sell
	ScheduledTime time.Time `json:"scheduled_time"`
	CreatedAt     time.Time `json:"created_at"`
	IsAMO         bool      `json:"is_amo"`     // Whether this order should be placed as After Market Order
}

// OrderCacheEntry represents an order stored in cache
type OrderCacheEntry struct {
	Order         Order     `json:"order"`
	ExpiryTime    time.Time `json:"expiry_time"`
	CreatedAt     time.Time `json:"created_at"`
}

// ExecutionResult represents the result of order execution
type ExecutionResult struct {
	OrderID      string    `json:"order_id"`
	Success      bool      `json:"success"`
	ExecutionID  string    `json:"execution_id,omitempty"`
	ExecutedAt   time.Time `json:"executed_at"`
	ErrorMessage string    `json:"error_message,omitempty"`
	ExecutedPrice float64  `json:"executed_price,omitempty"`
	ExecutedQuantity int   `json:"executed_quantity,omitempty"`
}

// ProfilingMetrics tracks timing information for order execution
type ProfilingMetrics struct {
	OrderID           string        `json:"order_id"`
	ScheduledTime     time.Time     `json:"scheduled_time"`
	SchedulerDelay    time.Duration `json:"scheduler_delay"`    // Time between scheduled time and execution start
	CacheLookupTime   time.Duration `json:"cache_lookup_time"`
	BrokerConnectTime time.Duration `json:"broker_connect_time"`
	OrderExecutionTime time.Duration `json:"order_execution_time"`
	CleanupTime       time.Duration `json:"cleanup_time"`
	TotalTime         time.Duration `json:"total_time"`
	StartedAt         time.Time     `json:"started_at"`
	CompletedAt       time.Time     `json:"completed_at"`
}

// ToJSON converts OrderCacheEntry to JSON
func (e *OrderCacheEntry) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// FromJSON creates OrderCacheEntry from JSON
func (e *OrderCacheEntry) FromJSON(data []byte) error {
	return json.Unmarshal(data, e)
}

// GenerateOrderID generates a unique order ID
func GenerateOrderID(symbol string, scheduledTime time.Time) string {
	return symbol + ":" + scheduledTime.Format(time.RFC3339)
}


