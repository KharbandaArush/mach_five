#!/bin/bash

# Setup cron job for trigger module

set -e

DEPLOY_PATH="/opt/trading-system"
CRON_USER="root"

echo "Setting up cron job for trigger module..."

# Create cron entry
CRON_ENTRY="* * * * * $DEPLOY_PATH/bin/trading-system -module=trigger >> $DEPLOY_PATH/logs/trigger-module.log 2>&1"

# Check if cron entry already exists
if crontab -u "$CRON_USER" -l 2>/dev/null | grep -q "trading-system.*trigger"; then
    echo "Cron entry already exists, updating..."
    crontab -u "$CRON_USER" -l 2>/dev/null | grep -v "trading-system.*trigger" | crontab -u "$CRON_USER" -
fi

# Add cron entry
(crontab -u "$CRON_USER" -l 2>/dev/null; echo "$CRON_ENTRY") | crontab -u "$CRON_USER" -

echo "Cron job installed successfully"
echo "Cron entry: $CRON_ENTRY"
echo ""
echo "To view cron jobs: crontab -u $CRON_USER -l"
echo "To remove cron job: crontab -u $CRON_USER -e"


