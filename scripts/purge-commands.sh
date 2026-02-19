#!/bin/bash
# purge-commands.sh - Wrapper script for Discord slash command purging with env file support
#
# Usage: ./scripts/purge-commands.sh [--env-file FILE] [purge-command-options]
#
# This script loads environment variables from a file (default: .env.dev)
# and runs the purge-commands binary with the specified options.
#
# Examples:
#   ./scripts/purge-commands.sh                     # Purge dev guild commands
#   ./scripts/purge-commands.sh --dry-run           # Dry-run for dev guild
#   ./scripts/purge-commands.sh --global            # Purge global commands
#   ./scripts/purge-commands.sh --guild 1234567890  # Purge from specific guild
#   ./scripts/purge-commands.sh --all --force       # Purge all commands
#   ./scripts/purge-commands.sh --env-file .env.prod --global  # Custom env file

set -euo pipefail

# Default environment file
ENV_FILE=".env.dev"
SCRIPT_ARGS=()
PURGE_ARGS=()

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_color() {
    local color=$1
    local message=$2
    echo -e "${color}${message}${NC}"
}

# Function to show help
show_help() {
    cat << EOF
${BLUE}Discord Command Purge Wrapper Script${NC}

${GREEN}Usage:${NC}
  ./scripts/purge-commands.sh [--env-file FILE] [purge-command-options]

${GREEN}Script-specific options:${NC}
  --env-file FILE    Load environment variables from FILE (default: .env.dev)
  --help             Show this help message

${GREEN}Purge command options (passed to purge-commands binary):${NC}
  --guild ID         Guild ID to purge commands from
  --global           Purge global commands
  --all              Purge all commands (global + all guilds)
  --dry-run          Show what would be deleted without actually deleting
  --force            Skip confirmation prompts
  --help             Show purge-commands help

${GREEN}Behavior:${NC}
  - If --guild, --global, or --all is specified, uses that mode
  - If no mode specified, uses DISCORD_DEV_GUILD_ID from environment file
  - All arguments except --env-file are passed to purge-commands

${GREEN}Examples:${NC}
  # Purge from dev guild (using DISCORD_DEV_GUILD_ID from .env.dev)
  ./scripts/purge-commands.sh

  # Dry-run for dev guild
  ./scripts/purge-commands.sh --dry-run

  # Purge global commands
  ./scripts/purge-commands.sh --global

  # Purge from specific guild (overrides env file)
  ./scripts/purge-commands.sh --guild 123456789012345678 --force

  # Purge all commands with custom env file
  ./scripts/purge-commands.sh --all --env-file .env.prod --force

  # Show purge-commands help
  ./scripts/purge-commands.sh --help
EOF
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --env-file)
            if [[ -z "${2:-}" ]]; then
                print_color "$RED" "Error: --env-file requires a filename"
                exit 1
            fi
            ENV_FILE="$2"
            shift 2
            ;;
        --help)
            show_help
            exit 0
            ;;
        *)
            # All other arguments go to purge-commands
            PURGE_ARGS+=("$1")
            shift
            ;;
    esac
done

# Check if environment file exists
if [[ ! -f "$ENV_FILE" ]]; then
    print_color "$RED" "Error: Environment file '$ENV_FILE' not found"
    print_color "$YELLOW" "Please create $ENV_FILE or specify a different file with --env-file"
    exit 1
fi

# Load environment variables from file
print_color "$BLUE" "Loading environment variables from: $ENV_FILE"
# Parse .env file line by line, handling special characters
while IFS='=' read -r key value || [[ -n "$key" ]]; do
    # Skip empty lines and comments
    [[ -z "$key" ]] && continue
    [[ "$key" =~ ^[[:space:]]*# ]] && continue
    
    # Remove leading/trailing whitespace from key and value
    key=$(echo "$key" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
    value=$(echo "$value" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
    
    # Remove quotes if present
    value=$(echo "$value" | sed -e 's/^"//' -e 's/"$//' -e "s/^'//" -e "s/'$//")
    
    # Export the variable
    export "$key"="$value"
done < "$ENV_FILE"

# Check if we have a mode specified
has_mode=false
for arg in "${PURGE_ARGS[@]}"; do
    case $arg in
        --guild|--global|--all)
            has_mode=true
            ;;
    esac
done

# If no mode specified, use DISCORD_DEV_GUILD_ID from env
if [[ "$has_mode" == false ]]; then
    if [[ -z "${DISCORD_DEV_GUILD_ID:-}" ]]; then
        print_color "$RED" "Error: No purge mode specified and DISCORD_DEV_GUILD_ID not found in $ENV_FILE"
        print_color "$YELLOW" "Please specify --guild, --global, or --all, or set DISCORD_DEV_GUILD_ID in $ENV_FILE"
        exit 1
    fi
    print_color "$BLUE" "Using dev guild ID from $ENV_FILE: $DISCORD_DEV_GUILD_ID"
    PURGE_ARGS+=("--guild" "$DISCORD_DEV_GUILD_ID")
fi

# Check for required Discord variables
if [[ -z "${DISCORD_TOKEN:-}" ]]; then
    print_color "$RED" "Error: DISCORD_TOKEN not found in $ENV_FILE"
    exit 1
fi

if [[ -z "${DISCORD_APP_ID:-}" ]]; then
    print_color "$RED" "Error: DISCORD_APP_ID not found in $ENV_FILE"
    exit 1
fi

# Build and run the command
CMD="go run ./cmd/purge-commands ${PURGE_ARGS[*]}"
print_color "$GREEN" "Running: $CMD"
echo

# Execute the command
eval "$CMD"