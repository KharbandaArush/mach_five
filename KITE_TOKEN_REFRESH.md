# Kite Token Refresh Mechanism

The trading system now includes automatic token refresh for Kite Connect API. This ensures that access tokens are automatically refreshed before they expire, preventing authentication failures.

## How It Works

1. **Automatic Refresh**: The system automatically refreshes access tokens:
   - **Proactively**: 1-2 hours before token expiry (background check every hour)
   - **On Error**: When a 401/403 error is detected, the system attempts to refresh the token and retry the operation

2. **Token Persistence**: When tokens are refreshed, they are automatically saved back to the `broker-config.json` file

3. **Thread-Safe**: Token refresh is thread-safe using mutex locks to prevent race conditions

## Configuration

Add the `refresh_token` field to your `broker-config.json`:

```json
{
  "type": "kite",
  "api_key": "your-kite-api-key",
  "api_secret": "your-kite-access-token",
  "refresh_token": "your-kite-refresh-token",
  "base_url": "https://kite.zerodha.com",
  "rate_limit": {
    "requests_per_second": 10,
    "burst_size": 20
  }
}
```

## Getting Your Refresh Token

### Method 1: From Initial Login Response

When you first authenticate with Kite Connect, you receive both `access_token` and `refresh_token`:

```python
from kiteconnect import KiteConnect

kite = KiteConnect(api_key="your-api-key")
data = kite.generate_session("request_token", api_secret="your-api-secret")
print("Access Token:", data["access_token"])
print("Refresh Token:", data["refresh_token"])  # Save this!
```

### Method 2: Using Kite Connect Login Flow

1. Go to https://kite.trade/connect/login?api_key=YOUR_API_KEY
2. Complete the login flow
3. You'll be redirected with a `request_token` in the URL
4. Exchange the `request_token` for `access_token` and `refresh_token`:

```python
from kiteconnect import KiteConnect

kite = KiteConnect(api_key="your-api-key")
data = kite.generate_session("request_token_from_url", api_secret="your-api-secret")
refresh_token = data["refresh_token"]  # Save this!
```

### Method 3: Using Kite Connect Web Interface

1. Log in to https://kite.trade/
2. Go to Settings > API
3. Generate a new access token
4. The refresh token will be provided along with the access token

## Important Notes

1. **Refresh Token Expiry**: Refresh tokens typically don't expire (or expire after a very long time), but access tokens expire every 24 hours

2. **Token Security**: 
   - Keep your `broker-config.json` file secure (it contains sensitive tokens)
   - The file is automatically updated with new tokens when refreshed
   - File permissions are set to 0600 (read/write for owner only)

3. **Without Refresh Token**: 
   - If `refresh_token` is not provided, the system will still work but won't auto-refresh
   - You'll need to manually update `api_secret` (access token) when it expires
   - The system will log a warning: `⚠️  No refresh token configured`

## Logs

When token refresh occurs, you'll see logs like:

```
🔄 Refreshing Kite access token...
✅ Token refreshed successfully
   Old token: xxxxx...
   New token: yyyyy...
   Expires at: 2025-11-08 13:45:27 IST
```

If refresh fails:

```
❌ Token refresh failed
   Status Code: 400
   Response: {"status": "error", "message": "Invalid refresh token"}
```

## Troubleshooting

### Token Refresh Fails

1. **Check Refresh Token**: Ensure the refresh token is correct and hasn't been revoked
2. **Check API Key**: Verify the API key matches the refresh token
3. **Check Network**: Ensure the system can reach `https://kite.zerodha.com`

### Tokens Not Persisting

1. **Check File Permissions**: Ensure the system has write access to `broker-config.json`
2. **Check Logs**: Look for warnings about config file updates
3. **Manual Update**: If auto-update fails, manually update the file with new tokens

### Still Getting 401 Errors

1. **Verify Refresh Token**: The refresh token might be invalid or expired
2. **Check Logs**: Look for token refresh attempts and their results
3. **Manual Refresh**: Try manually refreshing the token using Kite Connect API

## Testing Token Refresh

To test if token refresh is working:

1. Set an expired access token in `broker-config.json`
2. Ensure `refresh_token` is set correctly
3. Run the trigger module - it should automatically refresh the token
4. Check logs for refresh messages
5. Verify `broker-config.json` has been updated with new tokens


