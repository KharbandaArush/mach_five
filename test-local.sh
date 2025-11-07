#!/bin/bash

# Local testing script

set -e

echo "Testing Trading System Locally"
echo "================================"
echo ""

# Check if binary exists
if [ ! -f "trading-system" ]; then
    echo "❌ Binary not found. Run 'make build' first."
    exit 1
fi

# Check Redis
echo "Checking Redis connection..."
if ! redis-cli ping &> /dev/null; then
    echo "❌ Redis is not running. Start it with: redis-server"
    exit 1
fi
echo "✅ Redis is running"
echo ""

# Check config files
echo "Checking configuration files..."
if [ ! -f "config/google-credentials.json" ]; then
    echo "⚠️  WARNING: config/google-credentials.json not found"
    echo "   The read module will fail without this file."
fi

if [ ! -f "config/broker-config.json" ]; then
    echo "⚠️  WARNING: config/broker-config.json not found"
    echo "   Creating from example..."
    cp config/broker-config.json.example config/broker-config.json
fi
echo "✅ Configuration files checked"
echo ""

# Set default environment variables if not set
export GOOGLE_SHEETS_CREDENTIALS_PATH=${GOOGLE_SHEETS_CREDENTIALS_PATH:-./config/google-credentials.json}
export GOOGLE_SHEET_ID=${GOOGLE_SHEET_ID:-}
export GOOGLE_SHEET_RANGE=${GOOGLE_SHEET_RANGE:-Sheet1!A2:G100}
export REDIS_ADDR=${REDIS_ADDR:-localhost:6379}
export BROKER_CONFIG_PATH=${BROKER_CONFIG_PATH:-./config/broker-config.json}
export LOG_LEVEL=${LOG_LEVEL:-INFO}
export WORKER_POOL_SIZE=${WORKER_POOL_SIZE:-5}

echo "Environment variables:"
echo "  GOOGLE_SHEETS_CREDENTIALS_PATH=$GOOGLE_SHEETS_CREDENTIALS_PATH"
echo "  GOOGLE_SHEET_ID=$GOOGLE_SHEET_ID"
echo "  REDIS_ADDR=$REDIS_ADDR"
echo "  BROKER_CONFIG_PATH=$BROKER_CONFIG_PATH"
echo "  LOG_LEVEL=$LOG_LEVEL"
echo ""

# Menu
echo "What would you like to test?"
echo "1. Test Read Module (reads from Google Sheets)"
echo "2. Test Trigger Module (executes orders from cache)"
echo "3. Test both (read in background, then trigger)"
echo "4. Exit"
echo ""
read -p "Enter choice [1-4]: " choice

case $choice in
    1)
        echo ""
        echo "Starting Read Module..."
        echo "Press Ctrl+C to stop"
        echo ""
        ./trading-system -module=read
        ;;
    2)
        echo ""
        echo "Starting Trigger Module..."
        echo "This will execute any orders in cache that are due"
        echo ""
        ./trading-system -module=trigger
        ;;
    3)
        echo ""
        echo "Starting Read Module in background..."
        ./trading-system -module=read &
        READ_PID=$!
        echo "Read module PID: $READ_PID"
        echo "Waiting 5 seconds for orders to be cached..."
        sleep 5
        echo ""
        echo "Starting Trigger Module..."
        ./trading-system -module=trigger
        echo ""
        echo "Stopping Read Module..."
        kill $READ_PID 2>/dev/null || true
        echo "Done"
        ;;
    4)
        echo "Exiting..."
        exit 0
        ;;
    *)
        echo "Invalid choice"
        exit 1
        ;;
esac


