#!/bin/bash

# Quick Setup Script for Local Execution

set -e

echo "=== Trading System - Quick Setup ==="
echo ""

# Check prerequisites
echo "1. Checking prerequisites..."

if ! command -v redis-cli &> /dev/null; then
    echo "   ❌ Redis CLI not found. Install Redis first."
    exit 1
fi

if ! redis-cli ping > /dev/null 2>&1; then
    echo "   ❌ Redis is not running. Start it with: redis-server"
    exit 1
fi
echo "   ✅ Redis is running"

if [ ! -f "trading-system" ]; then
    echo "   ❌ Binary not found. Run: make build"
    exit 1
fi
echo "   ✅ Binary exists"

echo ""

# Check config files
echo "2. Checking configuration files..."

if [ ! -f "config/google-credentials.json" ]; then
    echo "   ❌ Google credentials not found"
    echo "   📝 Create it by:"
    echo "      1. Go to https://console.cloud.google.com/"
    echo "      2. Create service account and download JSON"
    echo "      3. Save as config/google-credentials.json"
    echo ""
    read -p "   Press Enter when credentials file is ready, or Ctrl+C to exit..."
fi

if [ -f "config/google-credentials.json" ]; then
    # Check if jq is available for validation
    if command -v jq > /dev/null 2>&1; then
        if cat config/google-credentials.json | jq . > /dev/null 2>&1; then
            echo "   ✅ Google credentials file is valid JSON"
        else
            echo "   ❌ Google credentials file is invalid JSON"
            echo "   ⚠️  Please check the file format"
            read -p "   Continue anyway? (y/n): " -n 1 -r
            echo
            if [[ ! $REPLY =~ ^[Yy]$ ]]; then
                exit 1
            fi
        fi
    else
        # Basic check without jq
        if grep -q "client_email" config/google-credentials.json && grep -q "private_key" config/google-credentials.json; then
            echo "   ✅ Google credentials file exists (basic validation passed)"
            echo "   💡 Install 'jq' for full JSON validation: brew install jq"
        else
            echo "   ⚠️  Google credentials file exists but may be incomplete"
            echo "   ⚠️  Please verify it contains 'client_email' and 'private_key'"
            read -p "   Continue anyway? (y/n): " -n 1 -r
            echo
            if [[ ! $REPLY =~ ^[Yy]$ ]]; then
                exit 1
            fi
        fi
    fi
fi

if [ ! -f "config/broker-config.json" ]; then
    echo "   ⚠️  Broker config not found. Creating from example..."
    cp config/broker-config.json.example config/broker-config.json
    echo "   ✅ Created broker-config.json (using mock broker)"
    echo "   📝 Edit config/broker-config.json to add Kite credentials"
fi

echo ""

# Get Google Sheet ID
echo "3. Google Sheet Configuration"
if [ -z "$GOOGLE_SHEET_ID" ]; then
    echo "   GOOGLE_SHEET_ID is not set"
    echo ""
    echo "   To get your Sheet ID:"
    echo "   1. Open your Google Sheet"
    echo "   2. Look at the URL: https://docs.google.com/spreadsheets/d/SHEET_ID_HERE/edit"
    echo "   3. Copy the SHEET_ID_HERE part"
    echo ""
    read -p "   Enter your Google Sheet ID: " SHEET_ID
    export GOOGLE_SHEET_ID="$SHEET_ID"
    echo "   ✅ GOOGLE_SHEET_ID set to: $GOOGLE_SHEET_ID"
else
    echo "   ✅ GOOGLE_SHEET_ID is set: $GOOGLE_SHEET_ID"
fi

echo ""

# Set other environment variables
export GOOGLE_SHEETS_CREDENTIALS_PATH=./config/google-credentials.json
export GOOGLE_SHEET_BUY_RANGE=to_buy!B2:J
export GOOGLE_SHEET_SELL_RANGE=to_sell!B2:J
export BROKER_CONFIG_PATH=./config/broker-config.json
export REDIS_ADDR=localhost:6379
export LOG_LEVEL=INFO
export WORKER_POOL_SIZE=5

echo "4. Environment variables set:"
echo "   GOOGLE_SHEET_ID: $GOOGLE_SHEET_ID"
echo "   GOOGLE_SHEETS_CREDENTIALS_PATH: $GOOGLE_SHEETS_CREDENTIALS_PATH"
echo "   BROKER_CONFIG_PATH: $BROKER_CONFIG_PATH"
echo "   REDIS_ADDR: $REDIS_ADDR"
echo ""

# Create logs directory
mkdir -p logs

# Test connection
echo "5. Testing Google Sheets connection..."
if ./trading-system -module=read > /tmp/test-read.log 2>&1 & 
then
    TEST_PID=$!
    sleep 3
    kill $TEST_PID 2>/dev/null || true
    wait $TEST_PID 2>/dev/null || true
    
    if grep -q "Starting Google Sheets reader service" /tmp/test-read.log; then
        echo "   ✅ Read module started successfully"
        if grep -q "Failed to read" /tmp/test-read.log; then
            echo "   ⚠️  Warning: Some errors detected. Check logs:"
            tail -5 /tmp/test-read.log
        fi
    else
        echo "   ❌ Read module failed to start"
        cat /tmp/test-read.log
    fi
fi

echo ""
echo "=== Setup Complete ==="
echo ""
echo "To run the system:"
echo "  1. Read Module:  ./trading-system -module=read"
echo "  2. Trigger Module: ./trading-system -module=trigger"
echo ""
echo "Or use the test script:"
echo "  ./test-system.sh"
echo ""
echo "For detailed setup instructions, see: LOCAL_SETUP_GUIDE.md"


