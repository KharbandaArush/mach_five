# Configuration Files You Need to Update

Before building and testing locally, you need to create/update these configuration files:

## 1. Google Sheets Credentials (REQUIRED for Read Module)

**File:** `config/google-credentials.json`

**What it is:** Service account credentials for accessing Google Sheets API

**How to get it:**
1. Go to https://console.cloud.google.com/
2. Create/select a project
3. Enable "Google Sheets API"
4. Create a Service Account
5. Download the JSON key file
6. Save it as `config/google-credentials.json`
7. Share your Google Sheet with the service account email (found in the JSON)

**Current status:** ❌ You need to create this file

**Example location:** `config/google-credentials.json.example` (template only)

---

## 2. Broker Configuration (REQUIRED)

**File:** `config/broker-config.json`

**What it is:** Broker API configuration for executing trades

**For local testing (Mock Broker - No real trades):**
```json
{
  "type": "mock",
  "api_key": "",
  "api_secret": "",
  "base_url": "",
  "rate_limit": {
    "requests_per_second": 10,
    "burst_size": 20
  }
}
```

**For production with Kite (Zerodha) - REQUIRED:**

```json
{
  "type": "kite",
  "api_key": "your-kite-api-key",
  "api_secret": "your-kite-access-token",
  "base_url": "https://kite.zerodha.com",
  "rate_limit": {
    "requests_per_second": 3,
    "burst_size": 5
  }
}
```

**Important:** For Kite, the `api_secret` field stores your **access token** (not API secret). See `KITE_SETUP.md` for detailed instructions.

**For production with Alpaca (or other brokers):**
```json
{
  "type": "alpaca",
  "api_key": "your-actual-api-key",
  "api_secret": "your-actual-api-secret",
  "base_url": "https://paper-api.alpaca.markets",
  "rate_limit": {
    "requests_per_second": 10,
    "burst_size": 20
  }
}
```

**Current status:** ✅ Can use example file for testing (mock broker)

**Quick setup:**
```bash
cp config/broker-config.json.example config/broker-config.json
# Edit if needed, or leave as-is for mock broker testing
```

---

## 3. Environment Variables (OPTIONAL - has defaults)

You can set these or use the defaults:

```bash
# Google Sheets
export GOOGLE_SHEET_ID="your-sheet-id-from-url"
export GOOGLE_SHEET_BUY_RANGE="to_buy!B2:J"
export GOOGLE_SHEET_SELL_RANGE="to_sell!B2:J"

# Redis (defaults to localhost:6379)
export REDIS_ADDR="localhost:6379"

# Logging (defaults to INFO)
export LOG_LEVEL="INFO"
```

**How to get Google Sheet ID:**
- Open your Google Sheet
- Look at the URL: `https://docs.google.com/spreadsheets/d/SHEET_ID_HERE/edit`
- Copy the `SHEET_ID_HERE` part

---

## Quick Start Commands

```bash
# 1. Run setup script
./setup-local.sh

# 2. Create Google credentials file (you need to do this manually)
#    See CONFIG_SETUP.md for detailed instructions

# 3. Copy broker config (for mock testing)
cp config/broker-config.json.example config/broker-config.json

# 4. Set your Google Sheet ID
export GOOGLE_SHEET_ID="your-sheet-id-here"

# 5. Make sure Redis is running
redis-server  # or: brew services start redis

# 6. Build and test
make build
./test-local.sh
```

---

## Summary

| File | Status | Action Required |
|------|--------|----------------|
| `config/google-credentials.json` | ❌ Missing | **YOU MUST CREATE THIS** - See CONFIG_SETUP.md |
| `config/broker-config.json` | ⚠️ Can create | Copy from example for mock testing |
| Environment variables | ✅ Optional | Set `GOOGLE_SHEET_ID` at minimum |

**Minimum to test:**
1. Create `config/google-credentials.json` (Google service account JSON)
2. Copy `config/broker-config.json.example` to `config/broker-config.json`
3. Set `GOOGLE_SHEET_ID` environment variable
4. Have Redis running

