#!/bin/bash

echo "=== Verifying Local Setup ==="
echo ""

# Check Redis
echo "1. Redis:"
if redis-cli ping > /dev/null 2>&1; then
    echo "   ✅ Redis is running"
else
    echo "   ❌ Redis is not running. Start with: redis-server"
fi

# Check Google credentials
echo ""
echo "2. Google Credentials:"
if [ -f "config/google-credentials.json" ]; then
    if command -v jq > /dev/null 2>&1 && cat config/google-credentials.json | jq . > /dev/null 2>&1; then
        echo "   ✅ Google credentials file exists and is valid JSON"
        CLIENT_EMAIL=$(cat config/google-credentials.json | jq -r '.client_email' 2>/dev/null || echo "N/A")
        echo "   Service Account: $CLIENT_EMAIL"
    elif cat config/google-credentials.json | grep -q "client_email"; then
        echo "   ✅ Google credentials file exists"
        echo "   (Install 'jq' for detailed validation: brew install jq)"
    else
        echo "   ❌ Google credentials file exists but may be invalid"
    fi
else
    echo "   ❌ Google credentials file not found"
fi

# Check Google Sheet ID
echo ""
echo "3. Google Sheet ID:"
if [ -n "$GOOGLE_SHEET_ID" ]; then
    echo "   ✅ GOOGLE_SHEET_ID is set: $GOOGLE_SHEET_ID"
else
    echo "   ❌ GOOGLE_SHEET_ID is not set"
    echo "   Set it with: export GOOGLE_SHEET_ID='your-sheet-id'"
fi

# Check broker config
echo ""
echo "4. Broker Config:"
if [ -f "config/broker-config.json" ]; then
    if command -v jq > /dev/null 2>&1; then
        BROKER_TYPE=$(cat config/broker-config.json | jq -r '.type' 2>/dev/null || echo "unknown")
        API_KEY=$(cat config/broker-config.json | jq -r '.api_key' 2>/dev/null || echo "")
        if [ "$BROKER_TYPE" != "null" ] && [ -n "$API_KEY" ] && [ "$API_KEY" != "null" ]; then
            echo "   ✅ Broker config exists"
            echo "   Type: $BROKER_TYPE"
            echo "   API Key: ${API_KEY:0:10}... (hidden)"
        else
            echo "   ⚠️  Broker config exists but may be incomplete"
        fi
    else
        echo "   ✅ Broker config file exists"
        echo "   (Install 'jq' for detailed validation: brew install jq)"
    fi
else
    echo "   ❌ Broker config file not found"
fi

# Check binary
echo ""
echo "5. Binary:"
if [ -f "trading-system" ]; then
    echo "   ✅ trading-system binary exists"
else
    echo "   ❌ trading-system binary not found. Run: make build"
fi

echo ""
echo "=== Verification Complete ==="
