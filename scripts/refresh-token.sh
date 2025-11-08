#!/bin/bash

# Script to get/refresh Kite access token and update broker-config.json
# Usage: ./refresh-token.sh <request_token|refresh_token> [--request-token] [--api-secret <secret>]

set -e

DEPLOY_PATH="/opt/trading-system"
CONFIG_FILE="$DEPLOY_PATH/config/broker-config.json"
BACKUP_FILE="${CONFIG_FILE}.backup.$(date +%Y%m%d_%H%M%S)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║        Kite Token Management Script                         ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

# Parse arguments
TOKEN=""
TOKEN_TYPE="refresh"
KITE_API_SECRET=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --request-token)
            TOKEN_TYPE="request"
            shift
            ;;
        --api-secret)
            KITE_API_SECRET="$2"
            shift 2
            ;;
        *)
            if [ -z "$TOKEN" ]; then
                TOKEN="$1"
            fi
            shift
            ;;
    esac
done

# Check if token is provided
if [ -z "$TOKEN" ]; then
    echo -e "${RED}❌ Error: Token is required${NC}"
    echo ""
    echo "Usage: $0 <request_token|refresh_token> [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --request-token           - Use request token (to get new access token)"
    echo "  --api-secret <secret>     - Kite API secret (required for checksum calculation)"
    echo ""
    echo "Examples:"
    echo "  $0 HfpmZ8v09RhTGGdiFaEr6UOn5XOzS1P1                    # Refresh existing token"
    echo "  $0 abc123xyz --request-token --api-secret YOUR_SECRET  # Get new token from request token"
    exit 1
fi

# Check if config file exists
if [ ! -f "$CONFIG_FILE" ]; then
    echo -e "${RED}❌ Error: Config file not found: $CONFIG_FILE${NC}"
    exit 1
fi

# Read API key and secret from config
API_KEY=$(grep -o '"api_key"[[:space:]]*:[[:space:]]*"[^"]*"' "$CONFIG_FILE" | cut -d'"' -f4)
API_SECRET=$(grep -o '"api_secret"[[:space:]]*:[[:space:]]*"[^"]*"' "$CONFIG_FILE" | cut -d'"' -f4)
BASE_URL=$(grep -o '"base_url"[[:space:]]*:[[:space:]]*"[^"]*"' "$CONFIG_FILE" | cut -d'"' -f4 || echo "https://kite.zerodha.com")

if [ -z "$API_KEY" ]; then
    echo -e "${RED}❌ Error: Could not find api_key in config file${NC}"
    exit 1
fi

if [ -z "$API_SECRET" ]; then
    echo -e "${YELLOW}⚠️  Warning: api_secret not found in config. Checksum will not be calculated.${NC}"
fi

echo "📋 Configuration:"
echo "   Config file: $CONFIG_FILE"
echo "   API Key: ${API_KEY:0:10}..."
echo "   Base URL: $BASE_URL"
echo ""

# Backup config file
echo "💾 Creating backup of config file..."
cp "$CONFIG_FILE" "$BACKUP_FILE"
echo "   Backup saved to: $BACKUP_FILE"
echo ""

# Determine endpoint and operation based on token type
if [ "$TOKEN_TYPE" == "request" ]; then
    # Generate session from request token
    API_URL="https://api.kite.trade/session/token"
    OPERATION="Generating session from request token"
    PARAM_NAME="request_token"
else
    # Refresh access token from refresh token
    API_URL="https://api.kite.trade/session/refresh_token"
    OPERATION="Refreshing access token"
    PARAM_NAME="refresh_token"
fi

echo "🔄 $OPERATION..."
echo "   Endpoint: $API_URL"
echo ""

# Calculate checksum (required for both operations)
# SHA256 of api_key + token + api_secret
# Note: api_secret in config stores access token, but checksum needs actual API secret
if [ -z "$KITE_API_SECRET" ]; then
    echo -e "${YELLOW}⚠️  Warning: --api-secret not provided${NC}"
    echo "   For request token operations, API secret is required for checksum calculation"
    echo "   The API secret is different from the access token stored in api_secret field"
    echo "   You can find it in your Kite Connect app settings"
    echo ""
    read -p "Enter Kite API Secret (or press Enter to skip): " KITE_API_SECRET
    if [ -z "$KITE_API_SECRET" ]; then
        echo -e "${RED}❌ Error: API secret is required for token operations${NC}"
        echo "   Please provide it using --api-secret flag or enter it when prompted"
        exit 1
    fi
fi

CHECKSUM=$(echo -n "${API_KEY}${TOKEN}${KITE_API_SECRET}" | sha256sum | cut -d' ' -f1)
echo "   Using checksum for authentication"
echo ""

# Show request details
echo "📤 Request Details:"
echo "   Method: POST"
echo "   URL: $API_URL"
echo "   Headers:"
echo "      Content-Type: application/x-www-form-urlencoded"
echo "      X-Kite-Version: 3"
echo "   Form Data:"
echo "      $PARAM_NAME: ${TOKEN:0:10}... (hidden)"
echo "      api_key: ${API_KEY:0:10}..."
echo "      checksum: ${CHECKSUM:0:10}... (hidden)"
echo ""

# Make API call (with verbose output to show headers)
echo "📡 Making API request..."

# Create temp files for output
TEMP_HEADERS=$(mktemp)
TEMP_BODY=$(mktemp)

# Make request and capture both headers (stderr) and body (stdout)
HTTP_CODE=$(curl -s -w "%{http_code}" -X POST "$API_URL" \
    -H "Content-Type: application/x-www-form-urlencoded" \
    -H "X-Kite-Version: 3" \
    -d "$PARAM_NAME=$TOKEN" \
    -d "api_key=$API_KEY" \
    -d "checksum=$CHECKSUM" \
    -D "$TEMP_HEADERS" \
    -o "$TEMP_BODY" \
    -v 2>&1 | tee /tmp/curl_verbose.txt | tail -1)

# Extract JSON response from body file
JSON_RESPONSE=$(cat "$TEMP_BODY")

# Show request/response headers from verbose output
echo ""
echo "📋 Request/Response Headers:"
grep -E "^> |^< " /tmp/curl_verbose.txt | head -20 || echo "   (Headers not captured)"
rm -f /tmp/curl_verbose.txt

# Cleanup temp files
rm -f "$TEMP_HEADERS" "$TEMP_BODY"
echo ""

# Check if curl was successful and we got a response
if [ -z "$JSON_RESPONSE" ]; then
    echo -e "${RED}❌ Error: Failed to get valid response from Kite API${NC}"
    echo "   Raw response: $RESPONSE"
    exit 1
fi

# Show response summary
echo "📥 Response Summary:"
echo "   HTTP Status: ${HTTP_CODE:-N/A}"
echo "   Response Body: $JSON_RESPONSE"
echo ""

# Parse response
STATUS=$(echo "$JSON_RESPONSE" | grep -o '"status"[[:space:]]*:[[:space:]]*"[^"]*"' | cut -d'"' -f4)
ACCESS_TOKEN=$(echo "$JSON_RESPONSE" | grep -o '"access_token"[[:space:]]*:[[:space:]]*"[^"]*"' | cut -d'"' -f4)
NEW_REFRESH_TOKEN=$(echo "$JSON_RESPONSE" | grep -o '"refresh_token"[[:space:]]*:[[:space:]]*"[^"]*"' | cut -d'"' -f4)
ERROR_MSG=$(echo "$JSON_RESPONSE" | grep -o '"message"[[:space:]]*:[[:space:]]*"[^"]*"' | cut -d'"' -f4)

if [ "$STATUS" != "success" ]; then
    echo -e "${RED}❌ Token refresh failed${NC}"
    echo "   Response: $JSON_RESPONSE"
    if [ -n "$ERROR_MSG" ]; then
        echo "   Error: $ERROR_MSG"
    fi
    exit 1
fi

if [ -z "$ACCESS_TOKEN" ]; then
    echo -e "${RED}❌ Error: Could not extract access token from response${NC}"
    echo "   Response: $RESPONSE"
    exit 1
fi

echo -e "${GREEN}✅ Token refresh successful${NC}"
echo "   Access Token: ${ACCESS_TOKEN:0:10}..."
if [ -n "$NEW_REFRESH_TOKEN" ]; then
    echo "   New Refresh Token: ${NEW_REFRESH_TOKEN:0:10}..."
fi
echo ""

# Update config file
echo "📝 Updating config file..."

# Use Python or jq to update JSON (prefer jq if available, fallback to Python)
if command -v jq &> /dev/null; then
    # Use jq to update JSON
    jq ".api_secret = \"$ACCESS_TOKEN\"" "$CONFIG_FILE" > "${CONFIG_FILE}.tmp"
    # Update refresh token if we got a new one (from request token) or if it was refreshed
    if [ -n "$NEW_REFRESH_TOKEN" ]; then
        jq ".refresh_token = \"$NEW_REFRESH_TOKEN\"" "${CONFIG_FILE}.tmp" > "${CONFIG_FILE}.tmp2"
        mv "${CONFIG_FILE}.tmp2" "${CONFIG_FILE}.tmp"
    elif [ "$TOKEN_TYPE" == "request" ]; then
        # If using request token and no refresh token in response, keep existing or warn
        echo -e "   ${YELLOW}⚠️  No refresh token in response - keeping existing${NC}"
    fi
    mv "${CONFIG_FILE}.tmp" "$CONFIG_FILE"
elif command -v python3 &> /dev/null; then
    # Use Python to update JSON
    python3 << EOF
import json
import sys

with open("$CONFIG_FILE", "r") as f:
    config = json.load(f)

config["api_secret"] = "$ACCESS_TOKEN"
if "$NEW_REFRESH_TOKEN":
    config["refresh_token"] = "$NEW_REFRESH_TOKEN"

with open("$CONFIG_FILE", "w") as f:
    json.dump(config, f, indent=2)
EOF
else
    echo -e "${YELLOW}⚠️  Warning: Neither jq nor python3 found. Using sed (less reliable)${NC}"
    # Fallback to sed (less reliable but should work for simple JSON)
    sed -i.bak "s/\"api_secret\"[[:space:]]*:[[:space:]]*\"[^\"]*\"/\"api_secret\": \"$ACCESS_TOKEN\"/" "$CONFIG_FILE"
    if [ -n "$NEW_REFRESH_TOKEN" ]; then
        sed -i.bak "s/\"refresh_token\"[[:space:]]*:[[:space:]]*\"[^\"]*\"/\"refresh_token\": \"$NEW_REFRESH_TOKEN\"/" "$CONFIG_FILE"
    fi
    rm -f "${CONFIG_FILE}.bak"
fi

# Set proper permissions
chmod 600 "$CONFIG_FILE"
chown root:root "$CONFIG_FILE"

echo -e "${GREEN}✅ Config file updated${NC}"
echo ""

# Restart services
echo "🔄 Restarting services..."

# Restart read service if it exists
if systemctl list-unit-files | grep -q trading-system-read; then
    echo "   Restarting trading-system-read service..."
    systemctl restart trading-system-read
    sleep 2
    if systemctl is-active --quiet trading-system-read; then
        echo -e "   ${GREEN}✅ trading-system-read restarted${NC}"
    else
        echo -e "   ${YELLOW}⚠️  trading-system-read may not be running${NC}"
    fi
fi

# Note: Trigger service runs via cron, no need to restart
echo "   Note: Trigger service runs via cron (no restart needed)"
echo ""

echo "╔══════════════════════════════════════════════════════════════╗"
if [ "$TOKEN_TYPE" == "request" ]; then
    echo "║         SESSION GENERATION COMPLETED SUCCESSFULLY            ║"
else
    echo "║              TOKEN REFRESH COMPLETED SUCCESSFULLY            ║"
fi
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""
echo "📋 Summary:"
if [ "$TOKEN_TYPE" == "request" ]; then
    echo "   ✅ Access token generated from request token"
else
    echo "   ✅ Access token refreshed"
fi
if [ -n "$NEW_REFRESH_TOKEN" ]; then
    echo "   ✅ Refresh token updated"
fi
echo "   ✅ Config file updated"
echo "   ✅ Services restarted"
echo ""
echo "💡 Next steps:"
echo "   - Monitor logs: tail -f $DEPLOY_PATH/logs/*.log"
echo "   - Check service status: systemctl status trading-system-read"
echo ""

