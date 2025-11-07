#!/bin/bash

# Watch logs in real-time
# Shows all module logs simultaneously

DEPLOY_PATH="/opt/trading-system"
LOG_DIR="$DEPLOY_PATH/logs"

echo "=== Watching Trading System Logs ==="
echo "Press Ctrl+C to stop"
echo ""

# Check if logs exist
if [ ! -f "$LOG_DIR/read-module.log" ] && [ ! -f "$LOG_DIR/trigger-module.log" ]; then
    echo "❌ No log files found"
    exit 1
fi

# Show recent logs first
echo "=== Recent Logs (last 10 lines each) ==="
echo ""
echo "--- Read Module ---"
sudo tail -10 "$LOG_DIR/read-module.log" 2>/dev/null || echo "No read module log"
echo ""
echo "--- Trigger Module ---"
sudo tail -10 "$LOG_DIR/trigger-module.log" 2>/dev/null || echo "No trigger module log"
echo ""

echo "=== Following Logs (real-time) ==="
echo ""

# Follow all logs
sudo tail -f "$LOG_DIR"/*.log 2>/dev/null || {
    echo "Starting to follow logs..."
    if [ -f "$LOG_DIR/read-module.log" ]; then
        sudo tail -f "$LOG_DIR/read-module.log" &
    fi
    if [ -f "$LOG_DIR/trigger-module.log" ]; then
        sudo tail -f "$LOG_DIR/trigger-module.log" &
    fi
    wait
}

