# Configuration Setup Guide

Before building and testing the system locally, you need to set up the following configuration files.

## Required Configuration Files

### 1. Google Sheets Credentials (`config/google-credentials.json`)

**Steps to create:**

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select an existing one
3. Enable the **Google Sheets API**:
   - Navigate to "APIs & Services" > "Library"
   - Search for "Google Sheets API"
   - Click "Enable"
4. Create a Service Account:
   - Go to "APIs & Services" > "Credentials"
   - Click "Create Credentials" > "Service Account"
   - Give it a name (e.g., "trading-system-reader")
   - Click "Create and Continue"
   - Skip role assignment (or assign "Editor" if needed)
   - Click "Done"
5. Create a key for the service account:
   - Click on the service account you just created
   - Go to "Keys" tab
   - Click "Add Key" > "Create new key"
   - Select "JSON" format
   - Download the JSON file
6. Save the downloaded file as `config/google-credentials.json`
7. Share your Google Sheet with the service account email:
   - Open your Google Sheet
   - Click "Share"
   - Add the service account email (found in the JSON file as `client_email`)
   - Give it "Viewer" or "Editor" permissions

**File location:** `config/google-credentials.json`

### 2. Broker Configuration (`config/broker-config.json`)

**For local testing with mock broker:**

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

**For production with Kite (Zerodha):**

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

**Note:** For Kite, `api_secret` field stores the **access token** (not API secret). See `KITE_SETUP.md` for detailed setup instructions.

**For production with Alpaca (or other brokers):**

```json
{
  "type": "alpaca",
  "api_key": "your-alpaca-api-key",
  "api_secret": "your-alpaca-api-secret",
  "base_url": "https://paper-api.alpaca.markets",
  "rate_limit": {
    "requests_per_second": 10,
    "burst_size": 20
  }
}
```

**File location:** `config/broker-config.json`

**Note:** You can also set broker config via environment variables:
- `BROKER_TYPE`
- `BROKER_API_KEY`
- `BROKER_API_SECRET`
- `BROKER_BASE_URL`

### 3. Environment Variables

Create a `.env` file or export these variables:

```bash
# Google Sheets Configuration
export GOOGLE_SHEETS_CREDENTIALS_PATH=./config/google-credentials.json
export GOOGLE_SHEET_ID=your-google-sheet-id
export GOOGLE_SHEET_BUY_RANGE=to_buy!B2:J
export GOOGLE_SHEET_SELL_RANGE=to_sell!B2:J
export GOOGLE_SHEETS_REFRESH_INTERVAL=30s

# Redis Configuration
export REDIS_ADDR=localhost:6379
export REDIS_PASSWORD=
export REDIS_DB=0

# Broker Configuration (optional if using config file)
export BROKER_CONFIG_PATH=./config/broker-config.json
export BROKER_TYPE=mock

# Logging Configuration
export LOG_LEVEL=INFO
export READ_LOG_PATH=./logs/read-module.log
export TRIGGER_LOG_PATH=./logs/trigger-module.log
export BROKER_LOG_PATH=./logs/broker-module.log

# Trigger Module Configuration
export WORKER_POOL_SIZE=5
```

## Quick Setup Commands

```bash
# 1. Copy example config files
cp config/broker-config.json.example config/broker-config.json
cp config/google-credentials.json.example config/broker-config.json

# 2. Edit the files with your actual values
# - Update config/google-credentials.json with your service account JSON
# - Update config/broker-config.json with your broker settings (or leave as mock for testing)

# 3. Create logs directory
mkdir -p logs

# 4. Get your Google Sheet ID from the URL:
# https://docs.google.com/spreadsheets/d/SHEET_ID_HERE/edit
# The SHEET_ID_HERE is what you need for GOOGLE_SHEET_ID
```

## Google Sheet Format

Your Google Sheet should have **two sub-sheets**: `to_buy` and `to_sell`

Each sheet should have the following structure (starting from row 2, row 1 is header):

| Column | Header | Description | Example |
|--------|--------|-------------|---------|
| B | planned_buy_price | Order price (float) | 150.50 |
| C | product | Product type | MIS |
| D | Name | Stock name | Reliance Industries |
| E | bse_code | BSE code | 500325 |
| F | symbol | Trading symbol | NSE:RELIANCE or BSE:RELIANCE |
| G | execute_date | Execution date (YYYY-MM-DD) | 2024-01-15 |
| H | execute_time | Execution time (HH:MM:SS) | 09:30:00 |
| I | Money Needed | Money required (float) | 15000.00 |
| J | Lots | Quantity/Lots (int) | 10 |

**Sheet Structure:**
- **to_buy** sheet: Contains buy orders (side = "Buy")
- **to_sell** sheet: Contains sell orders (side = "Sell")

**Date format:** YYYY-MM-DD (e.g., 2024-01-15)
**Time format:** HH:MM:SS or HH:MM (e.g., 09:30:00 or 09:30)
**Symbol format:** 
- With exchange: `NSE:RELIANCE`, `BSE:RELIANCE`
- Without exchange: `RELIANCE` (defaults to NSE)
**Quantity:** Integer from "Lots" column (defaults to 1 if invalid)

## Testing Checklist

Before running the system:

- [ ] Google Sheets credentials file exists at `config/google-credentials.json`
- [ ] Google Sheet is shared with the service account email
- [ ] Broker config file exists at `config/broker-config.json`
- [ ] Redis is running (`redis-server` or `brew services start redis` on Mac)
- [ ] Environment variables are set (or use defaults)
- [ ] Logs directory exists (`mkdir -p logs`)
- [ ] Google Sheet has test data in the correct format

## Next Steps

After setting up the configuration files:

1. Install dependencies: `go mod download`
2. Build the system: `make build`
3. Start Redis: `redis-server` (or `brew services start redis` on Mac)
4. Test read module: `./trading-system -module=read`
5. Test trigger module: `./trading-system -module=trigger`

