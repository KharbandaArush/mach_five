package trigger

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/mach_five/trading-system/internal/broker"
	"github.com/mach_five/trading-system/internal/cache"
	"github.com/mach_five/trading-system/internal/config"
	"github.com/mach_five/trading-system/internal/logger"
	"github.com/mach_five/trading-system/internal/models"
)

// Trigger handles order execution
type Trigger struct {
	config        *config.Config
	cache         *cache.RedisCache
	brokerManager *broker.BrokerManager
	logger        *logger.Logger
	workerPool    int
}

// NewTrigger creates a new trigger instance
func NewTrigger(cfg *config.Config, cache *cache.RedisCache, brokerMgr *broker.BrokerManager, log *logger.Logger) *Trigger {
	return &Trigger{
		config:        cfg,
		cache:         cache,
		brokerManager: brokerMgr,
		logger:        log,
		workerPool:    cfg.Trigger.WorkerPoolSize,
	}
}

// ExecuteDueOrders executes all orders that are due for execution
func (t *Trigger) ExecuteDueOrders(ctx context.Context) error {
	startTime := time.Now()
	t.logger.Info("Starting order execution cycle")

	// Get current time in IST (orders are scheduled in IST)
	istLocation, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		t.logger.Warn("Failed to load IST timezone, using UTC: %v", err)
		istLocation = time.UTC
	}
	now := time.Now().In(istLocation)
	
	t.logger.Debug("Checking for orders due at %s IST", now.Format("2006-01-02 15:04:05 IST"))
	
	// Get orders due for execution
	orders, err := t.cache.GetOrdersDueForExecution(now)
	if err != nil {
		return fmt.Errorf("failed to get orders due for execution: %w", err)
	}

	if len(orders) == 0 {
		t.logger.Info("No orders due for execution")
		return nil
	}

	t.logger.Info("Found %d orders due for execution", len(orders))

	// Use worker pool to execute orders concurrently
	orderChan := make(chan models.Order, len(orders))
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < t.workerPool; i++ {
		wg.Add(1)
		go t.worker(ctx, i, orderChan, &wg)
	}

	// Send orders to workers
	for _, order := range orders {
		orderChan <- order
	}
	close(orderChan)

	// Wait for all workers to complete
	wg.Wait()

	duration := time.Since(startTime)
	t.logger.Info("Order execution cycle completed in %v", duration)

	return nil
}

// worker processes orders from the channel
func (t *Trigger) worker(ctx context.Context, workerID int, orderChan <-chan models.Order, wg *sync.WaitGroup) {
	defer wg.Done()

	for order := range orderChan {
		select {
		case <-ctx.Done():
			t.logger.Info("Worker %d stopping due to context cancellation", workerID)
			return
		default:
			t.executeOrder(ctx, workerID, order)
		}
	}
}

// executeOrder executes a single order with profiling
func (t *Trigger) executeOrder(ctx context.Context, workerID int, order models.Order) {
	metrics := models.ProfilingMetrics{
		OrderID:       order.ID,
		ScheduledTime: order.ScheduledTime,
		StartedAt:     time.Now(),
	}

	// Calculate scheduler delay
	metrics.SchedulerDelay = time.Since(order.ScheduledTime)
	if metrics.SchedulerDelay < 0 {
		metrics.SchedulerDelay = 0
	}

	t.logger.Info("Worker %d processing order %s (scheduler delay: %v)", 
		workerID, order.ID, metrics.SchedulerDelay)

	// Try to acquire lock to prevent duplicate execution
	lockTTL := 30 * time.Second
	acquired, err := t.cache.TryLock(order.ID, lockTTL)
	if err != nil {
		t.logger.Error("Failed to acquire lock for order %s: %v", order.ID, err)
		t.removeOrder(order.ID, "lock acquisition failed")
		return
	}

	if !acquired {
		t.logger.Warn("Order %s is already being processed by another worker", order.ID)
		return
	}

	defer func() {
		// Release lock
		if err := t.cache.ReleaseLock(order.ID); err != nil {
			t.logger.Warn("Failed to release lock for order %s: %v", order.ID, err)
		}
	}()

	// Profile cache lookup (already done, but track time)
	cacheStart := time.Now()
	metrics.CacheLookupTime = time.Since(cacheStart)

	// Profile broker connection and execution
	brokerStart := time.Now()
	result, err := t.brokerManager.ExecuteOrderWithRetry(ctx, order, 3)
	metrics.BrokerConnectTime = time.Since(brokerStart)
	metrics.OrderExecutionTime = metrics.BrokerConnectTime // Combined for simplicity

	if err != nil {
		metrics.CompletedAt = time.Now()
		metrics.TotalTime = time.Since(metrics.StartedAt)
		t.logProfilingMetrics(metrics, false, err.Error())
		t.logger.Error("Order %s execution failed: %v", order.ID, err)
		t.removeOrder(order.ID, err.Error())
		return
	}

	// Profile cleanup
	cleanupStart := time.Now()
	t.removeOrder(order.ID, "")
	metrics.CleanupTime = time.Since(cleanupStart)

	metrics.CompletedAt = time.Now()
	metrics.TotalTime = time.Since(metrics.StartedAt)

	t.logProfilingMetrics(metrics, result.Success, result.ErrorMessage)
	t.logger.Info("Order %s executed successfully: %+v", order.ID, result)
}

// removeOrder removes an order from cache
func (t *Trigger) removeOrder(orderID, reason string) {
	if err := t.cache.RemoveOrder(orderID); err != nil {
		t.logger.Error("Failed to remove order %s from cache: %v", orderID, err)
	} else {
		if reason != "" {
			t.logger.Debug("Removed order %s from cache (reason: %s)", orderID, reason)
		} else {
			t.logger.Debug("Removed order %s from cache after execution", orderID)
		}
	}
}

// logProfilingMetrics logs profiling metrics as JSON
func (t *Trigger) logProfilingMetrics(metrics models.ProfilingMetrics, success bool, errorMsg string) {
	profileData := map[string]interface{}{
		"order_id":            metrics.OrderID,
		"scheduled_time":      metrics.ScheduledTime.Format(time.RFC3339),
		"scheduler_delay_ms":  metrics.SchedulerDelay.Milliseconds(),
		"cache_lookup_ms":     metrics.CacheLookupTime.Milliseconds(),
		"broker_connect_ms":   metrics.BrokerConnectTime.Milliseconds(),
		"order_execution_ms":  metrics.OrderExecutionTime.Milliseconds(),
		"cleanup_ms":         metrics.CleanupTime.Milliseconds(),
		"total_time_ms":       metrics.TotalTime.Milliseconds(),
		"started_at":          metrics.StartedAt.Format(time.RFC3339),
		"completed_at":        metrics.CompletedAt.Format(time.RFC3339),
		"success":             success,
	}

	if errorMsg != "" {
		profileData["error"] = errorMsg
	}

	jsonData, err := json.Marshal(profileData)
	if err != nil {
		t.logger.Error("Failed to marshal profiling metrics: %v", err)
		return
	}

	t.logger.Info("PROFILING: %s", string(jsonData))
}

// MaintainSystemReadiness ensures system is ready before execution
func (t *Trigger) MaintainSystemReadiness(ctx context.Context) error {
	// Check cache health
	if err := t.cache.HealthCheck(); err != nil {
		return fmt.Errorf("cache health check failed: %w", err)
	}

	// Check broker health
	if err := t.brokerManager.HealthCheck(ctx); err != nil {
		return fmt.Errorf("broker health check failed: %w", err)
	}

	t.logger.Debug("System readiness check passed")
	return nil
}


