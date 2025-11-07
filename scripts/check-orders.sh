#!/bin/bash

# Check orders in Redis cache
# Shows detailed information about cached orders

echo "=== Orders in Redis Cache ==="
echo ""

NOW=$(date +%s)
echo "Current time: $(date)"
echo "Current timestamp: $NOW"
echo ""

echo "Total orders in cache:"
redis-cli ZCARD pending_orders
echo ""

echo "=== Orders Due Now (within last 60 seconds) ==="
DUE_TIME=$((NOW - 60))
ORDERS_DUE=$(redis-cli ZRANGEBYSCORE pending_orders $DUE_TIME $NOW WITHSCORES 2>/dev/null)
if [ -z "$ORDERS_DUE" ]; then
    echo "No orders due for execution"
else
    echo "$ORDERS_DUE"
fi
echo ""

echo "=== Next 10 Orders (by scheduled time) ==="
redis-cli ZRANGE pending_orders 0 9 WITHSCORES | while read -r order_id score; do
    if [ -n "$order_id" ]; then
        scheduled_time=$(date -d "@$score" 2>/dev/null || date -r "$score" 2>/dev/null || echo "N/A")
        echo "$order_id -> $scheduled_time (timestamp: $score)"
    fi
done
echo ""

echo "=== Sample Order Details ==="
FIRST_ORDER=$(redis-cli ZRANGE pending_orders 0 0)
if [ -n "$FIRST_ORDER" ]; then
    echo "Order ID: $FIRST_ORDER"
    redis-cli GET "order:$FIRST_ORDER" 2>/dev/null | jq . 2>/dev/null || redis-cli GET "order:$FIRST_ORDER"
else
    echo "No orders in cache"
fi
echo ""

