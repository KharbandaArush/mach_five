# GCP Deployment Guide

## Deployment Status

✅ **Deployed to:** `instance-20251031-152142`  
✅ **Zone:** `us-central1-c`  
✅ **Project:** `spark-cluster-179418`  
✅ **Deployment Path:** `/opt/trading-system`

## Post-Deployment Setup

### Step 1: SSH into the Instance

```bash
gcloud compute ssh instance-20251031-152142 --zone=us-central1-c
```

### Step 2: Run Post-Deployment Setup

```bash
# Make script executable and run
chmod +x ~/post-deploy-setup.sh
sudo ~/post-deploy-setup.sh
```

Or run steps manually:

### Step 2a: Install Redis

```bash
sudo /opt/trading-system/scripts/install-redis.sh
```

### Step 2b: Upload Configuration Files

**Upload broker config:**
```bash
# From your local machine
gcloud compute scp config/broker-config.json \
    instance-20251031-152142:/opt/trading-system/config/broker-config.json \
    --zone=us-central1-c
```

**Upload Google credentials:**
```bash
# From your local machine
gcloud compute scp config/google-credentials.json \
    instance-20251031-152142:/opt/trading-system/config/google-credentials.json \
    --zone=us-central1-c
```

**Set permissions:**
```bash
# On the instance
sudo chmod 600 /opt/trading-system/config/*.json
sudo chown root:root /opt/trading-system/config/*.json
```

### Step 2c: Update Environment Variables in Systemd Service

Edit the systemd service file to set your Google Sheet ID:

```bash
sudo nano /etc/systemd/system/trading-system-read.service
```

Update the `GOOGLE_SHEET_ID` environment variable:
```
Environment="GOOGLE_SHEET_ID=your-sheet-id-here"
```

### Step 2d: Setup Cron Job

```bash
sudo /opt/trading-system/scripts/setup-cron.sh
```

### Step 2e: Install and Start Systemd Service

```bash
# Copy service file (if not already done)
sudo cp /opt/trading-system/scripts/trading-system-read.service /etc/systemd/system/

# Reload systemd
sudo systemctl daemon-reload

# Enable and start service
sudo systemctl enable trading-system-read
sudo systemctl start trading-system-read

# Check status
sudo systemctl status trading-system-read
```

## Verification

### Check Service Status

```bash
sudo systemctl status trading-system-read
```

### Check Logs

```bash
# Read module logs
tail -f /opt/trading-system/logs/read-module.log

# Trigger module logs (from cron)
tail -f /opt/trading-system/logs/trigger-module.log
```

### Check Redis

```bash
redis-cli ping
redis-cli ZCARD pending_orders
```

### Check Cron Job

```bash
sudo crontab -l
```

## Monitoring

### View Real-time Logs

```bash
# Read module
tail -f /opt/trading-system/logs/read-module.log

# Trigger module
tail -f /opt/trading-system/logs/trigger-module.log

# All logs
tail -f /opt/trading-system/logs/*.log
```

### Check Cached Orders

```bash
redis-cli ZRANGE pending_orders 0 -1
redis-cli KEYS "order:*" | head -5
```

### Check Service Health

```bash
# Service status
sudo systemctl status trading-system-read

# Service logs
sudo journalctl -u trading-system-read -f
```

## Troubleshooting

### Service Not Starting

```bash
# Check service status
sudo systemctl status trading-system-read

# Check logs
sudo journalctl -u trading-system-read -n 50

# Check if binary exists
ls -la /opt/trading-system/bin/trading-system

# Test binary manually
sudo /opt/trading-system/bin/trading-system -module=read
```

### Redis Connection Issues

```bash
# Check if Redis is running
sudo systemctl status redis-server

# Test connection
redis-cli ping

# Check Redis logs
sudo journalctl -u redis-server -n 50
```

### Configuration Issues

```bash
# Verify config files exist
ls -la /opt/trading-system/config/

# Check file permissions
sudo chmod 600 /opt/trading-system/config/*.json

# Test with manual run
sudo /opt/trading-system/bin/trading-system -module=read
```

### Cron Job Not Running

```bash
# Check cron jobs
sudo crontab -l

# Check cron logs
sudo grep CRON /var/log/syslog | tail -20

# Test trigger module manually
sudo /opt/trading-system/bin/trading-system -module=trigger
```

## Maintenance

### Restart Service

```bash
sudo systemctl restart trading-system-read
```

### Update Configuration

1. Upload new config file:
```bash
gcloud compute scp config/broker-config.json \
    instance-20251031-152142:/opt/trading-system/config/broker-config.json \
    --zone=us-central1-c
```

2. Restart service:
```bash
sudo systemctl restart trading-system-read
```

### Update Binary

1. Rebuild and deploy:
```bash
./deploy-to-gcp.sh
```

2. Restart service:
```bash
sudo systemctl restart trading-system-read
```

## File Structure on Instance

```
/opt/trading-system/
├── bin/
│   └── trading-system          # Main binary
├── config/
│   ├── broker-config.json      # Broker configuration (update this)
│   ├── google-credentials.json # Google credentials (upload this)
│   └── *.example               # Example files
├── logs/
│   ├── read-module.log         # Read module logs
│   ├── trigger-module.log      # Trigger module logs
│   └── broker-module.log       # Broker module logs
└── scripts/
    ├── install-redis.sh        # Redis installation
    ├── setup-cron.sh           # Cron setup
    └── *.service               # Systemd service files
```

## Quick Reference

```bash
# SSH to instance
gcloud compute ssh instance-20251031-152142 --zone=us-central1-c

# View logs
tail -f /opt/trading-system/logs/read-module.log

# Restart service
sudo systemctl restart trading-system-read

# Check status
sudo systemctl status trading-system-read

# Check Redis
redis-cli ping
redis-cli ZCARD pending_orders
```

