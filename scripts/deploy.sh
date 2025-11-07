#!/bin/bash

# Deployment script for GCP Compute Engine instance

set -e

echo "Starting deployment to GCP instance..."

# Configuration
INSTANCE_NAME="${GCP_INSTANCE_NAME:-trading-system-instance}"
ZONE="${GCP_ZONE:-us-central1-a}"
PROJECT_ID="${GCP_PROJECT_ID}"
DEPLOY_USER="${DEPLOY_USER:-$USER}"
DEPLOY_PATH="/opt/trading-system"

# Check if project ID is set
if [ -z "$PROJECT_ID" ]; then
    echo "Error: GCP_PROJECT_ID environment variable is not set"
    exit 1
fi

echo "Deploying to instance: $INSTANCE_NAME in zone: $ZONE"

# Build the binary
echo "Building Go binary..."
GOOS=linux GOARCH=amd64 go build -o trading-system cmd/trading-system/main.go

if [ ! -f "trading-system" ]; then
    echo "Error: Build failed"
    exit 1
fi

# Create deployment package
echo "Creating deployment package..."
TEMP_DIR=$(mktemp -d)
DEPLOY_DIR="$TEMP_DIR/trading-system"

mkdir -p "$DEPLOY_DIR/bin"
mkdir -p "$DEPLOY_DIR/config"
mkdir -p "$DEPLOY_DIR/logs"
mkdir -p "$DEPLOY_DIR/scripts"

# Copy binary
cp trading-system "$DEPLOY_DIR/bin/"

# Copy config examples (user should update these)
cp config/broker-config.json.example "$DEPLOY_DIR/config/broker-config.json.example"
cp config/google-credentials.json.example "$DEPLOY_DIR/config/google-credentials.json.example"

# Copy scripts
cp scripts/setup-cron.sh "$DEPLOY_DIR/scripts/"
cp scripts/install-redis.sh "$DEPLOY_DIR/scripts/"
chmod +x "$DEPLOY_DIR/scripts/"*.sh

# Copy systemd service files
cp scripts/trading-system-read.service "$DEPLOY_DIR/scripts/"
cp scripts/trading-system-trigger.service "$DEPLOY_DIR/scripts/"

# Create tarball
cd "$TEMP_DIR"
tar -czf trading-system.tar.gz trading-system/
cd - > /dev/null

echo "Deployment package created: $TEMP_DIR/trading-system.tar.gz"

# Copy to GCP instance
echo "Copying to GCP instance..."
gcloud compute scp "$TEMP_DIR/trading-system.tar.gz" "$INSTANCE_NAME:$DEPLOY_PATH.tar.gz" \
    --zone="$ZONE" --project="$PROJECT_ID"

# Run deployment commands on instance
echo "Running deployment commands on instance..."
gcloud compute ssh "$INSTANCE_NAME" --zone="$ZONE" --project="$PROJECT_ID" --command="
    set -e
    sudo mkdir -p $DEPLOY_PATH
    sudo tar -xzf $DEPLOY_PATH.tar.gz -C $DEPLOY_PATH --strip-components=1
    sudo chown -R root:root $DEPLOY_PATH
    sudo chmod +x $DEPLOY_PATH/bin/trading-system
    sudo chmod +x $DEPLOY_PATH/scripts/*.sh
    sudo mkdir -p $DEPLOY_PATH/logs
    sudo chmod 755 $DEPLOY_PATH/logs
    echo 'Deployment completed successfully'
"

# Cleanup
rm -rf "$TEMP_DIR"
rm -f trading-system

echo "Deployment completed!"
echo ""
echo "Next steps:"
echo "1. SSH into the instance: gcloud compute ssh $INSTANCE_NAME --zone=$ZONE"
echo "2. Update config files in $DEPLOY_PATH/config/"
echo "3. Run: sudo $DEPLOY_PATH/scripts/install-redis.sh"
echo "4. Run: sudo $DEPLOY_PATH/scripts/setup-cron.sh"
echo "5. Start the read service: sudo systemctl start trading-system-read"
echo "6. Enable the read service: sudo systemctl enable trading-system-read"


