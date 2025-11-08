#!/bin/bash

# Stream logs from GCP instance in real-time
# Usage: ./stream-logs.sh [module]
#   module: read, trigger, or all (default: all)

MODULE="${1:-all}"
INSTANCE="instance-20251031-152142"
ZONE="us-central1-c"
PROJECT="spark-cluster-179418"
LOG_DIR="/opt/trading-system/logs"

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║        📺 Streaming Trading System Logs                       ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""
echo "Instance: $INSTANCE"
echo "Module: $MODULE"
echo ""
echo "Press Ctrl+C to stop streaming"
echo ""

case "$MODULE" in
    read)
        echo "📖 Streaming Read Module logs..."
        gcloud compute ssh "$INSTANCE" --zone="$ZONE" --project="$PROJECT" \
            --command="sudo tail -f $LOG_DIR/read-module.log"
        ;;
    trigger)
        echo "🚀 Streaming Trigger Module logs..."
        gcloud compute ssh "$INSTANCE" --zone="$ZONE" --project="$PROJECT" \
            --command="sudo tail -f $LOG_DIR/trigger-module.log"
        ;;
    all|*)
        echo "📺 Streaming all logs (multiplexed)..."
        echo ""
        echo "Note: This will show logs from both modules interleaved"
        echo ""
        gcloud compute ssh "$INSTANCE" --zone="$ZONE" --project="$PROJECT" \
            --command="sudo tail -f $LOG_DIR/*.log 2>/dev/null || sudo tail -f $LOG_DIR/read-module.log $LOG_DIR/trigger-module.log"
        ;;
esac


