#!/bin/bash

# Install and configure Redis on GCP instance

set -e

echo "Installing Redis..."

# Detect OS
if [ -f /etc/os-release ]; then
    . /etc/os-release
    OS=$ID
else
    echo "Error: Cannot detect OS"
    exit 1
fi

# Install Redis based on OS
case $OS in
    ubuntu|debian)
        sudo apt-get update
        sudo apt-get install -y redis-server
        ;;
    centos|rhel|fedora)
        sudo yum install -y redis
        ;;
    *)
        echo "Error: Unsupported OS: $OS"
        exit 1
        ;;
esac

# Configure Redis
echo "Configuring Redis..."

# Backup original config
if [ -f /etc/redis/redis.conf ]; then
    sudo cp /etc/redis/redis.conf /etc/redis/redis.conf.backup
fi

# Enable Redis to start on boot
sudo systemctl enable redis-server 2>/dev/null || sudo systemctl enable redis 2>/dev/null || true

# Start Redis
sudo systemctl start redis-server 2>/dev/null || sudo systemctl start redis 2>/dev/null || true

# Check Redis status
if sudo systemctl is-active --quiet redis-server || sudo systemctl is-active --quiet redis; then
    echo "Redis is running"
else
    echo "Warning: Redis may not be running. Check with: sudo systemctl status redis"
fi

# Test Redis connection
if redis-cli ping > /dev/null 2>&1; then
    echo "Redis connection test successful"
else
    echo "Warning: Redis connection test failed"
fi

echo "Redis installation completed!"


