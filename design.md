# System Design Document

## Architecture Overview

The system is a distributed trading automation platform that reads order instructions from Google Sheets and executes them on a stock market broker according to scheduled times. The architecture follows a modular design with clear separation of concerns.

## System Components

### 1. Read Module

**Purpose**: Fetch order data from Google Sheets and prepare it for execution.

**Responsibilities**:
- Fetch Google sheet details from Config
- Authenticate with Google Sheets API
- Read buy orders from `to_buy` sheet (range: B2:J)
- Read sell orders from `to_sell` sheet (range: B2:J)
- Parse and validate order information from both sheets
- Store orders in a shared cache with expiry timestamps (scheduled time + 10 seconds)


**Source Structure**
- Google Sheets structure:
  - **Two sub-sheets**: `to_buy` and `to_sell`
  - **Range**: B2:J (row 1 is header, data starts from row 2)
  - **Column mapping** (B through J):
    - B: `planned_buy_price` (float) - Price for the order
    - C: `product` (string) - Product type (quantity is on this)
    - D: `Name` (string) - Stock name
    - E: `bse_code` (string) - BSE code
    - F: `symbol` (string) - Trading symbol (e.g., "NSE:RELIANCE" or "BSE:RELIANCE")
    - G: `execute_date` (string) - Date format: YYYY-MM-DD
    - H: `execute_time` (string) - Time format: HH:MM:SS or HH:MM
    - I: `Money Needed` (float) - Money required
    - J: `Lots` (int) - Number of lots (quantity)
  - **Sheet names**:
    - `to_buy` - Contains buy orders
    - `to_sell` - Contains sell orders

**Implementation Details**:
- **Language**: Go
- **Google Sheets Integration**: Use `golang.org/x/oauth2` and Google Sheets API v4
- **Execution**: Run as a background service or scheduled job (can be triggered by cron or run continuously)
- **Cache Details**: 
  - **Cache System**: Redis (using `github.com/go-redis/redis/v8`)
  - **Cache Data Structure**: 
    - Key format: `order:{orderID}` or `order:{symbol}:{scheduledTime}`
    - Value: JSON serialized OrderCacheEntry
    - TTL: Set to expiry time (scheduled time + 10 seconds)
    - Additional set: `pending_orders` (sorted set by scheduled time for efficient querying)

**Error Handling**:
- Retry logic for API failures
- Logging for failed reads
- Graceful degradation if Google Sheets is unavailable


### 2. Trigger Module

**Purpose**: Execute orders that are due for execution.

**Responsibilities**:
- Maintain system readiness (data, broker connection etc.)
- Check cache for orders due for execution
- Trigger order execution via Broker Module
- Clean up executed/failed orders from cache
- Prevent duplicate executions
- Runs multiple threads to execute the required orders
- Profiles the time taken by diffrent steps while placing an order including the schedular delay

**Implementation Details**:
- **Trigger Mechanism**: Called by cron scheduler every 1 minute
- **Order Selection**: Query cache for orders where `current_time >= scheduled_time && current_time <= expiry_time`
- **Execution Flow**:
  1. Validate order is still within expiry window
  2. Call Broker Module to execute order
  3. On success/failure: Remove order from cache
  4. Log execution result
- **Concurrency**: 
  - Use worker pool pattern with configurable number of goroutines
  - Each worker processes orders independently
  - Use Redis atomic operations to prevent duplicate execution
  - Lock mechanism using Redis SETNX for order execution
- **Profiling**:
  - Track scheduler delay (time between scheduled time and actual execution start)
  - Profile time for: cache lookup, broker connection, order execution, cleanup
  - Log profiling metrics to structured logs
  - Metrics format: JSON with timestamps for each step

**Error Handling**:
- If order execution fails, remove from cache to prevent retries
- Log all execution attempts
- Handle broker connection failures gracefully

### 3. Scheduler (Linux Cron)

**Purpose**: Trigger the Trigger Module at regular intervals.

**Implementation Details**:
- **Cron Expression**: `* * * * *` (every minute)
- **Command**: Execute the Trigger Module binary
- **Cron Entry Example**:
  ```
  * * * * * /path/to/trigger-module
  ```
- **Logging**: Redirect stdout/stderr to log files for monitoring

**Considerations**:
- Ensure cron has proper environment variables
- Handle overlapping executions (use file locks if needed)
- Monitor cron service status

### 4. Broker Module

**Purpose**: Execute trades on the stock market broker.

**Responsibilities**:
- Manage broker configuration (API keys, endpoints, credentials)
- Execute buy/sell orders
- **Order Splitting**: Automatically split large orders into multiple smaller orders
  - Divide quantity when it exceeds configured maximum order size (default: 1000)
  - Execute split orders sequentially (respecting rate limits)
  - Aggregate results from all split orders
  - Calculate weighted average execution price
  - Continue execution even if some splits fail (partial execution)
- Handle rate limiting
- Implement retry logic with adaptive strategies
- Error handling and recovery
- Check market hours (9:00 AM - 3:30 PM IST)
- Place After Market Orders (AMO) when market is closed

**Implementation Details**:
- **Configuration Management**:
  - Store broker config in file (JSON/YAML) or environment variables
  - Support multiple broker types (interface-based design)
  - Hot-reload configuration if needed
  
- **Broker Interface**:
  ```go
  type Broker interface {
      ExecuteOrder(order Order) (ExecutionResult, error)
      GetRateLimit() RateLimit
      HealthCheck() error
  }
  ```

- **Rate Limiting**:
  - Token bucket or sliding window algorithm
  - Configurable limits per broker
  - Queue orders if rate limit exceeded

- **Market Hours & AMO**:
  - Market hours: 9:00 AM - 3:30 PM IST (Monday to Friday)
  - Check current time against market hours before placing order
  - If market is closed: Place as After Market Order (AMO)
  - If market is open: Place as regular order
  - AMO orders are queued and executed when market opens next day

- **Error Handling & Retry**:
  - Categorize errors (network, authentication, rate limit, invalid order)
  - Exponential backoff for retries
  - Max retry attempts per order
  - Adaptive retry based on error type:
    - Network errors: Retry with backoff
    - Rate limit: Wait and retry
    - Invalid order: Don't retry, log error
    - Authentication: Alert and don't retry

- **Profiling**:
  - Track execution time per order
  - Track success/failure rates
  - Log metrics to file or monitoring system
  - Alert on high failure rates

## Data Flow

```
Google Sheets → Read Module → Cache (with expiry)
                                    ↓
                            Trigger Module (cron)
                                    ↓
                            Broker Module → Stock Market
```

## Shared Data Structures
Store it in Cache (Redis)

### Cache Structure
- **Type**: Redis (persistent, shared across processes)
- **Key Format**: `order:{orderID}` where orderID = `{symbol}:{scheduledTime}`
- **Value**: JSON serialized OrderCacheEntry
- **TTL**: Automatically expires at expiry time (scheduled time + 10 seconds)
- **Indexing**: 
  - Sorted Set: `pending_orders` with score = scheduled time (unix timestamp)
  - Used for efficient querying of orders due for execution
- **Cleanup**: 
  - Automatic removal via Redis TTL
  - Manual removal after successful/failed execution
  - Background cleanup job for stale entries

### Order Structure
```go
type Order struct {
    ID            string
    Symbol        string
    Price         float64
    Quantity      int  // May need to be inferred or added
    OrderType     string  // Market, Limit, etc.
    Side          string  // Buy, Sell
    ScheduledTime time.Time
}
```

## Configuration Management

### Environment Variables
- `GOOGLE_SHEETS_CREDENTIALS_PATH`: Path to OAuth credentials
- `GOOGLE_SHEET_ID`: ID of the target spreadsheet
- `GOOGLE_SHEET_BUY_RANGE`: Range for buy orders (default: "to_buy!B2:J")
- `GOOGLE_SHEET_SELL_RANGE`: Range for sell orders (default: "to_sell!B2:J")
- `BROKER_CONFIG_PATH`: Path to broker configuration file
- `REDIS_ADDR`: Redis server address (default: localhost:6379)
- `REDIS_PASSWORD`: Redis password (if required)
- `REDIS_DB`: Redis database number (default: 0)
- `LOG_LEVEL`: Logging level (DEBUG, INFO, WARN, ERROR)
- `WORKER_POOL_SIZE`: Number of concurrent workers in trigger module (default: 5)
- `MAX_ORDER_SIZE`: Maximum quantity per order before splitting (default: 1000)

### Configuration Files
- Broker configuration (JSON/YAML)
- Google Sheets credentials (JSON)

## Error Handling Strategy

1. **Read Module Errors**:
   - Retry with 0 delay
   - Log errors and continue operation
   - Alert if persistent failures

2. **Trigger Module Errors**:
   - Log all errors
   - Continue processing other orders
   - Remove failed orders from cache

3. **Broker Module Errors**:
   - Categorize and handle appropriately
   - Retry with adaptive strategy
   - Alert on critical failures

## Logging & Monitoring

- **Log Levels**: DEBUG, INFO, WARN, ERROR
- **Log Files**: Separate files per module or centralized logging
- **Metrics**: Track execution times, success rates, cache size
- **Alerts**: High failure rates, broker connection issues

## Deployment

- **No Docker**: Direct binary deployment
- **Platform**: GCP Compute Engine instance
- **Process Management**: Use systemd for service management
- **Binary Distribution**: Combined binary with subcommands (read, trigger)
- **File Structure**:
  ```
  /opt/trading-system/
    ├── bin/
    │   └── trading-system (combined binary)
    ├── config/
    │   ├── broker-config.json
    │   └── google-credentials.json
    ├── logs/
    │   ├── read-module.log
    │   ├── trigger-module.log
    │   └── broker-module.log
    └── scripts/
        ├── deploy.sh
        └── setup-cron.sh
  ```
- **GCP Setup**:
  - Install Redis on the instance
  - Configure systemd services for read-module (continuous) and cron for trigger-module
  - Set up monitoring and logging
  - Configure firewall rules if needed

## Security Considerations

- Secure storage of API credentials
- File permissions for configuration files
- OAuth token refresh for Google Sheets
- Broker API key encryption at rest
- Audit logging for all trade executions

## Future Enhancements

- Support for multiple brokers simultaneously
- Webhook-based triggers in addition to cron
- Database persistence for order history
- Web dashboard for monitoring
- Order modification/cancellation support

## Testing Strategy

- Unit tests for each module
- Integration tests for Google Sheets API
- Mock broker for testing execution logic
- End-to-end tests with test accounts
- Load testing for rate limiting

