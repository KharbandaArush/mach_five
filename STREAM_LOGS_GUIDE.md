# Streaming Logs Guide

## Quick Commands

### Stream All Logs (from your local machine)
```bash
# Stream all logs
gcloud compute ssh instance-20251031-152142 --zone=us-central1-c \
    --command="sudo tail -f /opt/trading-system/logs/*.log"
```

### Stream Read Module Logs
```bash
gcloud compute ssh instance-20251031-152142 --zone=us-central1-c \
    --command="sudo tail -f /opt/trading-system/logs/read-module.log"
```

### Stream Trigger Module Logs
```bash
gcloud compute ssh instance-20251031-152142 --zone=us-central1-c \
    --command="sudo tail -f /opt/trading-system/logs/trigger-module.log"
```

## Using the Stream Script

### From Local Machine
```bash
# Stream all logs
./scripts/stream-logs.sh

# Stream read module only
./scripts/stream-logs.sh read

# Stream trigger module only
./scripts/stream-logs.sh trigger
```

### From GCP Instance (SSH first)
```bash
# SSH to instance
gcloud compute ssh instance-20251031-152142 --zone=us-central1-c

# Then stream logs
sudo tail -f /opt/trading-system/logs/read-module.log
sudo tail -f /opt/trading-system/logs/trigger-module.log
sudo tail -f /opt/trading-system/logs/*.log
```

## Using Systemd Journal

### Stream via journalctl
```bash
# SSH to instance first
gcloud compute ssh instance-20251031-152142 --zone=us-central1-c

# Stream read module service logs
sudo journalctl -u trading-system-read -f

# Stream with timestamps
sudo journalctl -u trading-system-read -f --since "1 hour ago"
```

## Filter Logs While Streaming

### Filter by keyword
```bash
# Stream and filter for "Kite" or "order"
gcloud compute ssh instance-20251031-152142 --zone=us-central1-c \
    --command="sudo tail -f /opt/trading-system/logs/trigger-module.log | grep -i 'kite\|order'"

# Filter for errors only
gcloud compute ssh instance-20251031-152142 --zone=us-central1-c \
    --command="sudo tail -f /opt/trading-system/logs/*.log | grep -i 'error'"
```

### Stream with color highlighting
```bash
# SSH to instance
gcloud compute ssh instance-20251031-152142 --zone=us-central1-c

# Stream with grep highlighting
sudo tail -f /opt/trading-system/logs/trigger-module.log | grep --color=always -E "ERROR|SUCCESS|Kite|order"
```

## Multiple Terminal Windows

### Option 1: Separate terminals
```bash
# Terminal 1: Read module
gcloud compute ssh instance-20251031-152142 --zone=us-central1-c \
    --command="sudo tail -f /opt/trading-system/logs/read-module.log"

# Terminal 2: Trigger module
gcloud compute ssh instance-20251031-152142 --zone=us-central1-c \
    --command="sudo tail -f /opt/trading-system/logs/trigger-module.log"
```

### Option 2: Use tmux/screen
```bash
# SSH to instance
gcloud compute ssh instance-20251031-152142 --zone=us-central1-c

# Start tmux
tmux new -s logs

# Split window (Ctrl+B then %)
# In left pane: tail -f read-module.log
# In right pane: tail -f trigger-module.log
```

## Useful Commands

### View last N lines
```bash
# Last 50 lines
sudo tail -50 /opt/trading-system/logs/trigger-module.log

# Last 100 lines and follow
sudo tail -100 -f /opt/trading-system/logs/trigger-module.log
```

### Search logs
```bash
# Search for "Kite" in all logs
sudo grep -r "Kite" /opt/trading-system/logs/

# Search with context (5 lines before/after)
sudo grep -B 5 -A 5 "Kite" /opt/trading-system/logs/trigger-module.log
```

### Log file sizes
```bash
# Check log file sizes
ls -lh /opt/trading-system/logs/
```

## Tips

1. **Use Ctrl+C** to stop streaming
2. **Filter while streaming** using grep
3. **Multiple terminals** for different modules
4. **Use tmux/screen** for persistent sessions
5. **Check log rotation** if files get too large


