# Configuration Files Summary

## Files You Need to Update Before Testing/Deployment

### 1. ✅ `config/broker-config.json` - **REQUIRED**

**Status:** Already created from example (currently set to mock)

**Action Required:** Update with your Kite credentials

**For Kite (Production):**
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

**How to get Kite credentials:**
1. Go to https://developers.kite.trade/
2. Create a new app
3. Get your API Key
4. Generate an Access Token (see `KITE_SETUP.md` for details)
5. Update the config file

**Note:** `api_secret` field stores the **access token** for Kite (not the API secret)

---

### 2. ❌ `config/google-credentials.json` - **REQUIRED for Read Module**

**Status:** Not created yet

**Action Required:** Create this file with your Google Service Account credentials

**Steps:**
1. Go to https://console.cloud.google.com/
2. Create/select a project
3. Enable "Google Sheets API"
4. Create a Service Account
5. Download the JSON key file
6. Save as `config/google-credentials.json`
7. Share your Google Sheet with the service account email

**See `CONFIG_SETUP.md` for detailed instructions**

---

### 3. ⚠️ Environment Variables - **OPTIONAL** (has defaults)

You can set these or use defaults:

```bash
# Required for Read Module
export GOOGLE_SHEET_ID="your-sheet-id-from-url"

# Optional (has defaults)
export GOOGLE_SHEET_RANGE="Sheet1!A2:G100"
export REDIS_ADDR="localhost:6379"
export LOG_LEVEL="INFO"
export WORKER_POOL_SIZE=5
```

**How to get Google Sheet ID:**
- Open your Google Sheet
- URL format: `https://docs.google.com/spreadsheets/d/SHEET_ID_HERE/edit`
- Copy the `SHEET_ID_HERE` part

---

## Quick Setup Checklist

- [ ] Update `config/broker-config.json` with Kite credentials
- [ ] Create `config/google-credentials.json` with Google Service Account JSON
- [ ] Set `GOOGLE_SHEET_ID` environment variable
- [ ] Ensure Redis is running (`redis-server` or `brew services start redis`)
- [ ] Build the system: `make build` (already done ✅)
- [ ] Test locally: `./test-local.sh`

---

## Testing Order

1. **First:** Test with mock broker (no real trades)
   - Keep `config/broker-config.json` as `"type": "mock"`
   - Test read and trigger modules

2. **Then:** Test with Kite (paper trading recommended first)
   - Update `config/broker-config.json` with Kite credentials
   - Use small quantities for testing
   - Monitor logs and Kite app

---

## Symbol Format for Google Sheets

When using Kite, symbols should be in one of these formats:

- `NSE:RELIANCE` - Reliance on NSE
- `BSE:RELIANCE` - Reliance on BSE  
- `RELIANCE` - Reliance on NSE (defaults to NSE if no exchange specified)

---

## Next Steps

1. Read `KITE_SETUP.md` for detailed Kite Connect setup
2. Read `CONFIG_SETUP.md` for Google Sheets setup
3. Run `./setup-local.sh` to verify everything is ready
4. Run `./test-local.sh` to test the system


