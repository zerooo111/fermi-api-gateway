#!/bin/bash

# Fermi API Gateway - Pretty Log Viewer
# Usage: ./view-logs.sh [options]

SERVICE_NAME="fermi-gateway"
COLORS=true

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
GRAY='\033[0;90m'
NC='\033[0m' # No Color

# Parse arguments
FOLLOW=false
LINES=50
FILTER_ERRORS=false
FILTER_INFO=false
NO_COLOR=false

while [[ $# -gt 0 ]]; do
    case $1 in
        -f|--follow)
            FOLLOW=true
            shift
            ;;
        -n|--lines)
            LINES="$2"
            shift 2
            ;;
        -e|--errors)
            FILTER_ERRORS=true
            shift
            ;;
        -i|--info)
            FILTER_INFO=true
            shift
            ;;
        --no-color)
            NO_COLOR=true
            COLORS=false
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [options]"
            echo ""
            echo "Options:"
            echo "  -f, --follow       Follow log output (like tail -f)"
            echo "  -n, --lines N      Show last N lines (default: 50)"
            echo "  -e, --errors       Show only errors and warnings"
            echo "  -i, --info         Show only info and above (exclude debug)"
            echo "  --no-color         Disable color output"
            echo "  -h, --help         Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0                  # Show last 50 lines"
            echo "  $0 -f               # Follow logs"
            echo "  $0 -n 100 -e        # Last 100 lines, errors only"
            echo "  $0 --since '5 minutes ago'"
            exit 0
            ;;
        --since)
            SINCE="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Function to format log lines
format_log() {
    local line="$1"
    
    if [ "$NO_COLOR" = true ]; then
        echo "$line"
        return
    fi
    
    # Extract timestamp
    if [[ $line =~ ^[A-Za-z]{3}\ [0-9]{1,2}\ [0-9]{2}:[0-9]{2}:[0-9]{2} ]]; then
        timestamp=$(echo "$line" | grep -oE "^[A-Za-z]{3}\ [0-9]{1,2}\ [0-9]{2}:[0-9]{2}:[0-9]{2}")
        rest=$(echo "$line" | sed "s/^$timestamp //")
        
        # Color timestamp
        echo -e "${GRAY}${timestamp}${NC} ${rest}"
        return
    fi
    
    # Check for log level
    if [[ $line =~ (INFO|WARN|ERROR|FATAL|DEBUG) ]]; then
        if [[ $line =~ ERROR|FATAL ]]; then
            echo -e "${RED}${line}${NC}"
        elif [[ $line =~ WARN ]]; then
            echo -e "${YELLOW}${line}${NC}"
        elif [[ $line =~ INFO ]]; then
            echo -e "${GREEN}${line}${NC}"
        elif [[ $line =~ DEBUG ]]; then
            echo -e "${BLUE}${line}${NC}"
        else
            echo "$line"
        fi
        return
    fi
    
    # Format JSON logs (if they exist in the line)
    if [[ $line =~ \{[^}]*\} ]]; then
        # Try to extract and format JSON
        json_part=$(echo "$line" | grep -oE '\{[^}]*\}' | head -1)
        if command -v jq &> /dev/null && [[ -n "$json_part" ]]; then
            # Format JSON nicely
            formatted_json=$(echo "$json_part" | jq -c '.' 2>/dev/null)
            if [[ -n "$formatted_json" ]]; then
                # Replace JSON in line with formatted version
                before_json=$(echo "$line" | sed "s/{.*}//" | sed 's/[[:space:]]*$//')
                echo -e "${CYAN}${before_json}${NC}"
                echo "$formatted_json" | jq '.'
                return
            fi
        fi
    fi
    
    # Stack trace lines (indented)
    if [[ $line =~ ^[[:space:]]+ ]]; then
        echo -e "${GRAY}${line}${NC}"
        return
    fi
    
    # HTTP request logs
    if [[ $line =~ "HTTP request" ]]; then
        # Extract method, path, status, duration
        method=$(echo "$line" | grep -oE '"method":\s*"[^"]*"' | cut -d'"' -f4 || echo "")
        path=$(echo "$line" | grep -oE '"path":\s*"[^"]*"' | cut -d'"' -f4 || echo "")
        status=$(echo "$line" | grep -oE '"status":\s*[0-9]+' | grep -oE '[0-9]+' || echo "")
        duration=$(echo "$line" | grep -oE '"duration":\s*"[^"]*"' | cut -d'"' -f4 || echo "")
        
        if [[ -n "$status" ]]; then
            # Color status code
            if [[ $status -ge 500 ]]; then
                status_color="${RED}"
            elif [[ $status -ge 400 ]]; then
                status_color="${YELLOW}"
            else
                status_color="${GREEN}"
            fi
            
            echo -e "${BLUE}HTTP${NC} ${method} ${CYAN}${path}${NC} ${status_color}${status}${NC} ${GRAY}${duration}${NC}"
            return
        fi
    fi
    
    # Default output
    echo "$line"
}

# Build journalctl command
JOURNALCTL_CMD="journalctl -u $SERVICE_NAME"

if [[ -n "$SINCE" ]]; then
    JOURNALCTL_CMD="$JOURNALCTL_CMD --since \"$SINCE\""
elif [[ "$FOLLOW" = false ]]; then
    JOURNALCTL_CMD="$JOURNALCTL_CMD -n $LINES"
fi

if [[ "$FOLLOW" = true ]]; then
    JOURNALCTL_CMD="$JOURNALCTL_CMD -f"
fi

JOURNALCTL_CMD="$JOURNALCTL_CMD --no-pager"

# Filter options
if [[ "$FILTER_ERRORS" = true ]]; then
    JOURNALCTL_CMD="$JOURNALCTL_CMD | grep -E '(ERROR|WARN|FATAL)'"
fi

# Execute and format
if [[ "$FOLLOW" = true ]]; then
    # Follow mode - format as we go
    eval "$JOURNALCTL_CMD" | while IFS= read -r line; do
        format_log "$line"
    done
else
    # Non-follow mode - format all at once
    eval "$JOURNALCTL_CMD" | while IFS= read -r line; do
        format_log "$line"
    done
fi

