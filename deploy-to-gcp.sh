#!/bin/bash

# Quick deployment script for specific GCP instance

set -e

INSTANCE_NAME="instance-20251031-152142"
ZONE="${GCP_ZONE}"
PROJECT_ID="${GCP_PROJECT_ID:-$(gcloud config get-value project 2>/dev/null || echo '')}"
DEPLOY_PATH="/opt/trading-system"

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║        Trading System - GCP Deployment                      ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""
echo "Instance: $INSTANCE_NAME"
echo "Zone: ${ZONE:-'Not set - will try to detect'}"
echo "Project: ${PROJECT_ID:-'Not set'}"
echo ""

# Auto-detect zone if not set
if [ -z "$ZONE" ]; then
    echo "Detecting instance zone..."
    ZONE=$(gcloud compute instances list --filter="name:$INSTANCE_NAME" --format="get(zone)" 2>/dev/null | head -1 | xargs basename 2>/dev/null || echo "")
    if [ -z "$ZONE" ]; then
        echo "❌ Error: Could not detect zone. Please set GCP_ZONE environment variable"
        echo "   Example: export GCP_ZONE='us-central1-a'"
        exit 1
    fi
    echo "✅ Detected zone: $ZONE"
fi

# Check project ID
if [ -z "$PROJECT_ID" ]; then
    echo "❌ Error: GCP_PROJECT_ID not set. Please set it:"
    echo "   export GCP_PROJECT_ID='your-project-id'"
    exit 1
fi

echo "✅ Using zone: $ZONE"
echo "✅ Using project: $PROJECT_ID"
echo ""

# Build Linux binary
echo "Step 1: Building Linux binary..."
if [ -f "trading-system-linux" ]; then
    echo "   Using existing Linux binary"
else
    GOOS=linux GOARCH=amd64 go build -o trading-system-linux cmd/trading-system/main.go
    echo "   ✅ Linux binary built"
fi

# Create deployment package
echo ""
echo "Step 2: Creating deployment package..."
TEMP_DIR=$(mktemp -d)
DEPLOY_DIR="$TEMP_DIR/trading-system"

mkdir -p "$DEPLOY_DIR/bin"
mkdir -p "$DEPLOY_DIR/config"
mkdir -p "$DEPLOY_DIR/logs"
mkdir -p "$DEPLOY_DIR/scripts"

# Copy binary
cp trading-system-linux "$DEPLOY_DIR/bin/trading-system"
chmod +x "$DEPLOY_DIR/bin/trading-system"

# Copy config examples
cp config/broker-config.json.example "$DEPLOY_DIR/config/broker-config.json.example"
cp config/google-credentials.json.example "$DEPLOY_DIR/config/google-credentials.json.example"

# Copy scripts
cp scripts/setup-cron.sh "$DEPLOY_DIR/scripts/"
cp scripts/install-redis.sh "$DEPLOY_DIR/scripts/"
cp scripts/trading-system-read.service "$DEPLOY_DIR/scripts/"
cp scripts/trading-system-trigger.service "$DEPLOY_DIR/scripts/"
chmod +x "$DEPLOY_DIR/scripts/"*.sh

# Create tarball
cd "$TEMP_DIR"
tar -czf trading-system.tar.gz trading-system/
cd - > /dev/null

echo "   ✅ Package created: $TEMP_DIR/trading-system.tar.gz"
echo ""

# Copy to GCP instance (to home directory first)
echo "Step 3: Copying to GCP instance..."
gcloud compute scp "$TEMP_DIR/trading-system.tar.gz" "$INSTANCE_NAME:~/trading-system.tar.gz" \
    --zone="$ZONE" --project="$PROJECT_ID" \
    --quiet

echo "   ✅ File copied to instance"
echo ""

# Run deployment commands on instance
echo "Step 4: Extracting and setting up on instance..."
gcloud compute ssh "$INSTANCE_NAME" --zone="$ZONE" --project="$PROJECT_ID" --command="
    set -e
    echo 'Extracting files...'
    sudo mkdir -p $DEPLOY_PATH
    sudo tar -xzf ~/trading-system.tar.gz -C $DEPLOY_PATH --strip-components=1
    sudo chown -R root:root $DEPLOY_PATH
    sudo chmod +x $DEPLOY_PATH/bin/trading-system
    sudo chmod +x $DEPLOY_PATH/scripts/*.sh
    sudo mkdir -p $DEPLOY_PATH/logs
    sudo chmod 755 $DEPLOY_PATH/logs
    rm -f ~/trading-system.tar.gz
    echo '✅ Files extracted and permissions set'
    echo ''
    echo 'Deployment structure:'
    ls -la $DEPLOY_PATH/
" --quiet

# Cleanup
rm -rf "$TEMP_DIR"

echo ""
echo "╔══════════════════════════════════════════════════════════════╗"
echo "║              DEPLOYMENT COMPLETED SUCCESSFULLY               ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""
echo "📋 Next Steps (SSH into instance to complete setup):"
echo ""
echo "1. SSH into instance:"
echo "   gcloud compute ssh $INSTANCE_NAME --zone=$ZONE"
echo ""
echo "2. Install Redis:"
echo "   sudo /opt/trading-system/scripts/install-redis.sh"
echo ""
echo "3. Update configuration files:"
echo "   sudo nano /opt/trading-system/config/broker-config.json"
echo "   sudo nano /opt/trading-system/config/google-credentials.json"
echo ""
echo "4. Setup cron job:"
echo "   sudo /opt/trading-system/scripts/setup-cron.sh"
echo ""
echo "5. Install and start systemd service:"
echo "   sudo cp /opt/trading-system/scripts/trading-system-read.service /etc/systemd/system/"
echo "   sudo systemctl daemon-reload"
echo "   sudo systemctl enable trading-system-read"
echo "   sudo systemctl start trading-system-read"
echo ""
echo "6. Check status:"
echo "   sudo systemctl status trading-system-read"
echo "   tail -f /opt/trading-system/logs/read-module.log"
echo ""

