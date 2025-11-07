# Quick Setup Summary

## 🚀 Fast Track Setup (5 minutes)

### Step 1: Google Sheets Setup (2 min)

1. **Get your Sheet ID:**
   ```bash
   # From your Google Sheet URL:
   # https://docs.google.com/spreadsheets/d/YOUR_SHEET_ID/edit
   export GOOGLE_SHEET_ID="YOUR_SHEET_ID"
   ```

2. **Verify credentials:**
   ```bash
   # Check if credentials file exists
   ls -lh config/google-credentials.json
   
   # If missing, follow LOCAL_SETUP_GUIDE.md Step 1
   ```

### Step 2: Broker Setup (2 min)

1. **Edit broker config:**
   ```bash
   nano config/broker-config.json
   ```

2. **Update with Kite credentials:**
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

### Step 3: Run Setup Script (1 min)

```bash
# Interactive setup
./quick-setup.sh

# Or verify current setup
./verify-setup.sh
```

### Step 4: Test

```bash
# Set environment
export GOOGLE_SHEET_ID="your-sheet-id"
export GOOGLE_SHEETS_CREDENTIALS_PATH=./config/google-credentials.json
export BROKER_CONFIG_PATH=./config/broker-config.json

# Test read module
./trading-system -module=read &
sleep 5
pkill -f "trading-system -module=read"

# Test trigger module
./trading-system -module=trigger

# Check logs
tail -20 logs/read-module.log
tail -20 logs/trigger-module.log
```

---

## 📋 What You Need

### Required Files:
1. ✅ `config/google-credentials.json` - Google Service Account JSON
2. ✅ `config/broker-config.json` - Kite broker config
3. ✅ `GOOGLE_SHEET_ID` environment variable

### Required Services:
1. ✅ Redis running (`redis-server`)
2. ✅ Google Sheet shared with service account email

### Required Credentials:
1. **Google:** Service account JSON (from Google Cloud Console)
2. **Kite:** API Key + Access Token (from Kite Connect)

---

## 🔍 Quick Verification

```bash
# Run verification
./verify-setup.sh

# Check Redis
redis-cli ping

# Check config files
ls -lh config/*.json

# Check environment
echo $GOOGLE_SHEET_ID
```

---

## 📚 Full Documentation

- **Detailed Setup:** `LOCAL_SETUP_GUIDE.md`
- **Config Reference:** `CONFIG_SETUP.md`
- **Kite Setup:** `KITE_SETUP.md`

---

## ⚡ One-Liner Setup (if you have credentials)

```bash
export GOOGLE_SHEET_ID="your-sheet-id" && \
export GOOGLE_SHEETS_CREDENTIALS_PATH=./config/google-credentials.json && \
export BROKER_CONFIG_PATH=./config/broker-config.json && \
./trading-system -module=read
```



