# Bitfinex Data Controller

A real-time cryptocurrency data collection system specifically designed for collecting Bitfinex WebSocket data and storing it in Parquet format for ML training purposes.

## Features

- **Real-time WebSocket Data Collection**: Connects to Bitfinex WebSocket API to collect ticker, trades, books, and raw books data
- **Parquet Storage**: Efficient storage with ZSTD compression and 256MB segment rotation
- **GUI Control Interface**: Fyne-based UI for starting/stopping data collection and monitoring statistics
- **High-frequency Data Support**: Optimized for short-term trading strategies (minutes to 2 hours)
- **Bitfinex Protocol Compliance**: Full support for conf flags, heartbeats, checksums, and sequence numbers

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Bitfinex WS   │────│  Connection Mgr  │────│   Data Router   │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                                                         │
┌─────────────────┐    ┌──────────────────┐             │
│   Fyne GUI      │────│  Main App        │             │
└─────────────────┘    └──────────────────┘             │
                                                         │
                                                         ▼
                                               ┌─────────────────┐
                                               │ Parquet Writer  │
                                               └─────────────────┘
                                                         │
                                                         ▼
                                               ┌─────────────────┐
                                               │ Segment Storage │
                                               │  (256MB each)   │
                                               └─────────────────┘
```

## Configuration

Edit `config.yml` to configure:

- **Symbols**: Cryptocurrency pairs to collect (tBTCUSD, tETHUSD, etc.)
- **Channels**: Enable/disable ticker, trades, books, raw_books
- **Storage**: Base path, segment size, compression settings
- **WebSocket**: Connection parameters and Bitfinex conf flags
- **GUI**: Interface settings and refresh intervals

## Usage

### Running the Application

```bash
# Build the application
go build -o data-controller cmd/data-controller/main.go

# Run with default config
./data-controller

# Run with custom config
./data-controller -config path/to/config.yml
```

### GUI Controls

- **Start Data Collection**: Begins WebSocket connection and data collection
- **Stop Data Collection**: Gracefully stops all connections and flushes data
- **Force Flush**: Immediately flushes all buffered data to disk
- **Statistics**: Real-time view of collected data counts and errors

### Data Structure

Data is stored in the following directory structure:

```
/data/bitfinex/v2/{channel}/{symbol}/dt={YYYY-MM-DD}/hour={HH}/
  seg={UTC_START}--{UTC_END}--size~256MB/
    part-{channel}-{symbol}-{timestamp}-seq.parquet
    manifest.json
    controls.parquet (optional)
    wal.jsonl.zst (optional)
```

### Parquet Schema

Each data type (ticker, trades, books, raw_books) has its own optimized schema with:

- **Common fields**: Exchange, channel, symbol, timestamps, connection info
- **Type-specific fields**: Price, amount, order ID, etc.
- **Metadata**: Sequence numbers, checksums, quality metrics

## Dependencies

- **Go 1.21+**
- **Fyne v2**: GUI framework
- **gorilla/websocket**: WebSocket client
- **parquet-go**: Parquet file format support
- **zap**: Structured logging
- **YAML v3**: Configuration parsing

## Bitfinex Integration

This system implements the full Bitfinex WebSocket v2 protocol:

- **Configuration flags**: TIMESTAMP, SEQ_ALL, OB_CHECKSUM, BULK_UPDATES
- **Heartbeat monitoring**: 15-second intervals with 45-second timeout
- **Automatic reconnection**: Handles network issues and server maintenance
- **Checksum validation**: CRC32 validation for order book integrity
- **Sequence tracking**: Gap detection and recovery

## Performance

Optimized for high-frequency data collection:

- **Multi-connection support**: Up to 30 channels per connection
- **Buffered writes**: Configurable flush intervals and row counts
- **Memory management**: Ring buffers with backpressure handling
- **Compression**: ZSTD level 3 for optimal size/speed balance

## Monitoring

Built-in monitoring capabilities:

- **Real-time statistics**: Message counts, error rates, flush times
- **Prometheus metrics**: (planned) Integration with monitoring stack
- **Health checks**: (planned) HTTP endpoints for system monitoring
- **Quality metrics**: Checksum mismatches, heartbeat losses, reconnections

## License

This project is designed for educational and research purposes. Please ensure compliance with Bitfinex API terms of service when using this software.