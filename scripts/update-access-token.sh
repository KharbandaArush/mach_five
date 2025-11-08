#!/bin/bash

# Script to update access token in broker-config.json
# Usage: ./update-access-token.sh [access_token]

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
echo "║        Update Kite Access Token Script                      ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

# Check if config file exists
if [ ! -f "$CONFIG_FILE" ]; then
    echo -e "${RED}❌ Error: Config file not found: $CONFIG_FILE${NC}"
    exit 1
fi

# Get access token from argument or prompt
if [ -n "$1" ]; then
    ACCESS_TOKEN="$1"
else
    echo "Enter the new access token:"
    read -s ACCESS_TOKEN
    echo ""
fi

if [ -z "$ACCESS_TOKEN" ]; then
    echo -e "${RED}❌ Error: Access token is required${NC}"
    exit 1
fi

# Backup config file
echo "💾 Creating backup of config file..."
cp "$CONFIG_FILE" "$BACKUP_FILE"
echo "   Backup saved to: $BACKUP_FILE"
echo ""

# Update config file
echo "📝 Updating config file..."

# Use Python or jq to update JSON (prefer jq if available, fallback to Python)
if command -v jq &> /dev/null; then
    # Use jq to update JSON
    jq ".api_secret = \"$ACCESS_TOKEN\"" "$CONFIG_FILE" > "${CONFIG_FILE}.tmp"
    mv "${CONFIG_FILE}.tmp" "$CONFIG_FILE"
elif command -v python3 &> /dev/null; then
    # Use Python to update JSON
    python3 << EOF
import json
import sys

with open("$CONFIG_FILE", "r") as f:
    config = json.load(f)

config["api_secret"] = "$ACCESS_TOKEN"

with open("$CONFIG_FILE", "w") as f:
    json.dump(config, f, indent=2)
EOF
else
    echo -e "${YELLOW}⚠️  Warning: Neither jq nor python3 found. Using sed (less reliable)${NC}"
    # Fallback to sed (less reliable but should work for simple JSON)
    sed -i.bak "s/\"api_secret\"[[:space:]]*:[[:space:]]*\"[^\"]*\"/\"api_secret\": \"$ACCESS_TOKEN\"/" "$CONFIG_FILE"
    rm -f "${CONFIG_FILE}.bak"
fi

# Set proper permissions
chmod 600 "$CONFIG_FILE"
chown root:root "$CONFIG_FILE"

echo -e "${GREEN}✅ Config file updated${NC}"
echo "   Access Token: ${ACCESS_TOKEN:0:10}..."
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

# Restart trigger service if it exists
if systemctl list-unit-files | grep -q trading-system-trigger; then
    echo "   Restarting trading-system-trigger service..."
    systemctl restart trading-system-trigger
    sleep 2
    if systemctl is-active --quiet trading-system-trigger; then
        echo -e "   ${GREEN}✅ trading-system-trigger restarted${NC}"
    else
        echo -e "   ${YELLOW}⚠️  trading-system-trigger may not be running${NC}"
    fi
fi

echo ""
echo "╔══════════════════════════════════════════════════════════════╗"
echo "║         ACCESS TOKEN UPDATE COMPLETED SUCCESSFULLY           ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""
echo "📋 Summary:"
echo "   ✅ Access token updated in config"
echo "   ✅ Services restarted"
echo ""
echo "💡 Next steps:"
echo "   - Monitor logs: tail -f $DEPLOY_PATH/logs/*.log"
echo "   - Check service status: systemctl status trading-system-*"
echo ""

