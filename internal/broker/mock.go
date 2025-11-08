package broker

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/mach_five/trading-system/internal/config"
	"github.com/mach_five/trading-system/internal/logger"
	"github.com/mach_five/trading-system/internal/models"
)

// MockBroker is a mock broker for testing
type MockBroker struct {
	config *config.Config
	logger *logger.Logger
}

// NewMockBroker creates a new mock broker
func NewMockBroker(cfg *config.Config, log *logger.Logger) *MockBroker {
	return &MockBroker{
		config: cfg,
		logger: log,
	}
}

// ExecuteOrder simulates order execution
func (m *MockBroker) ExecuteOrder(ctx context.Context, order models.Order) (models.ExecutionResult, error) {
	m.logger.Info("Mock broker executing order: %s", order.ID)

	// Simulate network delay
	time.Sleep(time.Duration(rand.Intn(100)+50) * time.Millisecond)

	// Simulate occasional failures (10% failure rate)
	if rand.Float32() < 0.1 {
		return models.ExecutionResult{
			OrderID:     order.ID,
			Success:     false,
			ExecutedAt:  time.Now(),
			ErrorMessage: "mock broker simulated failure",
		}, fmt.Errorf("mock broker simulated failure")
	}

	// Simulate price slippage
	executedPrice := order.Price * (1.0 + (rand.Float64()-0.5)*0.001) // ±0.05% slippage

	result := models.ExecutionResult{
		OrderID:        order.ID,
		Success:        true,
		ExecutionID:    fmt.Sprintf("MOCK-%d", time.Now().UnixNano()),
		ExecutedAt:     time.Now(),
		ExecutedPrice:  executedPrice,
		ExecutedQuantity: order.Quantity,
	}

	m.logger.Info("Mock broker executed order successfully: %+v", result)
	return result, nil
}

// HealthCheck always returns healthy for mock broker
func (m *MockBroker) HealthCheck(ctx context.Context) error {
	return nil
}

// ValidateSymbol always returns true for mock broker (no validation)
func (m *MockBroker) ValidateSymbol(ctx context.Context, exchange, symbol string) (bool, error) {
	return true, nil
}


