# IP watcher

A simple utility periodically checks your external IP address and logs when it changes.

## Features

- Monitors your external IP address at configurable intervals
- Logs only when your IP changes (and on initial startup)
- No external dependencies, uses only Go standard library
- Can be installed as a systemd service for automatic startup
- Supports both IPv4 and IPv6 addresses

## Installation

### Manual Installation

1. Build the binary:
   ```bash
   go build
   ```

2. Run it directly:
   ```bash
   ./ip-watcher
   ```

### Recommended: Installation as a systemd Service

Run the included installer script as root:
```bash
sudo ./install.sh
```

This will:
- Build the binary if needed
- Copy it to `/usr/local/bin/ip-watcher`
- Create a systemd service file that logs to the systemd journal
- Enable and start the service

## Usage

```bash
./ip-watcher [options]
```

### Command-line Options

| Flag | Default | Description |
|------|---------|-------------|
| `-interval` | 60 | Check interval in seconds |
| `-log` | "" | Log file path (if not specified, logs to stdout only) |
| `-endpoint` | "https://api64.ipify.org?format=json" | URL of the IP checking service |
| `-quiet` | false | If true, only logs to file and not stdout |

### Example

Check IP every 5 minutes and log to a file:
```bash
./ip-watcher -interval 300 -log /var/log/ip-watcher.log
```

## Service Management

If installed as a systemd service:

```bash
# Check service status
sudo systemctl status ip-watcher

# Stop the service
sudo systemctl stop ip-watcher

# Start the service
sudo systemctl start ip-watcher

# Disable automatic startup
sudo systemctl disable ip-watcher

# Enable automatic startup
sudo systemctl enable ip-watcher

# View logs (the service logs directly to systemd journal)
sudo journalctl -u ip-watcher
```

## IP Services

The default endpoint is `https://api.ipify.org?format=json`, but you can use any service that returns your IP address. Some alternatives:

- `https://api.ipify.org` (plain text)
- `https://api64.ipify.org?format=json` (IPv6 capable)
- `https://icanhazip.com` (plain text)
- `https://ifconfig.me` (plain text)

## License

MIT License
