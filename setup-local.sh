#!/bin/bash

# Local setup script for testing

set -e

echo "Setting up local environment for testing..."

# Create necessary directories
echo "Creating directories..."
mkdir -p logs
mkdir -p config

# Check if config files exist
if [ ! -f "config/google-credentials.json" ]; then
    echo "⚠️  WARNING: config/google-credentials.json not found!"
    echo "   Please create this file with your Google Sheets service account credentials."
    echo "   See CONFIG_SETUP.md for instructions."
    cp config/google-credentials.json.example config/google-credentials.json
fi

if [ ! -f "config/broker-config.json" ]; then
    echo "Creating broker-config.json from example..."
    cp config/broker-config.json.example config/broker-config.json
    echo "✅ Created config/broker-config.json (using mock broker for testing)"
fi

# Check if Redis is running
echo "Checking Redis..."
if command -v redis-cli &> /dev/null; then
    if redis-cli ping &> /dev/null; then
        echo "✅ Redis is running"
    else
        echo "⚠️  Redis is installed but not running"
        echo "   Start it with: redis-server"
        echo "   Or on Mac: brew services start redis"
    fi
else
    echo "⚠️  Redis is not installed"
    echo "   Install with:"
    echo "   - Mac: brew install redis && brew services start redis"
    echo "   - Ubuntu/Debian: sudo apt-get install redis-server"
    echo "   - Or download from: https://redis.io/download"
fi

# Check if Go is installed
echo "Checking Go..."
if command -v go &> /dev/null; then
    GO_VERSION=$(go version | awk '{print $3}')
    echo "✅ Go is installed: $GO_VERSION"
    
    # Install dependencies
    echo "Installing Go dependencies..."
    go mod download
    go mod tidy
    
    # Build
    echo "Building trading system..."
    if make build; then
        echo "✅ Build successful!"
        echo ""
        echo "Next steps:"
        echo "1. Update config files (see CONFIG_SETUP.md)"
        echo "2. Start Redis if not running: redis-server"
        echo "3. Set environment variables (see CONFIG_SETUP.md)"
        echo "4. Test read module: ./trading-system -module=read"
        echo "5. Test trigger module: ./trading-system -module=trigger"
    else
        echo "❌ Build failed"
        exit 1
    fi
else
    echo "❌ Go is not installed"
    echo "   Install from: https://go.dev/dl/"
    echo "   Or on Mac: brew install go"
    exit 1
fi

echo ""
echo "✅ Setup complete!"


