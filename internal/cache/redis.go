package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/mach_five/trading-system/internal/models"
)

// RedisCache implements cache interface using Redis
type RedisCache struct {
	client *redis.Client
	ctx    context.Context
}

// NewRedisCache creates a new Redis cache instance
func NewRedisCache(addr, password string, db int) (*RedisCache, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx := context.Background()
	
	// Test connection
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisCache{
		client: rdb,
		ctx:    ctx,
	}, nil
}

// StoreOrder stores an order in cache with expiry
func (r *RedisCache) StoreOrder(order models.Order, expiryTime time.Time) error {
	orderID := order.ID
	entry := models.OrderCacheEntry{
		Order:      order,
		ExpiryTime: expiryTime,
		CreatedAt:  time.Now(),
	}

	data, err := entry.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal order: %w", err)
	}

	key := fmt.Sprintf("order:%s", orderID)
	ttl := time.Until(expiryTime)
	if ttl <= 0 {
		return fmt.Errorf("expiry time is in the past")
	}

	// Store order
	if err := r.client.Set(r.ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to store order: %w", err)
	}

	// Add to pending orders sorted set (score = scheduled time as unix timestamp)
	score := float64(order.ScheduledTime.Unix())
	if err := r.client.ZAdd(r.ctx, "pending_orders", &redis.Z{
		Score:  score,
		Member: orderID,
	}).Err(); err != nil {
		return fmt.Errorf("failed to add to pending orders: %w", err)
	}

	return nil
}

// GetOrdersDueForExecution returns orders that are due for execution
func (r *RedisCache) GetOrdersDueForExecution(now time.Time) ([]models.Order, error) {
	// Query pending_orders sorted set for orders where scheduled_time <= now
	maxScore := float64(now.Unix())
	
	orderIDs, err := r.client.ZRangeByScore(r.ctx, "pending_orders", &redis.ZRangeBy{
		Min: "0",
		Max: fmt.Sprintf("%.0f", maxScore),
	}).Result()
	
	if err != nil {
		return nil, fmt.Errorf("failed to query pending orders: %w", err)
	}

	var orders []models.Order
	for _, orderID := range orderIDs {
		key := fmt.Sprintf("order:%s", orderID)
		data, err := r.client.Get(r.ctx, key).Result()
		if err == redis.Nil {
			// Order expired or was removed, remove from sorted set
			r.client.ZRem(r.ctx, "pending_orders", orderID)
			continue
		} else if err != nil {
			continue
		}

		var entry models.OrderCacheEntry
		if err := entry.FromJSON([]byte(data)); err != nil {
			continue
		}

		// Check if order is still within expiry window
		if now.After(entry.ExpiryTime) {
			// Expired, remove it
			r.RemoveOrder(entry.Order.ID)
			continue
		}

		orders = append(orders, entry.Order)
	}

	return orders, nil
}

// RemoveOrder removes an order from cache
func (r *RedisCache) RemoveOrder(orderID string) error {
	key := fmt.Sprintf("order:%s", orderID)
	
	// Remove from hash
	if err := r.client.Del(r.ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to remove order: %w", err)
	}

	// Remove from sorted set
	if err := r.client.ZRem(r.ctx, "pending_orders", orderID).Err(); err != nil {
		return fmt.Errorf("failed to remove from pending orders: %w", err)
	}

	return nil
}

// TryLock attempts to acquire a lock for order execution (prevents duplicate execution)
func (r *RedisCache) TryLock(orderID string, ttl time.Duration) (bool, error) {
	lockKey := fmt.Sprintf("lock:order:%s", orderID)
	result, err := r.client.SetNX(r.ctx, lockKey, "1", ttl).Result()
	if err != nil {
		return false, fmt.Errorf("failed to acquire lock: %w", err)
	}
	return result, nil
}

// ReleaseLock releases the lock for an order
func (r *RedisCache) ReleaseLock(orderID string) error {
	lockKey := fmt.Sprintf("lock:order:%s", orderID)
	return r.client.Del(r.ctx, lockKey).Err()
}

// Close closes the Redis connection
func (r *RedisCache) Close() error {
	return r.client.Close()
}

// HealthCheck checks if Redis is accessible
func (r *RedisCache) HealthCheck() error {
	return r.client.Ping(r.ctx).Err()
}

