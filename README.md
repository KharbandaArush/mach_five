# Trading System

A Go-based trading automation system that reads orders from Google Sheets and executes them on stock market brokers according to scheduled times.

## Architecture

The system consists of four main components:

1. **Read Module**: Continuously reads orders from Google Sheets and stores them in Redis cache
2. **Trigger Module**: Executes orders that are due (triggered by cron every minute)
3. **Broker Module**: Handles order execution with rate limiting and adaptive retry
4. **Scheduler**: Linux cron job that triggers the Trigger Module

## Prerequisites

- Go 1.21 or later
- Redis server
- Google Sheets API credentials
- GCP Compute Engine instance (for deployment)

## Setup

### 1. Install Dependencies

```bash
make install-deps
```

### 2. Configure Google Sheets

1. Create a service account in Google Cloud Console
2. Enable Google Sheets API
3. Share your spreadsheet with the service account email
4. Download credentials JSON and save as `config/google-credentials.json`

### 3. Configure Broker

Copy and edit the broker configuration:

```bash
cp config/broker-config.json.example config/broker-config.json
# Edit config/broker-config.json with your broker settings
```

### 4. Set Environment Variables

```bash
export GOOGLE_SHEETS_CREDENTIALS_PATH=./config/google-credentials.json
export GOOGLE_SHEET_ID=your-sheet-id
export GOOGLE_SHEET_RANGE=Sheet1!A2:G100
export REDIS_ADDR=localhost:6379
export BROKER_CONFIG_PATH=./config/broker-config.json
export LOG_LEVEL=INFO
```

## Building

```bash
# Build for local development
make build

# Build for Linux (GCP deployment)
make build-linux
```

## Running Locally

### Start Redis

```bash
redis-server
```

### Run Read Module

```bash
./trading-system -module=read
```

### Run Trigger Module (manually)

```bash
./trading-system -module=trigger
```

## Google Sheets Format

The system expects the following columns in your Google Sheet:

| Column | Description | Example |
|--------|-------------|---------|
| Symbol | Stock symbol | AAPL |
| Price | Order price | 150.50 |
| Date | Scheduled date (YYYY-MM-DD) | 2024-01-15 |
| Time | Scheduled time (HH:MM:SS) | 09:30:00 |
| Order Type | Market or Limit (optional) | Market |
| Side | Buy or Sell (optional) | Buy |
| Quantity | Number of shares (optional) | 10 |

## Deployment to GCP

### 1. Set Environment Variables

```bash
export GCP_PROJECT_ID=your-project-id
export GCP_INSTANCE_NAME=trading-system-instance
export GCP_ZONE=us-central1-a
```

### 2. Deploy

```bash
make deploy
```

### 3. Post-Deployment Setup

SSH into your GCP instance and run:

```bash
# Install Redis
sudo /opt/trading-system/scripts/install-redis.sh

# Setup cron job
sudo /opt/trading-system/scripts/setup-cron.sh

# Update configuration files
sudo nano /opt/trading-system/config/google-credentials.json
sudo nano /opt/trading-system/config/broker-config.json

# Start read module service
sudo systemctl start trading-system-read
sudo systemctl enable trading-system-read
```

## Systemd Services

The deployment includes systemd service files:

- `trading-system-read.service`: Runs the read module continuously
- `trading-system-trigger.service`: Used by cron to trigger order execution

## Monitoring

Logs are written to:
- `/opt/trading-system/logs/read-module.log`
- `/opt/trading-system/logs/trigger-module.log`
- `/opt/trading-system/logs/broker-module.log`

Profiling metrics are logged in JSON format in the trigger module log.

## Development

```bash
# Format code
make fmt

# Run tests
make test

# Run linter
make lint
```

## Configuration

See `design.md` for detailed configuration options and architecture.

## License

[Add your license here]


# mach_five
