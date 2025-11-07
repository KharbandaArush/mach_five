#!/bin/bash

# Post-deployment setup script to run on GCP instance

set -e

DEPLOY_PATH="/opt/trading-system"

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║        Trading System - Post-Deployment Setup               ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then 
    echo "⚠️  This script should be run with sudo"
    echo "   Run: sudo $0"
    exit 1
fi

# Step 1: Install Redis
echo "Step 1: Installing Redis..."
if command -v redis-server > /dev/null 2>&1; then
    echo "   ✅ Redis is already installed"
else
    if [ -f "$DEPLOY_PATH/scripts/install-redis.sh" ]; then
        bash "$DEPLOY_PATH/scripts/install-redis.sh"
    else
        echo "   ⚠️  install-redis.sh not found, installing manually..."
        if command -v apt-get > /dev/null 2>&1; then
            apt-get update
            apt-get install -y redis-server
        elif command -v yum > /dev/null 2>&1; then
            yum install -y redis
        else
            echo "   ❌ Could not detect package manager"
            exit 1
        fi
    fi
    systemctl enable redis-server 2>/dev/null || systemctl enable redis 2>/dev/null || true
    systemctl start redis-server 2>/dev/null || systemctl start redis 2>/dev/null || true
    echo "   ✅ Redis installed and started"
fi

# Step 2: Check configuration files
echo ""
echo "Step 2: Checking configuration files..."
if [ ! -f "$DEPLOY_PATH/config/broker-config.json" ]; then
    echo "   ⚠️  broker-config.json not found"
    echo "   📝 Create it from example:"
    echo "      cp $DEPLOY_PATH/config/broker-config.json.example $DEPLOY_PATH/config/broker-config.json"
    echo "      nano $DEPLOY_PATH/config/broker-config.json"
else
    echo "   ✅ broker-config.json exists"
fi

if [ ! -f "$DEPLOY_PATH/config/google-credentials.json" ]; then
    echo "   ⚠️  google-credentials.json not found"
    echo "   📝 You need to upload your Google credentials file"
    echo "      Use: gcloud compute scp <local-path> instance-20251031-152142:$DEPLOY_PATH/config/google-credentials.json"
else
    echo "   ✅ google-credentials.json exists"
fi

# Step 3: Setup cron
echo ""
echo "Step 3: Setting up cron job..."
if [ -f "$DEPLOY_PATH/scripts/setup-cron.sh" ]; then
    bash "$DEPLOY_PATH/scripts/setup-cron.sh"
    echo "   ✅ Cron job configured"
else
    echo "   ⚠️  setup-cron.sh not found"
fi

# Step 4: Setup systemd service
echo ""
echo "Step 4: Setting up systemd service..."
if [ -f "$DEPLOY_PATH/scripts/trading-system-read.service" ]; then
    cp "$DEPLOY_PATH/scripts/trading-system-read.service" /etc/systemd/system/
    systemctl daemon-reload
    echo "   ✅ Systemd service file installed"
    
    # Check if service should be started
    read -p "   Start trading-system-read service now? (y/n): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        systemctl enable trading-system-read
        systemctl start trading-system-read
        echo "   ✅ Service started and enabled"
        sleep 2
        systemctl status trading-system-read --no-pager
    else
        echo "   ℹ️  Service not started. Start manually with:"
        echo "      sudo systemctl enable trading-system-read"
        echo "      sudo systemctl start trading-system-read"
    fi
else
    echo "   ⚠️  trading-system-read.service not found"
fi

# Step 5: Verify setup
echo ""
echo "Step 5: Verifying setup..."
echo "   Checking Redis..."
if redis-cli ping > /dev/null 2>&1; then
    echo "   ✅ Redis is running"
else
    echo "   ❌ Redis is not running"
fi

echo "   Checking binary..."
if [ -f "$DEPLOY_PATH/bin/trading-system" ] && [ -x "$DEPLOY_PATH/bin/trading-system" ]; then
    echo "   ✅ Binary exists and is executable"
else
    echo "   ❌ Binary not found or not executable"
fi

echo ""
echo "╔══════════════════════════════════════════════════════════════╗"
echo "║                    SETUP COMPLETE                            ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""
echo "📋 Useful commands:"
echo ""
echo "   Check read module logs:"
echo "   tail -f $DEPLOY_PATH/logs/read-module.log"
echo ""
echo "   Check trigger module logs:"
echo "   tail -f $DEPLOY_PATH/logs/trigger-module.log"
echo ""
echo "   Check service status:"
echo "   sudo systemctl status trading-system-read"
echo ""
echo "   Restart service:"
echo "   sudo systemctl restart trading-system-read"
echo ""
echo "   Check cron jobs:"
echo "   crontab -l"
echo ""

