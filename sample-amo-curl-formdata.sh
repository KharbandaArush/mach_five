#!/bin/bash

# Sample curl command for Kite AMO (After Market Order) using form-data
# This uses multipart/form-data format instead of JSON

# Configuration (replace with your actual values)
API_KEY="ov1cwm7907krzbe5"
ACCESS_TOKEN="your_access_token_here"
BASE_URL="https://kite.zerodha.com"

# Order details (example values)
EXCHANGE="NSE"
TRADING_SYMBOL="RELIANCE"
TRANSACTION_TYPE="BUY"  # or "SELL"
ORDER_TYPE="MARKET"     # or "LIMIT"
QUANTITY=1
PRICE=2500.00            # Required for LIMIT orders, optional for MARKET
PRODUCT="CNC"           # CNC (Cash and Carry) for delivery-based trades
VALIDITY="DAY"          # Always DAY for AMO orders

# Execute curl command with form-data (-F flag)
curl -X POST "${BASE_URL}/oms/orders/amo" \
  -H "X-Kite-Version: 3" \
  -H "Authorization: token ${API_KEY}:${ACCESS_TOKEN}" \
  -F "exchange=${EXCHANGE}" \
  -F "tradingsymbol=${TRADING_SYMBOL}" \
  -F "transaction_type=${TRANSACTION_TYPE}" \
  -F "order_type=${ORDER_TYPE}" \
  -F "variety=amo" \
  -F "quantity=${QUANTITY}" \
  -F "price=${PRICE}" \
  -F "product=${PRODUCT}" \
  -F "validity=${VALIDITY}" \
  -v

# Single-line version:
# curl -X POST "https://kite.zerodha.com/oms/orders/amo" -H "X-Kite-Version: 3" -H "Authorization: token ov1cwm7907krzbe5:your_access_token_here" -F "exchange=NSE" -F "tradingsymbol=RELIANCE" -F "transaction_type=BUY" -F "order_type=MARKET" -F "variety=amo" -F "quantity=1" -F "price=2500.00" -F "product=CNC" -F "validity=DAY"


