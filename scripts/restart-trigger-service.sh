#!/bin/bash

# Script to restart the trigger service on GCP instance after deployment

INSTANCE_NAME="instance-20251031-152142"
ZONE="${GCP_ZONE:-us-central1-c}"
PROJECT_ID="${GCP_PROJECT_ID:-spark-cluster-179418}"

echo "🔄 Restarting trigger service on GCP instance..."

# Copy service file if needed and restart
gcloud compute ssh "$INSTANCE_NAME" --zone="$ZONE" --project="$PROJECT_ID" --command="
    set -e
    echo '📋 Checking trigger service status...'
    
    # Copy service file if it exists in deployment path
    if [ -f /opt/trading-system/scripts/trading-system-trigger.service ]; then
        echo '📝 Copying service file...'
        sudo cp /opt/trading-system/scripts/trading-system-trigger.service /etc/systemd/system/
        sudo systemctl daemon-reload
        echo '✅ Service file updated'
    fi
    
    # Check if service exists
    if systemctl list-unit-files | grep -q trading-system-trigger; then
        echo '🔄 Restarting trigger service...'
        sudo systemctl restart trading-system-trigger
        sleep 2
        sudo systemctl status trading-system-trigger --no-pager -l
        echo ''
        echo '✅ Trigger service restarted'
    else
        echo '⚠️  Service not found. Installing...'
        sudo cp /opt/trading-system/scripts/trading-system-trigger.service /etc/systemd/system/
        sudo systemctl daemon-reload
        sudo systemctl enable trading-system-trigger
        sudo systemctl start trading-system-trigger
        sleep 2
        sudo systemctl status trading-system-trigger --no-pager -l
        echo ''
        echo '✅ Trigger service installed and started'
    fi
    
    echo ''
    echo '📊 Recent logs:'
    tail -20 /opt/trading-system/logs/trigger-module.log 2>/dev/null || echo 'No logs yet'
"

