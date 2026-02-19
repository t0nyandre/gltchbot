#!/bin/bash
# setup-monitor.sh - Installation script for Discord bot stack monitor
#
# Sets up the monitor-stack.sh script as a systemd service with:
# - Automatic startup on boot
# - Log rotation configuration
# - Proper permissions and environment
#
# Usage: sudo ./scripts/setup-monitor.sh [--user USER] [--install-dir DIR]

set -euo pipefail

# Default configuration
DEFAULT_USER="${SUDO_USER:-$USER}"
DEFAULT_INSTALL_DIR="/opt/gltchbot-monitor"
DEFAULT_SERVICE_NAME="gltchbot-monitor"
DEFAULT_LOG_DIR="/var/log/gltchbot"
DEFAULT_INTERVAL=30

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Print colored output
print_color() {
    local color="$1"
    local message="$2"
    echo -e "${color}${message}${NC}"
}

print_error() { print_color "$RED" "$1"; }
print_success() { print_color "$GREEN" "$1"; }
print_warn() { print_color "$YELLOW" "$1"; }
print_info() { print_color "$BLUE" "$1"; }

# Check if running as root
check_root() {
    if [[ $EUID -ne 0 ]]; then
        print_error "This script must be run as root (use sudo)"
        exit 1
    fi
}

# Check if user exists
user_exists() {
    local user="$1"
    id "$user" >/dev/null 2>&1
}

# Get the project root directory
get_project_root() {
    local script_dir
    script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    echo "$(dirname "$script_dir")"
}

show_help() {
    cat << EOF
${BLUE}Discord Bot Monitor Setup Script${NC}

${GREEN}Usage:${NC}
  sudo ./scripts/setup-monitor.sh [OPTIONS]

${GREEN}Options:${NC}
  --user USER          User to run the monitor as (default: \$SUDO_USER)
  --install-dir DIR    Installation directory (default: $DEFAULT_INSTALL_DIR)
  --interval SECONDS   Check interval in seconds (default: $DEFAULT_INTERVAL)
  --help               Show this help message

${GREEN}What this script does:${NC}
  1. Copies monitor script to installation directory
  2. Creates systemd service file
  3. Sets up log rotation
  4. Enables and starts the service
  5. Configures automatic startup on boot

${GREEN}Examples:${NC}
  # Default installation
  sudo ./scripts/setup-monitor.sh

  # Custom user and directory
  sudo ./scripts/setup-monitor.sh --user deploy --install-dir /opt/monitor

  # Custom check interval (60 seconds)
  sudo ./scripts/setup-monitor.sh --interval 60

EOF
}

parse_arguments() {
    INSTALL_USER="$DEFAULT_USER"
    INSTALL_DIR="$DEFAULT_INSTALL_DIR"
    CHECK_INTERVAL="$DEFAULT_INTERVAL"
    
    while [[ $# -gt 0 ]]; do
        case $1 in
            --user)
                if [[ -z "${2:-}" ]]; then
                    print_error "Error: --user requires a username"
                    exit 1
                fi
                INSTALL_USER="$2"
                shift 2
                ;;
            --install-dir)
                if [[ -z "${2:-}" ]]; then
                    print_error "Error: --install-dir requires a directory path"
                    exit 1
                fi
                INSTALL_DIR="$2"
                shift 2
                ;;
            --interval)
                if [[ -z "${2:-}" ]] || ! [[ "$2" =~ ^[0-9]+$ ]]; then
                    print_error "Error: --interval requires a positive number"
                    exit 1
                fi
                CHECK_INTERVAL="$2"
                shift 2
                ;;
            --help)
                show_help
                exit 0
                ;;
            *)
                print_error "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done
}

validate_environment() {
    print_info "Validating environment..."
    
    # Check if user exists
    if ! user_exists "$INSTALL_USER"; then
        print_error "User '$INSTALL_USER' does not exist"
        exit 1
    fi
    
    # Check if Docker is installed
    if ! command -v docker >/dev/null 2>&1; then
        print_error "Docker is not installed or not in PATH"
        exit 1
    fi
    
    # Check if Docker Compose is available
    if ! command -v docker compose >/dev/null 2>&1; then
        print_error "Docker Compose is not available"
        print_warn "Try: docker-compose or install docker-compose-plugin"
        exit 1
    fi
    
    # Check if curl is available (for API health checks)
    if ! command -v curl >/dev/null 2>&1; then
        print_warn "curl is not installed. API health checks will fail."
        print_warn "Install with: apt-get install curl"
    fi
    
    print_success "Environment validation passed"
}

install_monitor_script() {
    print_info "Installing monitor script..."
    
    local project_root
    project_root=$(get_project_root)
    local source_script="$project_root/scripts/monitor-stack.sh"
    
    if [[ ! -f "$source_script" ]]; then
        print_error "Monitor script not found: $source_script"
        exit 1
    fi
    
    # Create installation directory
    mkdir -p "$INSTALL_DIR"
    
    # Copy monitor script
    cp "$source_script" "$INSTALL_DIR/monitor-stack.sh"
    
    # Make it executable
    chmod +x "$INSTALL_DIR/monitor-stack.sh"
    
    # Create configuration file
    cat > "$INSTALL_DIR/monitor.env" << EOF
# Discord Bot Monitor Configuration
# This file is sourced by the systemd service

# Check interval in seconds
MONITOR_INTERVAL=$CHECK_INTERVAL

# Log file location
MONITOR_LOG_FILE="$DEFAULT_LOG_DIR/monitor.log"

# API port (should match your docker-compose.yml)
API_PORT=8080

# Dry run mode (set to "true" for testing)
# MONITOR_DRY_RUN=false

# Additional environment variables can be added here
# They will be available to the monitor script
EOF
    
    # Set ownership
    chown -R "$INSTALL_USER:$INSTALL_USER" "$INSTALL_DIR"
    
    print_success "Monitor script installed to $INSTALL_DIR"
}

create_log_directory() {
    print_info "Setting up log directory..."
    
    # Create log directory
    mkdir -p "$DEFAULT_LOG_DIR"
    
    # Set ownership
    chown "$INSTALL_USER:$INSTALL_USER" "$DEFAULT_LOG_DIR"
    
    # Create initial log file
    touch "$DEFAULT_LOG_DIR/monitor.log"
    chown "$INSTALL_USER:$INSTALL_USER" "$DEFAULT_LOG_DIR/monitor.log"
    
    print_success "Log directory created: $DEFAULT_LOG_DIR"
}

setup_log_rotation() {
    print_info "Setting up log rotation..."
    
    local logrotate_conf="/etc/logrotate.d/$DEFAULT_SERVICE_NAME"
    
    cat > "$logrotate_conf" << EOF
$DEFAULT_LOG_DIR/monitor.log {
    daily
    missingok
    rotate 7
    compress
    delaycompress
    notifempty
    create 0640 $INSTALL_USER $INSTALL_USER
    postrotate
        systemctl try-reload-or-restart $DEFAULT_SERVICE_NAME >/dev/null 2>&1 || true
    endscript
}
EOF
    
    chmod 0644 "$logrotate_conf"
    
    print_success "Log rotation configured: $logrotate_conf"
    print_info "Logs will be kept for 7 days and compressed"
}

create_systemd_service() {
    print_info "Creating systemd service..."
    
    local service_file="/etc/systemd/system/$DEFAULT_SERVICE_NAME.service"
    local project_root
    project_root=$(get_project_root)
    
    cat > "$service_file" << EOF
[Unit]
Description=Discord Bot Stack Monitor
After=docker.service
Requires=docker.service
Wants=network-online.target
After=network-online.target

[Service]
Type=simple
User=$INSTALL_USER
Group=$INSTALL_USER
WorkingDirectory=$project_root

# Environment variables
EnvironmentFile=$INSTALL_DIR/monitor.env

# Ensure Docker socket is accessible
Environment=DOCKER_HOST=unix:///var/run/docker.sock

# Monitor script
ExecStart=$INSTALL_DIR/monitor-stack.sh \\
    --interval \$MONITOR_INTERVAL \\
    --log \$MONITOR_LOG_FILE

# Restart on failure
Restart=on-failure
RestartSec=10

# Standard output/error logging
StandardOutput=journal
StandardError=journal

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=$DEFAULT_LOG_DIR $INSTALL_DIR

[Install]
WantedBy=multi-user.target
EOF
    
    chmod 0644 "$service_file"
    
    print_success "Systemd service created: $service_file"
}

enable_and_start_service() {
    print_info "Enabling and starting service..."
    
    # Reload systemd daemon
    systemctl daemon-reload
    
    # Enable service to start on boot
    if systemctl enable "$DEFAULT_SERVICE_NAME.service" >/dev/null 2>&1; then
        print_success "Service enabled to start on boot"
    else
        print_warn "Failed to enable service (may already be enabled)"
    fi
    
    # Start the service
    if systemctl start "$DEFAULT_SERVICE_NAME.service"; then
        print_success "Service started successfully"
    else
        print_error "Failed to start service"
        print_warn "Check status with: systemctl status $DEFAULT_SERVICE_NAME"
        exit 1
    fi
    
    # Show service status
    print_info "Service status:"
    systemctl status "$DEFAULT_SERVICE_NAME.service" --no-pager --lines=5
}

create_manual_test_script() {
    print_info "Creating test script..."
    
    local test_script="$INSTALL_DIR/test-monitor.sh"
    
    cat > "$test_script" << EOF
#!/bin/bash
# test-monitor.sh - Manual test script for the monitor

set -euo pipefail

echo "Testing Discord Bot Monitor"
echo "=========================="
echo ""

# Test 1: Check if service is running
echo "1. Checking service status..."
if systemctl is-active --quiet $DEFAULT_SERVICE_NAME.service; then
    echo "   ✓ Service is running"
else
    echo "   ✗ Service is not running"
    echo "   Run: sudo systemctl start $DEFAULT_SERVICE_NAME.service"
fi

# Test 2: Check logs
echo ""
echo "2. Checking logs..."
if [[ -f "$DEFAULT_LOG_DIR/monitor.log" ]]; then
    echo "   ✓ Log file exists: $DEFAULT_LOG_DIR/monitor.log"
    echo "   Last 5 lines:"
    tail -5 "$DEFAULT_LOG_DIR/monitor.log" | sed 's/^/     /'
else
    echo "   ✗ Log file not found"
fi

# Test 3: Dry run test
echo ""
echo "3. Testing monitor in dry-run mode..."
cd "$(get_project_root)"
if ./scripts/monitor-stack.sh --dry-run --verbose --interval 5; then
    echo "   ✓ Dry run test passed"
else
    echo "   ✗ Dry run test failed"
fi

# Test 4: Docker stack status
echo ""
echo "4. Checking Docker stack..."
if docker compose ps >/dev/null 2>&1; then
    echo "   ✓ Docker Compose is accessible"
    echo "   Container status:"
    docker compose ps --format "table {{.Name}}\t{{.Status}}\t{{.Ports}}" | sed 's/^/     /'
else
    echo "   ✗ Cannot access Docker Compose"
fi

echo ""
echo "Test complete!"
echo ""
echo "Useful commands:"
echo "  sudo systemctl status $DEFAULT_SERVICE_NAME"
echo "  sudo journalctl -u $DEFAULT_SERVICE_NAME -f"
echo "  tail -f $DEFAULT_LOG_DIR/monitor.log"
echo "  sudo systemctl restart $DEFAULT_SERVICE_NAME"
EOF
    
    chmod +x "$test_script"
    chown "$INSTALL_USER:$INSTALL_USER" "$test_script"
    
    print_success "Test script created: $test_script"
}

show_completion_message() {
    local project_root
    project_root=$(get_project_root)
    
    cat << EOF

${GREEN}===============================================${NC}
${GREEN}   Discord Bot Monitor Installation Complete   ${NC}
${GREEN}===============================================${NC}

${BLUE}Installation Summary:${NC}
  • Monitor script:      $INSTALL_DIR/monitor-stack.sh
  • Configuration:       $INSTALL_DIR/monitor.env
  • Systemd service:     $DEFAULT_SERVICE_NAME.service
  • Log directory:       $DEFAULT_LOG_DIR
  • Check interval:      ${CHECK_INTERVAL}s
  • Run as user:         $INSTALL_USER

${BLUE}Service Management:${NC}
  Check status:    sudo systemctl status $DEFAULT_SERVICE_NAME
  View logs:       sudo journalctl -u $DEFAULT_SERVICE_NAME -f
  Restart:         sudo systemctl restart $DEFAULT_SERVICE_NAME
  Stop:            sudo systemctl stop $DEFAULT_SERVICE_NAME
  Enable/disable:  sudo systemctl enable|disable $DEFAULT_SERVICE_NAME

${BLUE}Log Files:${NC}
  Monitor logs:    $DEFAULT_LOG_DIR/monitor.log (rotates daily, keeps 7 days)
  System logs:     sudo journalctl -u $DEFAULT_SERVICE_NAME

${BLUE}Testing:${NC}
  Run test script: $INSTALL_DIR/test-monitor.sh
  Dry-run test:    cd $project_root && ./scripts/monitor-stack.sh --dry-run --verbose

${BLUE}Configuration:${NC}
  Edit settings:   sudo nano $INSTALL_DIR/monitor.env
  Then reload:     sudo systemctl restart $DEFAULT_SERVICE_NAME

${BLUE}Next Steps:${NC}
  1. Monitor will automatically check your Docker stack every ${CHECK_INTERVAL} seconds
  2. Check logs to ensure it's working: tail -f $DEFAULT_LOG_DIR/monitor.log
  3. Test failure scenarios by stopping containers: docker compose stop bot

${YELLOW}Note: The monitor requires Docker and Docker Compose to be installed${NC}
${YELLOW}and accessible by the $INSTALL_USER user.${NC}

EOF
}

main() {
    # Parse arguments first (to handle --help)
    parse_arguments "$@"
    
    print_info "Starting Discord Bot Monitor installation..."
    echo ""
    
    # Check root privileges
    check_root
    
    # Show installation summary
    print_info "Installation Summary:"
    print_info "  User:          $INSTALL_USER"
    print_info "  Install dir:   $INSTALL_DIR"
    print_info "  Check interval: ${CHECK_INTERVAL}s"
    print_info "  Service name:  $DEFAULT_SERVICE_NAME"
    echo ""
    
    # Validate environment
    validate_environment
    
    # Installation steps
    install_monitor_script
    create_log_directory
    setup_log_rotation
    create_systemd_service
    enable_and_start_service
    create_manual_test_script
    
    # Show completion message
    show_completion_message
}

# Run main function
main "$@"