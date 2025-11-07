# GCP Debugging Guide

This guide provides tools and commands to debug the trading system on GCP.

## Quick Debug Commands

### 1. Check Service Status
```bash
sudo systemctl status trading-system-read
```

### 2. Check Redis and Orders
```bash
# Check Redis is running
redis-cli ping

# Count orders in cache
redis-cli ZCARD pending_orders

# See orders due now
NOW=$(date +%s)
redis-cli ZRANGEBYSCORE pending_orders 0 $NOW LIMIT 0 10
```

### 3. View Logs
```bash
# Read module logs
sudo tail -f /opt/trading-system/logs/read-module.log

# Trigger module logs
sudo tail -f /opt/trading-system/logs/trigger-module.log

# All logs
sudo tail -f /opt/trading-system/logs/*.log
```

### 4. Check Configuration
```bash
# Broker config
sudo cat /opt/trading-system/config/broker-config.json | jq .

# Environment variables
sudo systemctl show trading-system-read | grep -E "(GOOGLE|BROKER|REDIS)"
```

### 5. Test Trigger Module Manually
```bash
sudo /opt/trading-system/bin/trading-system -module=trigger
```

### 6. Check Cron Job
```bash
sudo crontab -l
```

## Debug Scripts

### Main Debug Script
Run the comprehensive debug script:
```bash
chmod +x /opt/trading-system/scripts/debug-gcp.sh
sudo /opt/trading-system/scripts/debug-gcp.sh
```

### Test Trigger Module
```bash
chmod +x /opt/trading-system/scripts/test-trigger.sh
sudo /opt/trading-system/scripts/test-trigger.sh
```

### Check Orders
```bash
chmod +x /opt/trading-system/scripts/check-orders.sh
/opt/trading-system/scripts/check-orders.sh
```

### Watch Logs
```bash
chmod +x /opt/trading-system/scripts/watch-logs.sh
sudo /opt/trading-system/scripts/watch-logs.sh
```

## Common Issues and Solutions

### Issue 1: Orders Not Being Executed

**Check:**
1. Are orders in Redis cache?
   ```bash
   redis-cli ZCARD pending_orders
   ```

2. Are orders due for execution?
   ```bash
   NOW=$(date +%s)
   redis-cli ZRANGEBYSCORE pending_orders 0 $NOW LIMIT 0 10
   ```

3. Is cron job running?
   ```bash
   sudo crontab -l
   ```

4. Check trigger module logs for errors:
   ```bash
   sudo tail -50 /opt/trading-system/logs/trigger-module.log
   ```

**Solution:**
- If no orders in cache: Check read module logs
- If orders not due: Check scheduled times
- If cron not running: Run `sudo /opt/trading-system/scripts/setup-cron.sh`
- If errors in logs: Check broker configuration

### Issue 2: Broker Connection Issues

**Check:**
1. Broker config is correct:
   ```bash
   sudo cat /opt/trading-system/config/broker-config.json | jq .
   ```

2. API keys are set:
   ```bash
   sudo systemctl show trading-system-read | grep BROKER
   ```

3. Test broker manually:
   ```bash
   # Check if broker type is correct
   sudo cat /opt/trading-system/config/broker-config.json | jq -r '.type'
   ```

**Solution:**
- Update broker config with correct API keys
- Restart service: `sudo systemctl restart trading-system-read`

### Issue 3: Orders Not Being Read from Google Sheets

**Check:**
1. Google credentials exist:
   ```bash
   sudo ls -la /opt/trading-system/config/google-credentials.json
   ```

2. Google Sheet ID is set:
   ```bash
   sudo systemctl show trading-system-read | grep GOOGLE_SHEET_ID
   ```

3. Read module logs:
   ```bash
   sudo tail -50 /opt/trading-system/logs/read-module.log
   ```

**Solution:**
- Verify Google credentials file
- Set GOOGLE_SHEET_ID in systemd service
- Check sheet permissions

### Issue 4: Trigger Module Not Running

**Check:**
1. Cron job exists:
   ```bash
   sudo crontab -l | grep trading-system
   ```

2. Manual test:
   ```bash
   sudo /opt/trading-system/bin/trading-system -module=trigger
   ```

**Solution:**
- Setup cron: `sudo /opt/trading-system/scripts/setup-cron.sh`
- Check cron logs: `sudo grep CRON /var/log/syslog | tail -20`

## Step-by-Step Debugging Process

1. **Run Main Debug Script**
   ```bash
   sudo /opt/trading-system/scripts/debug-gcp.sh
   ```

2. **Check Orders in Cache**
   ```bash
   /opt/trading-system/scripts/check-orders.sh
   ```

3. **Test Trigger Manually**
   ```bash
   sudo /opt/trading-system/scripts/test-trigger.sh
   ```

4. **Watch Logs in Real-Time**
   ```bash
   sudo /opt/trading-system/scripts/watch-logs.sh
   ```

5. **Verify Configuration**
   ```bash
   sudo cat /opt/trading-system/config/broker-config.json | jq .
   sudo systemctl show trading-system-read | grep -E "(GOOGLE|BROKER)"
   ```

## Useful Commands Reference

```bash
# Service management
sudo systemctl status trading-system-read
sudo systemctl restart trading-system-read
sudo systemctl stop trading-system-read
sudo systemctl start trading-system-read

# Logs
sudo tail -f /opt/trading-system/logs/read-module.log
sudo tail -f /opt/trading-system/logs/trigger-module.log
sudo journalctl -u trading-system-read -f

# Redis
redis-cli ping
redis-cli ZCARD pending_orders
redis-cli ZRANGE pending_orders 0 -1
redis-cli KEYS "order:*"

# Configuration
sudo cat /opt/trading-system/config/broker-config.json
sudo systemctl show trading-system-read

# Manual testing
sudo /opt/trading-system/bin/trading-system -module=read
sudo /opt/trading-system/bin/trading-system -module=trigger
```

## Getting Help

If issues persist:
1. Collect logs: `sudo tail -100 /opt/trading-system/logs/*.log > /tmp/debug-logs.txt`
2. Check service status: `sudo systemctl status trading-system-read`
3. Verify configuration files
4. Test each module manually

