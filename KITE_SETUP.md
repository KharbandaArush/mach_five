# Kite Connect Setup Guide

This guide explains how to set up Zerodha Kite Connect API for the trading system.

## Prerequisites

1. Zerodha Kite account
2. Kite Connect API credentials (API Key and Access Token)

## Getting Kite Connect Credentials

### Step 1: Create Kite Connect App

1. Log in to [Kite Connect Developer Portal](https://developers.kite.trade/)
2. Go to "My Apps" section
3. Click "Create new app"
4. Fill in the details:
   - App name: Your app name
   - Redirect URL: `http://localhost:8080/callback` (for local testing)
   - App type: Trading API
5. Save and note down your **API Key** and **API Secret**

### Step 2: Generate Access Token

You need to generate an access token using the Kite Connect login flow. There are two methods:

#### Method 1: Using Kite Connect Login (Recommended for Production)

1. Use the Kite Connect login URL:
   ```
   https://kite.zerodha.com/connect/login?api_key=YOUR_API_KEY&v=3
   ```
2. Complete the login flow
3. You'll be redirected to your callback URL with a `request_token`
4. Exchange the `request_token` for an `access_token` using the API

#### Method 2: Manual Token Generation (For Testing)

1. Use Kite Connect Python/JavaScript library to generate token
2. Or use the Kite Connect web interface to generate token
3. Access tokens are valid until you logout or regenerate

### Step 3: Configure the System

Update `config/broker-config.json`:

```json
{
  "type": "kite",
  "api_key": "your-api-key-here",
  "api_secret": "your-access-token-here",
  "base_url": "https://kite.zerodha.com",
  "rate_limit": {
    "requests_per_second": 3,
    "burst_size": 5
  }
}
```

**Important Notes:**
- `api_secret` field is used to store the **access token** (not the API secret)
- For paper trading, use: `https://kite.zerodha.com/connect/login?api_key=YOUR_API_KEY&v=3`
- Access tokens expire when you logout or after a period of inactivity

## Symbol Format

The system expects symbols in one of these formats:

1. **With Exchange:** `NSE:RELIANCE`, `BSE:RELIANCE`
2. **Without Exchange:** `RELIANCE` (defaults to NSE)

Examples:
- `NSE:RELIANCE` - Reliance on NSE
- `BSE:RELIANCE` - Reliance on BSE
- `RELIANCE` - Reliance on NSE (default)

## Order Types Supported

- **MARKET** - Market orders (executed immediately at market price)
- **LIMIT** - Limit orders (executed at specified price or better)

## After Market Orders (AMO)

The system automatically places After Market Orders (AMO) when the market is closed:

- **Market Hours**: 9:00 AM - 3:30 PM IST (Monday to Friday)
- **Before 9:00 AM**: Orders placed as AMO, executed when market opens
- **After 3:30 PM**: Orders placed as AMO, executed next market day
- **Weekends**: Orders placed as AMO, executed next Monday

AMO orders are automatically queued by Kite and executed at market open. The system logs when AMO orders are placed and their expected execution time.

## Product Types

Currently defaults to **MIS** (Intraday). To change:
- Edit `internal/broker/kite.go` and modify the `Product` field
- Options: `MIS` (Intraday), `CNC` (Delivery), `NRML` (Carry Forward)

## Rate Limits

Kite Connect has rate limits:
- **Production:** 3 requests per second
- **Burst:** 5 requests

The system is configured with these limits by default. Adjust in `broker-config.json` if needed.

## Testing

1. Use paper trading mode first
2. Start with small quantities
3. Monitor logs for any errors
4. Check order status on Kite web/mobile app

## Security

- **Never commit** `broker-config.json` with real credentials to version control
- Store credentials securely
- Rotate access tokens regularly
- Use environment variables for production:
  ```bash
  export BROKER_API_KEY="your-key"
  export BROKER_API_SECRET="your-token"
  ```

## Troubleshooting

### "Invalid API Key or Access Token"
- Verify your API key is correct
- Check if access token has expired (regenerate if needed)
- Ensure you're using the correct base URL

### "Order rejected"
- Check if market is open
- Verify symbol format is correct
- Check if you have sufficient margin
- Verify order parameters (quantity, price, etc.)

### "Rate limit exceeded"
- Reduce `requests_per_second` in config
- Add delays between orders
- Check Kite Connect rate limit documentation

## Additional Resources

- [Kite Connect Documentation](https://kite.trade/docs/connect/v3/)
- [Kite Connect API Reference](https://kite.trade/docs/connect/v3/orders/)
- [Kite Connect GitHub](https://github.com/zerodha/kiteconnect)


