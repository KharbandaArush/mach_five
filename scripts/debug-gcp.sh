#!/bin/bash

# Debug script for GCP instance
# Run this on the GCP instance to diagnose issues

set -e

DEPLOY_PATH="/opt/trading-system"
LOG_DIR="$DEPLOY_PATH/logs"

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║        Trading System - GCP Debugging Tool                 ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

# Check if running on GCP instance
if [ ! -d "$DEPLOY_PATH" ]; then
    echo "❌ Error: This script should be run on the GCP instance"
    echo "   Expected path: $DEPLOY_PATH"
    exit 1
fi

echo "=== 1. Service Status ==="
echo ""
sudo systemctl status trading-system-read --no-pager | head -15
echo ""

echo "=== 2. Redis Status ==="
echo ""
if redis-cli ping > /dev/null 2>&1; then
    echo "✅ Redis is running"
    echo ""
    echo "Pending orders count:"
    redis-cli ZCARD pending_orders
    echo ""
    echo "Orders due now (next 10):"
    NOW=$(date +%s)
    redis-cli ZRANGEBYSCORE pending_orders 0 $NOW LIMIT 0 10
    echo ""
    echo "All pending orders (first 20):"
    redis-cli ZRANGE pending_orders 0 19
else
    echo "❌ Redis is not running"
fi
echo ""

echo "=== 3. Configuration Files ==="
echo ""
echo "Broker config:"
if [ -f "$DEPLOY_PATH/config/broker-config.json" ]; then
    sudo cat "$DEPLOY_PATH/config/broker-config.json" | jq . 2>/dev/null || sudo cat "$DEPLOY_PATH/config/broker-config.json"
else
    echo "❌ Broker config not found"
fi
echo ""

echo "Google credentials:"
if [ -f "$DEPLOY_PATH/config/google-credentials.json" ]; then
    echo "✅ Google credentials file exists"
    sudo ls -la "$DEPLOY_PATH/config/google-credentials.json"
else
    echo "❌ Google credentials not found"
fi
echo ""

echo "=== 4. Environment Variables ==="
echo ""
sudo systemctl show trading-system-read | grep -E "(GOOGLE|BROKER|REDIS|LOG)" | sort
echo ""

echo "=== 5. Recent Read Module Logs ==="
echo ""
if [ -f "$LOG_DIR/read-module.log" ]; then
    echo "Last 20 lines:"
    sudo tail -20 "$LOG_DIR/read-module.log"
else
    echo "❌ Read module log not found"
fi
echo ""

echo "=== 6. Recent Trigger Module Logs ==="
echo ""
if [ -f "$LOG_DIR/trigger-module.log" ]; then
    echo "Last 20 lines:"
    sudo tail -20 "$LOG_DIR/trigger-module.log"
else
    echo "❌ Trigger module log not found"
fi
echo ""

echo "=== 7. Cron Job Status ==="
echo ""
echo "Cron jobs:"
sudo crontab -l 2>/dev/null | grep trading-system || echo "No cron jobs found"
echo ""

echo "=== 8. Orders Due for Execution ==="
echo ""
NOW=$(date +%s)
echo "Current time: $(date)"
echo "Current timestamp: $NOW"
echo ""
echo "Orders due now (within last 60 seconds):"
DUE_TIME=$((NOW - 60))
redis-cli ZRANGEBYSCORE pending_orders $DUE_TIME $NOW WITHSCORES 2>/dev/null | head -20 || echo "No orders found"
echo ""

echo "=== 9. Test Trigger Module Manually ==="
echo ""
read -p "Run trigger module manually? (y/n): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "Running trigger module..."
    sudo "$DEPLOY_PATH/bin/trading-system" -module=trigger 2>&1 | tee /tmp/trigger-test.log
    echo ""
    echo "Trigger test output saved to /tmp/trigger-test.log"
fi
echo ""

echo "=== 10. Check Broker Connection ==="
echo ""
read -p "Test broker health check? (y/n): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "Testing broker connection..."
    # This would require a test command, for now just show config
    echo "Broker type: $(sudo cat $DEPLOY_PATH/config/broker-config.json | jq -r '.type' 2>/dev/null || echo 'unknown')"
fi
echo ""

echo "=== 11. System Resources ==="
echo ""
echo "Memory usage:"
free -h | head -2
echo ""
echo "Disk usage:"
df -h /opt | tail -1
echo ""

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║                    DEBUG SUMMARY                             ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""
echo "Common issues to check:"
echo "1. Are orders in Redis cache? (Check section 2)"
echo "2. Are orders due for execution? (Check section 8)"
echo "3. Is cron job running? (Check section 7)"
echo "4. Are there errors in trigger logs? (Check section 6)"
echo "5. Is broker config correct? (Check section 3)"
echo ""

