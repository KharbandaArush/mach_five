package broker

import (
	"context"
	"fmt"
	"sync"

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

	log.Info("🔧 Initializing broker manager with type: %s", cfg.Broker.Type)

	switch cfg.Broker.Type {
	case "mock":
		log.Warn("⚠️  Using MOCK broker - no real trades will be executed!")
		broker = NewMockBroker(cfg, log)
	case "alpaca":
		log.Info("📊 Initializing Alpaca broker")
		broker, err = NewAlpacaBroker(cfg, log)
		if err != nil {
			return nil, fmt.Errorf("failed to create Alpaca broker: %w", err)
		}
	case "kite":
		log.Info("🪁 Initializing Kite (Zerodha) broker")
		broker, err = NewKiteBroker(cfg, log)
		if err != nil {
			return nil, fmt.Errorf("failed to create Kite broker: %w", err)
		}
		log.Success("✅ Kite broker initialized successfully")
	default:
		log.Error("❌ Unknown broker type: %s, falling back to mock", cfg.Broker.Type)
		log.Warn("⚠️  Supported types: mock, alpaca, kite")
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

// ExecuteOrder executes an order without retries
func (bm *BrokerManager) ExecuteOrder(ctx context.Context, order models.Order) (models.ExecutionResult, error) {
	// Wait for rate limit
	if err := bm.rateLimit.Wait(ctx); err != nil {
		return models.ExecutionResult{}, fmt.Errorf("rate limit wait failed: %w", err)
	}

	// Execute order (single attempt, no retries)
	execResult, err := bm.broker.ExecuteOrder(ctx, order)
	if err != nil {
		bm.logger.Error("Order %s execution failed: %v", order.ID, err)
		return execResult, err
	}

	bm.logger.Info("Order %s executed successfully: %+v", order.ID, execResult)
	return execResult, nil
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


