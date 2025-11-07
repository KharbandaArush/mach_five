package broker

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mach_five/trading-system/internal/config"
	"github.com/mach_five/trading-system/internal/logger"
	"github.com/mach_five/trading-system/internal/models"
	"golang.org/x/time/rate"
)

// Broker interface for executing orders
type Broker interface {
	ExecuteOrder(ctx context.Context, order models.Order) (models.ExecutionResult, error)
	HealthCheck(ctx context.Context) error
}

// BrokerManager manages broker instances and rate limiting
type BrokerManager struct {
	broker    Broker
	config    *config.Config
	logger    *logger.Logger
	rateLimit *rate.Limiter
	mu        sync.RWMutex
}

// NewBrokerManager creates a new broker manager
func NewBrokerManager(cfg *config.Config, log *logger.Logger) (*BrokerManager, error) {
	var broker Broker
	var err error

	switch cfg.Broker.Type {
	case "mock":
		broker = NewMockBroker(cfg, log)
	case "alpaca":
		broker, err = NewAlpacaBroker(cfg, log)
		if err != nil {
			return nil, fmt.Errorf("failed to create Alpaca broker: %w", err)
		}
	case "kite":
		broker, err = NewKiteBroker(cfg, log)
		if err != nil {
			return nil, fmt.Errorf("failed to create Kite broker: %w", err)
		}
	default:
		return nil, fmt.Errorf("unknown broker type: %s (supported: mock, alpaca, kite)", cfg.Broker.Type)
	}

	// Create rate limiter
	rateLimiter := rate.NewLimiter(
		rate.Limit(cfg.Broker.RateLimit.RequestsPerSecond),
		cfg.Broker.RateLimit.BurstSize,
	)

	return &BrokerManager{
		broker:    broker,
		config:    cfg,
		logger:    log,
		rateLimit: rateLimiter,
	}, nil
}

// ExecuteOrderWithRetry executes an order with adaptive retry logic and order splitting
func (bm *BrokerManager) ExecuteOrderWithRetry(ctx context.Context, order models.Order, maxRetries int) (models.ExecutionResult, error) {
	// Check if order splitting is needed
	if bm.config.Broker.OrderSplitting.Enabled && order.Quantity > bm.config.Broker.OrderSplitting.MaxOrderSize {
		return bm.executeSplitOrder(ctx, order, maxRetries)
	}

	// Execute single order (no splitting needed)
	return bm.executeSingleOrder(ctx, order, maxRetries)
}

// executeSingleOrder executes a single order with retry logic
func (bm *BrokerManager) executeSingleOrder(ctx context.Context, order models.Order, maxRetries int) (models.ExecutionResult, error) {
	var lastErr error
	var result models.ExecutionResult

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			bm.logger.Info("Retrying order %s (attempt %d/%d)", order.ID, attempt, maxRetries)
		}

		// Wait for rate limit
		if err := bm.rateLimit.Wait(ctx); err != nil {
			return result, fmt.Errorf("rate limit wait failed: %w", err)
		}

		// Execute order
		execResult, err := bm.broker.ExecuteOrder(ctx, order)
		if err == nil {
			bm.logger.Info("Order %s executed successfully: %+v", order.ID, execResult)
			return execResult, nil
		}

		lastErr = err
		bm.logger.Warn("Order execution failed (attempt %d): %v", attempt+1, err)

		// Determine if we should retry based on error type
		if !shouldRetry(err) {
			bm.logger.Error("Order %s failed with non-retryable error: %v", order.ID, err)
			break
		}

		// Adaptive retry delay based on error type
		retryDelay := bm.getRetryDelay(err, attempt)
		if retryDelay > 0 {
			select {
			case <-ctx.Done():
				return result, ctx.Err()
			case <-time.After(retryDelay):
			}
		}
	}

	return result, fmt.Errorf("order execution failed after %d attempts: %w", maxRetries+1, lastErr)
}

// executeSplitOrder splits a large order into multiple smaller orders and executes them
func (bm *BrokerManager) executeSplitOrder(ctx context.Context, order models.Order, maxRetries int) (models.ExecutionResult, error) {
	maxSize := bm.config.Broker.OrderSplitting.MaxOrderSize
	totalQuantity := order.Quantity

	// Calculate number of split orders needed
	numSplits := (totalQuantity + maxSize - 1) / maxSize // Ceiling division

	bm.logger.Info("Splitting order %s: quantity %d into %d orders (max size: %d)", 
		order.ID, totalQuantity, numSplits, maxSize)

	var allResults []models.ExecutionResult
	var totalExecutedQuantity int
	var totalExecutedPrice float64
	var hasError bool
	var lastError error

	// Execute split orders sequentially
	for i := 0; i < numSplits; i++ {
		// Calculate quantity for this split
		splitQuantity := maxSize
		if i == numSplits-1 {
			// Last order gets the remainder
			splitQuantity = totalQuantity - (i * maxSize)
		}

		// Create split order
		splitOrder := order
		splitOrder.ID = fmt.Sprintf("%s_split_%d_%d", order.ID, i+1, numSplits)
		splitOrder.Quantity = splitQuantity

		bm.logger.Info("Executing split order %d/%d: %s (quantity: %d)", 
			i+1, numSplits, splitOrder.ID, splitQuantity)

		// Execute split order with retry
		result, err := bm.executeSingleOrder(ctx, splitOrder, maxRetries)
		if err != nil {
			bm.logger.Error("Split order %s failed: %v", splitOrder.ID, err)
			hasError = true
			lastError = err
			// Continue with other splits even if one fails
			continue
		}

		allResults = append(allResults, result)
		totalExecutedQuantity += result.ExecutedQuantity
		if result.ExecutedPrice > 0 {
			// Average price calculation (weighted by quantity)
			if totalExecutedPrice == 0 {
				totalExecutedPrice = result.ExecutedPrice
			} else {
				// Weighted average
				totalExecutedPrice = (totalExecutedPrice*float64(totalExecutedQuantity-splitQuantity) + 
					result.ExecutedPrice*float64(splitQuantity)) / float64(totalExecutedQuantity)
			}
		}

		// Small delay between split orders to respect rate limits
		if i < numSplits-1 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	// Aggregate results
	aggregatedResult := models.ExecutionResult{
		OrderID:          order.ID,
		Success:          !hasError && totalExecutedQuantity > 0,
		ExecutedAt:       time.Now(),
		ExecutedQuantity: totalExecutedQuantity,
		ExecutedPrice:    totalExecutedPrice,
	}

	if hasError {
		aggregatedResult.ErrorMessage = fmt.Sprintf("Some split orders failed. Executed: %d/%d", 
			totalExecutedQuantity, totalQuantity)
		bm.logger.Warn("Order %s partially executed: %d/%d shares", 
			order.ID, totalExecutedQuantity, totalQuantity)
	} else {
		bm.logger.Info("Order %s fully executed: %d/%d shares across %d split orders", 
			order.ID, totalExecutedQuantity, totalQuantity, numSplits)
	}

	if lastError != nil && totalExecutedQuantity == 0 {
		return aggregatedResult, fmt.Errorf("all split orders failed: %w", lastError)
	}

	return aggregatedResult, nil
}

// shouldRetry determines if an error is retryable
func shouldRetry(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	
	// Don't retry authentication errors
	authErrors := []string{"auth", "unauthorized", "forbidden", "401", "403"}
	for _, substr := range authErrors {
		if strings.Contains(errStr, substr) {
			return false
		}
	}

	// Don't retry invalid order errors
	invalidErrors := []string{"invalid", "bad request", "400"}
	for _, substr := range invalidErrors {
		if strings.Contains(errStr, substr) {
			return false
		}
	}

	// Retry network errors, rate limits, and server errors
	retryErrors := []string{"network", "timeout", "rate limit", "429", "500", "502", "503", "504"}
	for _, substr := range retryErrors {
		if strings.Contains(errStr, substr) {
			return true
		}
	}

	return false
}

// getRetryDelay returns the retry delay based on error type and attempt
func (bm *BrokerManager) getRetryDelay(err error, attempt int) time.Duration {
	errStr := strings.ToLower(err.Error())

	// Rate limit errors: wait longer
	if strings.Contains(errStr, "rate limit") || strings.Contains(errStr, "429") {
		return time.Duration(attempt+1) * 2 * time.Second
	}

	// Network errors: exponential backoff
	if strings.Contains(errStr, "network") || strings.Contains(errStr, "timeout") || strings.Contains(errStr, "connection") {
		return time.Duration(1<<uint(attempt)) * time.Second
	}

	// Server errors: linear backoff
	if strings.Contains(errStr, "500") || strings.Contains(errStr, "502") || 
	   strings.Contains(errStr, "503") || strings.Contains(errStr, "504") {
		return time.Duration(attempt+1) * time.Second
	}

	// Default: no delay (as per requirements for Read Module, but we use adaptive for broker)
	return time.Duration(attempt) * time.Second
}

// HealthCheck checks broker health
func (bm *BrokerManager) HealthCheck(ctx context.Context) error {
	return bm.broker.HealthCheck(ctx)
}

// GetRateLimit returns the rate limiter
func (bm *BrokerManager) GetRateLimit() *rate.Limiter {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	return bm.rateLimit
}


