#!/bin/bash

# Quick test script for trigger module
# Run this to manually trigger order execution

DEPLOY_PATH="/opt/trading-system"

echo "=== Testing Trigger Module ==="
echo ""

# Check if running on GCP instance
if [ ! -d "$DEPLOY_PATH" ]; then
    echo "❌ Error: This script should be run on the GCP instance"
    exit 1
fi

echo "Current time: $(date)"
echo ""

echo "Orders in cache:"
redis-cli ZCARD pending_orders
echo ""

echo "Orders due now:"
NOW=$(date +%s)
redis-cli ZRANGEBYSCORE pending_orders 0 $NOW LIMIT 0 10
echo ""

echo "Running trigger module..."
echo ""

sudo "$DEPLOY_PATH/bin/trading-system" -module=trigger

echo ""
echo "=== Check logs for results ==="
echo "Trigger log: tail -f $DEPLOY_PATH/logs/trigger-module.log"
echo ""

