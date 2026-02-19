#!/bin/bash
# monitor-stack.sh - Production monitoring script for Discord bot Docker stack
#
# Monitors: bot, API, and PostgreSQL containers
# Checks: Container status, API health endpoint, optional database connectivity
# Actions: Restarts failed services with exponential backoff
# Logging: Timestamped logs to ./monitor.log (configurable)
#
# Usage: ./scripts/monitor-stack.sh [--interval SECONDS] [--log FILE] [--dry-run]
#
# Configuration via environment variables or command line arguments.
# Designed to run as a systemd service or cron job.

set -euo pipefail

# Default configuration
DEFAULT_INTERVAL=30
DEFAULT_LOG_FILE="./monitor.log"
DEFAULT_MAX_RETRIES=5
DEFAULT_BACKOFF_BASE=2
DEFAULT_API_PORT=8080
DEFAULT_API_TIMEOUT=5

# Service names (Docker Compose service names)
BOT_SERVICE="bot"
API_SERVICE="api"
POSTGRES_SERVICE="postgres"

# Global state
declare -A RESTART_ATTEMPTS
declare -A BACKOFF_TIMERS
MONITOR_PID=$$
LOG_FILE="$DEFAULT_LOG_FILE"
CHECK_INTERVAL="$DEFAULT_INTERVAL"
DRY_RUN=false
VERBOSE=false

# Colors for output (disabled when logging to file)
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Logging functions
log() {
    local timestamp
    timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    local message="$1"
    local level="${2:-INFO}"
    
    if [[ -t 1 ]] && [[ "$VERBOSE" == "true" ]]; then
        case "$level" in
            ERROR) echo -e "${RED}[$timestamp] [$level]${NC} $message" ;;
            WARN)  echo -e "${YELLOW}[$timestamp] [$level]${NC} $message" ;;
            INFO)  echo -e "${GREEN}[$timestamp] [$level]${NC} $message" ;;
            DEBUG) echo -e "${CYAN}[$timestamp] [$level]${NC} $message" ;;
            *)     echo -e "${BLUE}[$timestamp] [$level]${NC} $message" ;;
        esac
    fi
    
    echo "[$timestamp] [$level] $message" >> "$LOG_FILE"
}

log_error() { log "$1" "ERROR"; }
log_warn() { log "$1" "WARN"; }
log_info() { log "$1" "INFO"; }
log_debug() { log "$1" "DEBUG"; }

# Utility functions
get_container_id() {
    local service="$1"
    docker ps -q -f "name=gltchbot-$service" 2>/dev/null || echo ""
}

is_container_running() {
    local service="$1"
    local container_id
    container_id=$(get_container_id "$service")
    [[ -n "$container_id" ]]
}

get_container_status() {
    local service="$1"
    docker inspect -f '{{.State.Status}}' "gltchbot-$service" 2>/dev/null || echo "not_found"
}

check_api_health() {
    local timeout="${1:-$DEFAULT_API_TIMEOUT}"
    local port="${2:-$DEFAULT_API_PORT}"
    
    if curl -s -f -m "$timeout" "http://localhost:$port/health" >/dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

check_database_connectivity() {
    local container_id
    container_id=$(get_container_id "$POSTGRES_SERVICE")
    
    if [[ -z "$container_id" ]]; then
        return 1
    fi
    
    # Try to connect to PostgreSQL using pg_isready inside the container
    if docker exec "$container_id" pg_isready -q 2>/dev/null; then
        return 0
    else
        return 1
    fi
}

calculate_backoff() {
    local service="$1"
    local attempts="${RESTART_ATTEMPTS[$service]:-0}"
    
    if [[ $attempts -eq 0 ]]; then
        echo 0
        return
    fi
    
    # Exponential backoff: base^attempts, capped at 300 seconds (5 minutes)
    local backoff=$((DEFAULT_BACKOFF_BASE ** (attempts - 1)))
    if [[ $backoff -gt 300 ]]; then
        backoff=300
    fi
    
    echo "$backoff"
}

should_restart() {
    local service="$1"
    local attempts="${RESTART_ATTEMPTS[$service]:-0}"
    local last_attempt="${BACKOFF_TIMERS[$service]:-0}"
    local current_time
    current_time=$(date +%s)
    local backoff
    
    if [[ $attempts -eq 0 ]]; then
        return 0
    fi
    
    backoff=$(calculate_backoff "$service")
    
    # Check if enough time has passed since last attempt
    if [[ $((current_time - last_attempt)) -ge $backoff ]]; then
        return 0
    else
        return 1
    fi
}

record_restart_attempt() {
    local service="$1"
    local current_time
    current_time=$(date +%s)
    
    RESTART_ATTEMPTS[$service]=$((${RESTART_ATTEMPTS[$service]:-0} + 1))
    BACKOFF_TIMERS[$service]=$current_time
    
    log_warn "Restart attempt #${RESTART_ATTEMPTS[$service]} for $service (next backoff: $(calculate_backoff "$service")s)"
}

reset_restart_attempts() {
    local service="$1"
    unset "RESTART_ATTEMPTS[$service]"
    unset "BACKOFF_TIMERS[$service]"
    log_debug "Reset restart attempts for $service"
}

restart_service() {
    local service="$1"
    local reason="$2"
    
    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "DRY RUN: Would restart $service because: $reason"
        return 0
    fi
    
    log_warn "Restarting $service: $reason"
    
    if docker compose restart "$service" >/dev/null 2>&1; then
        log_info "Successfully restarted $service"
        record_restart_attempt "$service"
        return 0
    else
        log_error "Failed to restart $service"
        return 1
    fi
}

restart_stack() {
    local reason="$1"
    
    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "DRY RUN: Would restart entire stack because: $reason"
        return 0
    fi
    
    log_warn "Restarting entire Docker stack: $reason"
    
    if docker compose down && docker compose up -d; then
        log_info "Successfully restarted entire stack"
        # Reset all restart attempts after full stack restart
        RESTART_ATTEMPTS=()
        BACKOFF_TIMERS=()
        return 0
    else
        log_error "Failed to restart entire stack"
        return 1
    fi
}

check_service() {
    local service="$1"
    local container_id
    local status
    
    container_id=$(get_container_id "$service")
    
    if [[ -z "$container_id" ]]; then
        log_error "$service container not found"
        return 1
    fi
    
    status=$(get_container_status "$service")
    
    case "$status" in
        running)
            log_debug "$service is running (container: ${container_id:0:12})"
            return 0
            ;;
        restarting)
            log_warn "$service is restarting"
            return 1
            ;;
        exited|dead)
            log_error "$service has exited/stopped (status: $status)"
            return 1
            ;;
        not_found)
            log_error "$service container not found"
            return 1
            ;;
        *)
            log_warn "$service in unknown state: $status"
            return 1
            ;;
    esac
}

perform_checks() {
    local issues=0
    local postgres_ok=true
    local api_ok=true
    local bot_ok=true
    
    log_debug "Starting health checks..."
    
    # Check PostgreSQL
    if ! check_service "$POSTGRES_SERVICE"; then
        log_error "PostgreSQL check failed"
        postgres_ok=false
        ((issues++))
    elif ! check_database_connectivity; then
        log_error "PostgreSQL connectivity check failed"
        postgres_ok=false
        ((issues++))
    fi
    
    # Check API
    if ! check_service "$API_SERVICE"; then
        log_error "API container check failed"
        api_ok=false
        ((issues++))
    elif ! check_api_health; then
        log_error "API health endpoint check failed"
        api_ok=false
        ((issues++))
    fi
    
    # Check Bot
    if ! check_service "$BOT_SERVICE"; then
        log_error "Bot container check failed"
        bot_ok=false
        ((issues++))
    fi
    
    # Determine actions based on checks
    if [[ "$postgres_ok" == "false" ]]; then
        log_error "Critical: PostgreSQL is down, restarting entire stack"
        if should_restart "stack"; then
            restart_stack "PostgreSQL failure"
        else
            log_warn "Skipping stack restart due to backoff timer"
        fi
    elif [[ "$api_ok" == "false" ]]; then
        log_warn "API issue detected"
        if should_restart "$API_SERVICE"; then
            restart_service "$API_SERVICE" "API health check failed"
        else
            log_warn "Skipping API restart due to backoff timer"
        fi
    elif [[ "$bot_ok" == "false" ]]; then
        log_warn "Bot issue detected"
        if should_restart "$BOT_SERVICE"; then
            restart_service "$BOT_SERVICE" "Bot container not healthy"
        else
            log_warn "Skipping bot restart due to backoff timer"
        fi
    fi
    
    # Reset restart attempts for healthy services
    if [[ "$postgres_ok" == "true" ]]; then
        reset_restart_attempts "$POSTGRES_SERVICE"
    fi
    if [[ "$api_ok" == "true" ]]; then
        reset_restart_attempts "$API_SERVICE"
    fi
    if [[ "$bot_ok" == "true" ]]; then
        reset_restart_attempts "$BOT_SERVICE"
    fi
    
    if [[ $issues -eq 0 ]]; then
        log_debug "All checks passed"
        return 0
    else
        log_warn "Found $issues issue(s)"
        return 1
    fi
}

show_help() {
    cat << EOF
${BLUE}Discord Bot Stack Monitor${NC}

${GREEN}Usage:${NC}
  ./scripts/monitor-stack.sh [OPTIONS]

${GREEN}Options:${NC}
  --interval SECONDS    Check interval in seconds (default: $DEFAULT_INTERVAL)
  --log FILE            Log file path (default: $DEFAULT_LOG_FILE)
  --dry-run             Simulate actions without actually restarting
  --verbose             Show colored output to terminal
  --help                Show this help message

${GREEN}Environment Variables:${NC}
  MONITOR_INTERVAL      Override check interval
  MONITOR_LOG_FILE      Override log file path
  MONITOR_DRY_RUN       Set to "true" for dry run mode
  API_PORT              API port (default: $DEFAULT_API_PORT)

${GREEN}Features:${NC}
  - Monitors bot, API, and PostgreSQL containers
  - Checks API health endpoint (/health)
  - Verifies database connectivity
  - Exponential backoff for restart attempts
  - Comprehensive logging with timestamps
  - Configurable check interval
  - Dry-run mode for testing

${GREEN}Examples:${NC}
  # Basic monitoring (30s interval, logs to ./monitor.log)
  ./scripts/monitor-stack.sh

  # Verbose mode with 60s interval
  ./scripts/monitor-stack.sh --interval 60 --verbose

  # Dry run for testing
  ./scripts/monitor-stack.sh --dry-run --verbose

  # Custom log location
  ./scripts/monitor-stack.sh --log /var/log/bot-monitor.log

EOF
}

parse_arguments() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --interval)
                if [[ -z "${2:-}" ]] || ! [[ "$2" =~ ^[0-9]+$ ]]; then
                    log_error "Error: --interval requires a positive number"
                    exit 1
                fi
                CHECK_INTERVAL="$2"
                shift 2
                ;;
            --log)
                if [[ -z "${2:-}" ]]; then
                    log_error "Error: --log requires a filename"
                    exit 1
                fi
                LOG_FILE="$2"
                shift 2
                ;;
            --dry-run)
                DRY_RUN=true
                shift
                ;;
            --verbose)
                VERBOSE=true
                shift
                ;;
            --help)
                show_help
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done
}

setup_logging() {
    # Create log directory if it doesn't exist
    local log_dir
    log_dir=$(dirname "$LOG_FILE")
    if [[ ! -d "$log_dir" ]]; then
        mkdir -p "$log_dir"
    fi
    
    # Create log file if it doesn't exist
    if [[ ! -f "$LOG_FILE" ]]; then
        touch "$LOG_FILE"
    fi
    
    log_info "================================================"
    log_info "Starting Discord Bot Stack Monitor"
    log_info "PID: $MONITOR_PID"
    log_info "Interval: ${CHECK_INTERVAL}s"
    log_info "Log file: $LOG_FILE"
    log_info "Dry run: $DRY_RUN"
    log_info "================================================"
}

cleanup() {
    log_info "Shutting down monitor..."
    exit 0
}

main() {
    # Parse command line arguments
    parse_arguments "$@"
    
    # Override with environment variables if set
    [[ -n "${MONITOR_INTERVAL:-}" ]] && CHECK_INTERVAL="$MONITOR_INTERVAL"
    [[ -n "${MONITOR_LOG_FILE:-}" ]] && LOG_FILE="$MONITOR_LOG_FILE"
    [[ "${MONITOR_DRY_RUN:-}" == "true" ]] && DRY_RUN=true
    [[ -n "${API_PORT:-}" ]] && DEFAULT_API_PORT="$API_PORT"
    
    # Setup logging and signal handlers
    setup_logging
    trap cleanup SIGINT SIGTERM
    
    log_info "Monitor started. Press Ctrl+C to stop."
    
    # Main monitoring loop
    while true; do
        perform_checks
        sleep "$CHECK_INTERVAL"
    done
}

# Run main function with all arguments
main "$@"