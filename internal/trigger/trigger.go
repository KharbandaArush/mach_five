package trigger

import (
	"context"
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
	config              *config.Config
	cache               *cache.RedisCache
	brokerManager       *broker.BrokerManager
	logger              *logger.Logger
	workerPool          int
	istLocation         *time.Location // Cached timezone location
	healthCheckMu       sync.Mutex     // Mutex to ensure only one health check runs at a time
	healthCheckInProgress bool         // Flag to track if health check is running
}

// NewTrigger creates a new trigger instance
func NewTrigger(cfg *config.Config, cache *cache.RedisCache, brokerMgr *broker.BrokerManager, log *logger.Logger) *Trigger {
	// Load and cache timezone location once
	istLocation, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		log.Warn("Failed to load IST timezone, using UTC: %v", err)
		istLocation = time.UTC
	}
	
	return &Trigger{
		config:        cfg,
		cache:         cache,
		brokerManager: brokerMgr,
		logger:        log,
		workerPool:    cfg.Trigger.WorkerPoolSize,
		istLocation:   istLocation,
	}
}

// ExecuteDueOrders executes all orders that are due for execution
func (t *Trigger) ExecuteDueOrders(ctx context.Context) error {
	// Get current time in IST using cached location (optimized for 1ms polling)
	now := time.Now().In(t.istLocation)
	
	// Get orders due for execution
	orders, err := t.cache.GetOrdersDueForExecution(now)
	if err != nil {
		t.logger.Error("❌ Failed to get orders due for execution")
		t.logger.Error("   Current time (IST): %s", now.Format("2006-01-02 15:04:05 IST"))
		t.logger.Error("   Error: %v", err)
		return fmt.Errorf("failed to get orders due for execution: %w", err)
	}

	// Return silently if no orders are due (no logging - critical for 1ms polling)
	if len(orders) == 0 {
		return nil
	}
	
	startTime := time.Now()
	t.logger.Debug("Checking for orders due at %s IST", now.Format("2006-01-02 15:04:05 IST"))

	// Only log when there are orders to execute
	t.logger.Section("🚀 Order Execution Cycle Started")

	t.logger.Success("Found %d orders due for execution", len(orders))
	
	// Log orders in table format
	headers := []string{"Order ID", "Symbol", "Side", "Qty", "Price", "Scheduled Time"}
	rows := make([][]string, 0, len(orders))
	for _, order := range orders {
		rows = append(rows, []string{
			truncateString(order.ID, 20),
			truncateString(order.Symbol, 20),
			order.Side,
			fmt.Sprintf("%d", order.Quantity),
			fmt.Sprintf("%.2f", order.Price),
			order.ScheduledTime.Format("15:04:05 IST"),
		})
	}
	t.logger.Table(headers, rows)

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
	t.logger.Success("✅ Order execution cycle completed in %v", duration)

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

	t.logger.Info("👷 Worker %d processing order %s (⏱️  scheduler delay: %v)", 
		workerID, order.ID, metrics.SchedulerDelay)

	// Try to acquire lock to prevent duplicate execution
	lockTTL := 30 * time.Second
	acquired, err := t.cache.TryLock(order.ID, lockTTL)
	if err != nil {
		t.logger.Error("❌ Failed to acquire lock for order %s", order.ID)
		t.logger.Error("   Order ID: %s", order.ID)
		t.logger.Error("   Lock TTL: %v", lockTTL)
		t.logger.Error("   Error: %v", err)
		t.logger.Error("   Redis connection may be unstable")
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
	result, err := t.brokerManager.ExecuteOrder(ctx, order)
	metrics.BrokerConnectTime = time.Since(brokerStart)
	metrics.OrderExecutionTime = metrics.BrokerConnectTime // Combined for simplicity

	if err != nil {
		metrics.CompletedAt = time.Now()
		metrics.TotalTime = time.Since(metrics.StartedAt)
		t.logProfilingMetrics(metrics, false, err.Error())
		t.logger.Error("❌ Order %s execution failed", order.ID)
		t.logger.Error("   Order Details:")
		t.logger.Error("     - ID: %s", order.ID)
		t.logger.Error("     - Symbol: %s", order.Symbol)
		t.logger.Error("     - Side: %s", order.Side)
		t.logger.Error("     - Quantity: %d", order.Quantity)
		t.logger.Error("     - Price: %.2f", order.Price)
		t.logger.Error("     - Scheduled Time: %s", order.ScheduledTime.Format("2006-01-02 15:04:05 IST"))
		t.logger.Error("   Error: %v", err)
		t.logger.Error("   Full error details logged by broker module above")
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
	if result.Success {
		t.logger.Success("✅ Order %s executed successfully", order.ID)
		t.logger.TableSimple("Execution Details", map[string]string{
			"Order ID":        order.ID,
			"Symbol":          order.Symbol,
			"Side":            order.Side,
			"Quantity":        fmt.Sprintf("%d", result.ExecutedQuantity),
			"Price":           fmt.Sprintf("%.2f", result.ExecutedPrice),
			"Execution ID":    result.ExecutionID,
			"Executed At":     result.ExecutedAt.Format("15:04:05 IST"),
		})
	} else {
		t.logger.Error("❌ Order %s execution failed: %s", order.ID, result.ErrorMessage)
	}
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

// logProfilingMetrics logs profiling metrics in tabular format
func (t *Trigger) logProfilingMetrics(metrics models.ProfilingMetrics, success bool, errorMsg string) {
	// Format times for display
	scheduledTime := metrics.ScheduledTime.Format("2006-01-02 15:04:05 IST")
	startedAt := metrics.StartedAt.Format("2006-01-02 15:04:05 IST")
	completedAt := metrics.CompletedAt.Format("2006-01-02 15:04:05 IST")
	
	// Status indicator
	statusIcon := "✅"
	statusText := "SUCCESS"
	if !success {
		statusIcon = "❌"
		statusText = "FAILED"
	}

	// Build table data
	tableData := map[string]string{
		"Order ID":            metrics.OrderID,
		"Status":              fmt.Sprintf("%s %s", statusIcon, statusText),
		"Scheduled Time":      scheduledTime,
		"Started At":          startedAt,
		"Completed At":        completedAt,
		"Scheduler Delay":     fmt.Sprintf("%d ms", metrics.SchedulerDelay.Milliseconds()),
		"Cache Lookup":        fmt.Sprintf("%d ms", metrics.CacheLookupTime.Milliseconds()),
		"Broker Connect":      fmt.Sprintf("%d ms", metrics.BrokerConnectTime.Milliseconds()),
		"Order Execution":     fmt.Sprintf("%d ms", metrics.OrderExecutionTime.Milliseconds()),
		"Cleanup":            fmt.Sprintf("%d ms", metrics.CleanupTime.Milliseconds()),
		"Total Time":          fmt.Sprintf("%d ms", metrics.TotalTime.Milliseconds()),
	}

	// Add error message if present
	if errorMsg != "" {
		// Truncate long error messages
		if len(errorMsg) > 100 {
			errorMsg = errorMsg[:97] + "..."
		}
		tableData["Error"] = errorMsg
	}

	// Log as table
	t.logger.Section("📊 Profiling Metrics")
	t.logger.TableSimple("Order Execution Profile", tableData)
}

// MaintainSystemReadiness ensures system is ready before execution
func (t *Trigger) MaintainSystemReadiness(ctx context.Context) error {
	// Check cache health
	if err := t.cache.HealthCheck(); err != nil {
		t.logger.Error("❌ Cache health check failed")
		t.logger.Error("   Error: %v", err)
		t.logger.Error("   Redis may be down or unreachable")
		return fmt.Errorf("cache health check failed: %w", err)
	}
	t.logger.Debug("✅ Cache health check passed")

	// Check broker health
	brokerHealthOk := true
	if err := t.brokerManager.HealthCheck(ctx); err != nil {
		brokerHealthOk = false
		t.logger.Error("❌ Broker health check failed")
		t.logger.Error("   Error: %v", err)
		t.logger.Error("   Broker may be unreachable or credentials invalid")
		t.logger.Warn("   ⚠️  Orders will still attempt execution, but may fail")
		// Don't fail completely - allow orders to attempt execution
		// return fmt.Errorf("broker health check failed: %w", err)
	}

	// Only log success when both checks pass
	if brokerHealthOk {
		t.logger.Info("✅ System readiness check passed (cache and broker healthy)")
	} else {
		t.logger.Info("⚠️  System readiness check completed (cache healthy, broker unhealthy)")
	}
	
	return nil
}

// RunContinuous runs the trigger in a continuous loop, checking for orders at regular intervals
func (t *Trigger) RunContinuous(ctx context.Context) error {
	checkInterval := t.config.Trigger.CheckInterval
	healthCheckInterval := t.config.Trigger.HealthCheckInterval
	
	t.logger.Info("🔄 Starting continuous trigger loop")
	t.logger.Info("   Check interval: %v", checkInterval)
	t.logger.Info("   Health check interval: %v", healthCheckInterval)
	
	checkTicker := time.NewTicker(checkInterval)
	defer checkTicker.Stop()
	
	healthCheckTicker := time.NewTicker(healthCheckInterval)
	defer healthCheckTicker.Stop()
	
	lastHealthCheck := time.Now()
	
	// Run initial health check
	if err := t.MaintainSystemReadiness(ctx); err != nil {
		t.logger.Warn("⚠️  Initial health check failed, will retry: %v", err)
	}
	
	for {
		select {
		case <-ctx.Done():
			t.logger.Info("🛑 Stopping continuous trigger loop")
			return ctx.Err()
			
		case <-checkTicker.C:
			// Check for due orders
			if err := t.ExecuteDueOrders(ctx); err != nil {
				t.logger.Error("❌ Error executing due orders: %v", err)
				// Continue running even if there's an error
			}
			
		case <-healthCheckTicker.C:
			// Run periodic health checks (ensure only one runs at a time)
			if time.Since(lastHealthCheck) >= healthCheckInterval {
				// Check if health check is already running
				t.healthCheckMu.Lock()
				if t.healthCheckInProgress {
					t.healthCheckMu.Unlock()
					t.logger.Debug("⏭️  Health check already in progress, skipping")
				} else {
					t.healthCheckInProgress = true
					t.healthCheckMu.Unlock()
					
					// Update lastHealthCheck before starting (prevents race condition)
					lastHealthCheck = time.Now()
					
					// Run health check in goroutine to avoid blocking
					go func() {
						defer func() {
							t.healthCheckMu.Lock()
							t.healthCheckInProgress = false
							t.healthCheckMu.Unlock()
						}()
						
						t.logger.Debug("🔍 Running periodic health check")
						if err := t.MaintainSystemReadiness(ctx); err != nil {
							t.logger.Warn("⚠️  Periodic health check failed: %v", err)
						}
					}()
				}
			}
		}
	}
}

// truncateString truncates a string to max length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}


