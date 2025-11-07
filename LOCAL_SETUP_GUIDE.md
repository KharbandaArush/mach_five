# Local Setup Guide - Complete Instructions

This guide will help you set up the trading system locally so it can execute orders.

## Prerequisites Checklist

- [x] Go installed (✅ Already installed)
- [x] Redis running (✅ Already running)
- [x] Binary built (✅ trading-system exists)
- [ ] Google Sheets credentials configured
- [ ] Google Sheet ID set
- [ ] Broker config (Kite) configured
- [ ] Environment variables set

---

## Step 1: Set Up Google Sheets Credentials

### Option A: Use Existing Credentials (If you have them)

If you already have a `config/google-credentials.json` file, verify it's valid:

```bash
# Check if file exists and is readable
cat config/google-credentials.json | jq . 2>/dev/null && echo "✅ Valid JSON" || echo "❌ Invalid JSON"
```

### Option B: Create New Google Service Account

1. **Go to Google Cloud Console:**
   - Visit: https://console.cloud.google.com/
   - Sign in with your Google account

2. **Create or Select a Project:**
   - Click on project dropdown at top
   - Click "New Project" or select existing one
   - Note the project name

3. **Enable Google Sheets API:**
   - Go to "APIs & Services" > "Library"
   - Search for "Google Sheets API"
   - Click on it and click "Enable"

4. **Create Service Account:**
   - Go to "APIs & Services" > "Credentials"
   - Click "Create Credentials" > "Service Account"
   - Service account name: `trading-system-reader`
   - Click "Create and Continue"
   - Skip role assignment (click "Continue")
   - Click "Done"

5. **Create and Download Key:**
   - Click on the service account you just created
   - Go to "Keys" tab
   - Click "Add Key" > "Create new key"
   - Select "JSON" format
   - Click "Create"
   - The JSON file will download automatically

6. **Save the Credentials:**
   ```bash
   # Move the downloaded file to the config directory
   mv ~/Downloads/your-project-*.json config/google-credentials.json
   ```

7. **Share Your Google Sheet:**
   - Open your Google Sheet
   - Click "Share" button
   - Get the service account email from the JSON file:
     ```bash
     cat config/google-credentials.json | grep client_email
     ```
   - Add the service account email (e.g., `trading-system-reader@your-project.iam.gserviceaccount.com`)
   - Give it "Viewer" or "Editor" permissions
   - Click "Send"

---

## Step 2: Get Your Google Sheet ID

1. **Open your Google Sheet** in a browser
2. **Look at the URL:**
   ```
   https://docs.google.com/spreadsheets/d/SHEET_ID_HERE/edit#gid=0
   ```
3. **Copy the SHEET_ID_HERE part** (the long string between `/d/` and `/edit`)

Example:
- URL: `https://docs.google.com/spreadsheets/d/1a2b3c4d5e6f7g8h9i0j/edit`
- Sheet ID: `1a2b3c4d5e6f7g8h9i0j`

4. **Verify your sheet has the correct structure:**
   - Sheet must have two tabs: `to_buy` and `to_sell`
   - Each sheet should have headers in row 1
   - Data should start from row 2
   - Columns B through J should contain: planned_buy_price, product, Name, bse_code, symbol, execute_date, execute_time, Money Needed, Lots

---

## Step 3: Set Up Broker Configuration (Kite)

### Get Kite Connect Credentials

1. **Go to Kite Connect Developer Portal:**
   - Visit: https://developers.kite.trade/
   - Sign in with your Zerodha account

2. **Create a New App:**
   - Go to "My Apps" section
   - Click "Create new app"
   - Fill in:
     - App name: `Trading System`
     - Redirect URL: `http://localhost:8080/callback` (for local testing)
     - App type: Trading API
   - Click "Create"
   - **Note down your API Key**

3. **Generate Access Token:**
   
   **Method 1: Using Kite Connect Login (Recommended)**
   - Visit: `https://kite.zerodha.com/connect/login?api_key=YOUR_API_KEY&v=3`
   - Replace `YOUR_API_KEY` with your actual API key
   - Complete the login flow
   - You'll be redirected with a `request_token`
   - Exchange the `request_token` for an `access_token` using Kite Connect API
   
   **Method 2: Using Python Script (Quick)**
   ```python
   from kiteconnect import KiteConnect
   
   api_key = "your_api_key"
   api_secret = "your_api_secret"
   request_token = "request_token_from_login"
   
   kite = KiteConnect(api_key=api_key)
   data = kite.generate_session(request_token, api_secret=api_secret)
   access_token = data["access_token"]
   print(f"Access Token: {access_token}")
   ```

4. **Update Broker Config:**
   ```bash
   # Edit the broker config file
   nano config/broker-config.json
   ```
   
   Update with your credentials:
   ```json
   {
     "type": "kite",
     "api_key": "your-kite-api-key-here",
     "api_secret": "your-kite-access-token-here",
     "base_url": "https://kite.zerodha.com",
     "rate_limit": {
       "requests_per_second": 3,
       "burst_size": 5
     }
   }
   ```
   
   **Important:** The `api_secret` field stores your **access token** (not the API secret)

---

## Step 4: Set Environment Variables

Create a setup script or export variables:

```bash
# Create a setup script
cat > setup-env.sh << 'EOF'
#!/bin/bash

# Google Sheets Configuration
export GOOGLE_SHEETS_CREDENTIALS_PATH=./config/google-credentials.json
export GOOGLE_SHEET_ID="YOUR_SHEET_ID_HERE"  # Replace with your actual sheet ID
export GOOGLE_SHEET_BUY_RANGE=to_buy!B2:J
export GOOGLE_SHEET_SELL_RANGE=to_sell!B2:J
export GOOGLE_SHEETS_REFRESH_INTERVAL=30s

# Redis Configuration
export REDIS_ADDR=localhost:6379
export REDIS_PASSWORD=""
export REDIS_DB=0

# Broker Configuration
export BROKER_CONFIG_PATH=./config/broker-config.json

# Logging Configuration
export LOG_LEVEL=INFO
export READ_LOG_PATH=./logs/read-module.log
export TRIGGER_LOG_PATH=./logs/trigger-module.log
export BROKER_LOG_PATH=./logs/broker-module.log

# Trigger Module Configuration
export WORKER_POOL_SIZE=5

echo "✅ Environment variables set"
echo "GOOGLE_SHEET_ID: $GOOGLE_SHEET_ID"
EOF

chmod +x setup-env.sh
```

**Or export manually:**
```bash
export GOOGLE_SHEET_ID="your-sheet-id-here"
export GOOGLE_SHEETS_CREDENTIALS_PATH=./config/google-credentials.json
export BROKER_CONFIG_PATH=./config/broker-config.json
export LOG_LEVEL=INFO
export REDIS_ADDR=localhost:6379
```

---

## Step 5: Verify Your Setup

Run this verification script:

```bash
cat > verify-setup.sh << 'EOF'
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
    if cat config/google-credentials.json | jq . > /dev/null 2>&1; then
        echo "   ✅ Google credentials file exists and is valid JSON"
        CLIENT_EMAIL=$(cat config/google-credentials.json | jq -r '.client_email')
        echo "   Service Account: $CLIENT_EMAIL"
    else
        echo "   ❌ Google credentials file exists but is invalid JSON"
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
    BROKER_TYPE=$(cat config/broker-config.json | jq -r '.type')
    API_KEY=$(cat config/broker-config.json | jq -r '.api_key')
    if [ "$BROKER_TYPE" != "null" ] && [ "$API_KEY" != "null" ] && [ "$API_KEY" != "" ]; then
        echo "   ✅ Broker config exists"
        echo "   Type: $BROKER_TYPE"
        echo "   API Key: ${API_KEY:0:10}... (hidden)"
    else
        echo "   ⚠️  Broker config exists but may be incomplete"
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
EOF

chmod +x verify-setup.sh
./verify-setup.sh
```

---

## Step 6: Test the System

### Test 1: Test Read Module

```bash
# Source your environment
source setup-env.sh  # or export variables manually

# Run read module for a few seconds
./trading-system -module=read &
READ_PID=$!
sleep 5
kill $READ_PID 2>/dev/null || true

# Check logs
echo "=== Read Module Logs ==="
tail -20 logs/read-module.log
```

**Expected output:**
- Should connect to Google Sheets
- Should read orders from to_buy and to_sell sheets
- Should cache orders in Redis

### Test 2: Test Trigger Module

```bash
# Run trigger module
./trading-system -module=trigger

# Check logs
echo "=== Trigger Module Logs ==="
tail -20 logs/trigger-module.log
```

**Expected output:**
- Should check Redis for orders
- Should execute orders if any are due
- Should log execution results

### Test 3: Check Redis Cache

```bash
# Check if orders are cached
echo "=== Cached Orders ==="
redis-cli ZRANGE pending_orders 0 -1

# Check order details
redis-cli KEYS "order:*" | head -3 | while read key; do
    echo "Order: $key"
    redis-cli GET "$key" | jq . 2>/dev/null || redis-cli GET "$key"
done
```

---

## Step 7: Run Full System Test

```bash
# Create a test script
cat > test-system.sh << 'EOF'
#!/bin/bash

source setup-env.sh

echo "=== Starting Full System Test ==="
echo ""

# Start read module in background
echo "1. Starting Read Module..."
./trading-system -module=read > logs/read-module.log 2>&1 &
READ_PID=$!
echo "   Read module PID: $READ_PID"
sleep 10

# Run trigger module
echo ""
echo "2. Running Trigger Module..."
./trading-system -module=trigger > logs/trigger-module.log 2>&1

# Stop read module
echo ""
echo "3. Stopping Read Module..."
kill $READ_PID 2>/dev/null || true
wait $READ_PID 2>/dev/null || true

# Show logs
echo ""
echo "=== READ MODULE LOGS ==="
tail -30 logs/read-module.log

echo ""
echo "=== TRIGGER MODULE LOGS ==="
tail -30 logs/trigger-module.log

echo ""
echo "=== Test Complete ==="
EOF

chmod +x test-system.sh
./test-system.sh
```

---

## Troubleshooting

### Issue: "private key should be a PEM" error

**Solution:** Your Google credentials JSON might be corrupted or in wrong format.
- Re-download the credentials JSON from Google Cloud Console
- Make sure it's the complete file, not truncated

### Issue: "GOOGLE_SHEET_ID is empty"

**Solution:** 
```bash
export GOOGLE_SHEET_ID="your-actual-sheet-id"
```

### Issue: "No data found in sheet"

**Solution:**
- Verify sheet has `to_buy` and `to_sell` tabs
- Check that data starts from row 2 (row 1 is headers)
- Verify the service account has access to the sheet

### Issue: "Kite API authentication failed"

**Solution:**
- Verify your API key is correct
- Check that access token is valid (not expired)
- For paper trading, make sure you're using the correct base URL

### Issue: "Redis connection failed"

**Solution:**
```bash
# Start Redis
redis-server

# Or on Mac with Homebrew
brew services start redis
```

---

## Quick Reference

**Essential Commands:**
```bash
# Set environment
export GOOGLE_SHEET_ID="your-sheet-id"
export GOOGLE_SHEETS_CREDENTIALS_PATH=./config/google-credentials.json
export BROKER_CONFIG_PATH=./config/broker-config.json

# Run read module
./trading-system -module=read

# Run trigger module
./trading-system -module=trigger

# Check logs
tail -f logs/read-module.log
tail -f logs/trigger-module.log
```

**File Locations:**
- Config: `config/broker-config.json`, `config/google-credentials.json`
- Logs: `logs/read-module.log`, `logs/trigger-module.log`
- Binary: `trading-system`

---

## Next Steps

Once setup is complete:
1. Verify all checks pass with `./verify-setup.sh`
2. Test with `./test-system.sh`
3. Monitor logs for any errors
4. Start using the system for real trading (with caution!)



