# Discord Bot Stack Monitoring

This document describes the production monitoring solution for the Discord bot Docker stack.

## Overview

The monitoring system consists of two main scripts:

1. **`scripts/monitor-stack.sh`** - Main monitoring script that checks the health of bot, API, and PostgreSQL containers
2. **`scripts/setup-monitor.sh`** - Installation script that sets up the monitor as a systemd service

## Features

- ✅ **Multi-service monitoring**: Checks bot, API, and PostgreSQL containers
- ✅ **Health endpoint verification**: Tests API `/health` endpoint
- ✅ **Database connectivity**: Verifies PostgreSQL is reachable
- ✅ **Exponential backoff**: Prevents restart loops with intelligent backoff timing
- ✅ **Comprehensive logging**: Timestamped logs with rotation
- ✅ **Production-safe**: Proper error handling and resource management
- ✅ **Configurable**: Adjustable check interval, log location, etc.
- ✅ **Dry-run mode**: Test without actually restarting services

## Quick Start

### 1. Test the Monitor (Dry Run)

Before installing, test the monitor in dry-run mode:

```bash
cd /home/t0nyandre/vibe-code-discord-bot
./scripts/monitor-stack.sh --dry-run --verbose --interval 10
```

This will run checks every 10 seconds without restarting anything. Press `Ctrl+C` to stop.

### 2. Install as Systemd Service

Install the monitor as a systemd service that starts automatically on boot:

```bash
sudo ./scripts/setup-monitor.sh
```

By default, this will:
- Install to `/opt/gltchbot-monitor/`
- Run as the current user
- Check every 30 seconds
- Store logs in `/var/log/gltchbot/monitor.log`
- Configure log rotation (7 days retention)

### 3. Custom Installation

For custom configuration:

```bash
# Custom user, directory, and interval
sudo ./scripts/setup-monitor.sh --user deploy --install-dir /opt/monitor --interval 60

# Check interval only
sudo ./scripts/setup-monitor.sh --interval 45
```

## Manual Usage

### Basic Monitoring

```bash
# Default settings (30s interval, logs to ./monitor.log)
./scripts/monitor-stack.sh

# Custom interval and log location
./scripts/monitor-stack.sh --interval 60 --log /var/log/bot-monitor.log

# Verbose output to terminal
./scripts/monitor-stack.sh --verbose

# Dry run for testing
./scripts/monitor-stack.sh --dry-run --verbose
```

### Environment Variables

Configure via environment variables:

```bash
export MONITOR_INTERVAL=45
export MONITOR_LOG_FILE="/var/log/bot/monitor.log"
export MONITOR_DRY_RUN="true"
export API_PORT=8080
./scripts/monitor-stack.sh
```

## Service Management

Once installed as a systemd service:

```bash
# Check status
sudo systemctl status gltchbot-monitor

# View logs
sudo journalctl -u gltchbot-monitor -f
tail -f /var/log/gltchbot/monitor.log

# Restart service
sudo systemctl restart gltchbot-monitor

# Stop service
sudo systemctl stop gltchbot-monitor

# Enable/disable auto-start on boot
sudo systemctl enable gltchbot-monitor
sudo systemctl disable gltchbot-monitor
```

## What Gets Monitored

### 1. Docker Containers
- **bot**: `gltchbot-bot` container status
- **api**: `gltchbot-api` container status  
- **postgres**: `gltchbot-postgres` container status

### 2. API Health
- HTTP GET request to `http://localhost:8080/health`
- Expects `{"status":"ok"}` response
- 5-second timeout

### 3. Database Connectivity
- Uses `pg_isready` inside PostgreSQL container
- Verifies database is accepting connections

## Restart Logic

### Priority Order:
1. **PostgreSQL down** → Restart entire stack (critical dependency)
2. **API down** → Restart API container only
3. **Bot down** → Restart bot container only

### Exponential Backoff:
- Prevents restart loops
- Backoff sequence: 0s, 30s, 60s, 120s, 240s, 300s (max)
- Resets when service becomes healthy

## Logging

### Log Locations:
- **System logs**: `journalctl -u gltchbot-monitor`
- **Monitor logs**: `/var/log/gltchbot/monitor.log` (default)
- **Custom location**: Set via `--log` flag or `MONITOR_LOG_FILE`

### Log Format:
```
[2025-02-19 12:00:00] [INFO] Starting Discord Bot Stack Monitor
[2025-02-19 12:00:00] [INFO] Monitor started. Press Ctrl+C to stop.
[2025-02-19 12:00:00] [DEBUG] Starting health checks...
[2025-02-19 12:00:00] [DEBUG] bot is running (container: a1b2c3d4e5f6)
[2025-02-19 12:00:00] [DEBUG] api is running (container: b2c3d4e5f6g7)
[2025-02-19 12:00:00] [DEBUG] postgres is running (container: c3d4e5f6g7h8)
[2025-02-19 12:00:00] [DEBUG] All checks passed
```

### Log Rotation:
- Configured via `/etc/logrotate.d/gltchbot-monitor`
- Rotates daily
- Keeps 7 days of logs
- Compresses old logs

## Testing

### Test Script
After installation, a test script is created:

```bash
/opt/gltchbot-monitor/test-monitor.sh
```

### Manual Testing
```bash
# Test API health endpoint
curl http://localhost:8080/health

# Check container status
docker compose ps

# Simulate failure
docker compose stop bot
# Monitor should detect and restart (if not in dry-run)

# Check monitor logs
tail -f /var/log/gltchbot/monitor.log
```

## Configuration

### Configuration File
`/opt/gltchbot-monitor/monitor.env`:
```bash
# Discord Bot Monitor Configuration
MONITOR_INTERVAL=30
MONITOR_LOG_FILE="/var/log/gltchbot/monitor.log"
API_PORT=8080
# MONITOR_DRY_RUN=false
```

Edit and restart the service:
```bash
sudo nano /opt/gltchbot-monitor/monitor.env
sudo systemctl restart gltchbot-monitor
```

### Docker Compose Requirements
The monitor expects these container names (from `docker-compose.yml`):
- `gltchbot-bot`
- `gltchbot-api` 
- `gltchbot-postgres`

## Troubleshooting

### Common Issues

1. **Permission denied**
   ```bash
   sudo chmod +x scripts/*.sh
   sudo ./scripts/setup-monitor.sh
   ```

2. **Docker not accessible**
   ```bash
   # Add user to docker group
   sudo usermod -aG docker $USER
   # Log out and back in
   ```

3. **API health check fails**
   ```bash
   # Check if API is running
   docker compose ps api
   # Test manually
   curl -v http://localhost:8080/health
   ```

4. **Service won't start**
   ```bash
   # Check systemd logs
   sudo journalctl -u gltchbot-monitor -xe
   # Check syntax
   sudo systemctl daemon-reload
   ```

### Debug Mode
Run with verbose output:
```bash
./scripts/monitor-stack.sh --verbose --interval 5
```

## Architecture

```
Systemd Service (gltchbot-monitor)
        ↓
monitor-stack.sh (main loop)
        ↓
Checks every 30s:
├── Docker container status
├── API health endpoint
├── Database connectivity
        ↓
Actions:
├── Log issues with timestamps
├── Restart services with backoff
├── Reset counters when healthy
```

## Security Considerations

- The monitor runs with minimal privileges
- Systemd service includes security hardening:
  - `NoNewPrivileges=true`
  - `PrivateTmp=true`
  - `ProtectSystem=strict`
  - `ProtectHome=true`
- Only requires Docker socket access
- Logs are owned by service user

## Extending

### Adding Notifications
Edit `monitor-stack.sh` to add notification hooks:

```bash
# In perform_checks() function, add:
send_slack_notification() {
    local message="$1"
    curl -X POST -H 'Content-type: application/json' \
        --data "{\"text\":\"$message\"}" \
        $SLACK_WEBHOOK_URL
}

# Call when issues are detected
if [[ "$bot_ok" == "false" ]]; then
    send_slack_notification "Bot container is down!"
    # ... existing restart logic
fi
```

### Adding More Health Checks
Extend the `perform_checks()` function to add:
- Disk space monitoring
- Memory usage checks
- Network connectivity tests
- Custom health endpoints

## Maintenance

### Updating the Monitor
```bash
# Update scripts in project
git pull origin main

# Re-run setup (preserves config)
sudo ./scripts/setup-monitor.sh
```

### Checking Resource Usage
```bash
# Monitor process
ps aux | grep monitor-stack

# Log file size
du -h /var/log/gltchbot/monitor.log

# System resource usage
systemctl status gltchbot-monitor --no-pager --lines=20
```

## Support

For issues or questions:
1. Check logs: `tail -f /var/log/gltchbot/monitor.log`
2. System logs: `sudo journalctl -u gltchbot-monitor -f`
3. Test manually: `./scripts/monitor-stack.sh --dry-run --verbose`

---

*Last updated: $(date +%Y-%m-%d)*