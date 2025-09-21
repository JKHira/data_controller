#!/bin/bash

echo "=========================================="
echo "Data Collector"
echo "=========================================="
echo "Starting data collection..."
echo "Data will be saved to: $(pwd)/data"
echo "Press Ctrl+C to stop gracefully"
echo "=========================================="

# Create data directory if it doesn't exist
mkdir -p data

# Run the collector
./data-controller -nogui

echo ""
echo "=========================================="
echo "Data collection stopped"
echo "Checking collected data..."

# Show what was collected
if [ -d "data/bitfinex" ]; then
    echo "Data collected successfully:"
    find data/bitfinex -name "*.arrow" -exec ls -lh {} \;
    echo ""
    echo "Directory structure:"
    tree data/bitfinex 2>/dev/null || find data/bitfinex -type d
else
    echo "No data directory found - there may have been an issue"
fi

echo "=========================================="