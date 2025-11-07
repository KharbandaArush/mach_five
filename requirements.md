# System Requirements

## Overview

Build a system to fetch orders, price, date, time and symbols from a google sheet and place trades on the stock market as per the schedule.

## Components

### Read Module

A script to read from the google sheet and push the order details in a cache along with an expiry date 10 seconds from the scheduled date. The module needs to fetch the data from 2 sub sheets within the google sheets. It needs to fetch buy orders from to_buy (B2:J) and sell orders from to_sell(B2:J). The top row is a header - planned_buy_price, product(quantityIs on),	Name, bse_code, symbol, execute_date, execute_time, Money Needed, Lots. 

### Trigger Module

This script will be triggered to place the order. This module keeps the system work by keeping the data, broker connection and system ready before getting triggered by the cron scheduler. Deletes the orders from the shared data structure after execution or failure to prevent duplicate execution.

### Scheduler

The code should use linux OS based Cron to trigger the Trigger module every 1 minute.

### Broker Module

This module fetches and maintains the configuration to execute orders on the broker, the broker can change in the future. This module is also responsible for error handling, rate limiting, profiling. Based on the error it adapts and executes the order again. The broker module needs to place orders to kite (zerodha). The system should place after market orders id the market is closed. The market opens at 9:00 am and closes at 3:30 pm. Split the quantity in multiple orders by dividing them.

## Design Choices

- The system is built in Go
- The system uses cron as a scheduler to minimize scheduler delay
- The system doesn't use docker but is directly deployed on the system

