#!/usr/bin/env bash
set -e -o pipefail
set +m

DEPENDENCIES=(caddy go node npm setsid pgrep pkill)
SCRIPT_DIR="$(dirname "$(realpath "$0")")"
BACKEND_DIR="${SCRIPT_DIR}/backend"
FRONTEND_DIR="${SCRIPT_DIR}/frontend"
CADDYFILE="${SCRIPT_DIR}/Caddyfile"
BACKEND_PORT=8080
FRONTEND_PORT=8081
PROXY_PORT_FIRST=8000
AIR_BIN=""
BACKEND_PID=""
FRONTEND_PID=""
CADDY_PID=""
SILENT=false
WORKING_DIR="${TMPDIR:-/tmp}/temp-$(basename "$0")-$BASHPID"

# Help message
for ARGUMENT in "$@"; do
    if [[ "$ARGUMENT" == "-h" ]] || [[ "$ARGUMENT" == "--help" ]]; then
        echo "Usage: $(basename "$0") [ARGUMENT...]"
        echo "Runs the local development stack in the foreground: air rebuilds and"
        echo "serves the backend, Vite serves the frontend, and Caddy is the reverse"
        echo "proxy on ports 8000-8003. Press Ctrl+C to stop all services."
        echo "ARGUMENT can be:"
        echo "    --silent Suppresses script status messages, not service logs."
        exit 0
    fi
done

# Write a status message unless --silent is set
function log {
    if [[ "$SILENT" != true ]]; then
        echo "$*" >&2
    fi
}

# Write an error message
function error {
    echo "Error: $*" >&2
}

# Quote a value for a POSIX shell command
function shell_quote {
    local value="$1"
    printf "'%s'" "${value//\'/\'\\\'\'}"
}

# Check arguments
while [[ "$#" -gt 0 ]]; do
    if [[ "$1" == "--silent" ]]; then
        SILENT=true
    else
        echo "Error: Unknown argument: \"$1\"" >&2
        exit 13
    fi
    shift
done

# Check dependencies
for CMD in "${DEPENDENCIES[@]}"; do
    if ! command -v "$CMD" >/dev/null 2>&1; then
        error "\"${CMD}\" is missing!"
        exit 10
    fi
done

AIR_BIN="$(go env GOPATH)/bin/air"
if [[ ! -x "$AIR_BIN" ]]; then
    error "\"air\" is missing at \"${AIR_BIN}\"!"
    error "Install it with: go install github.com/air-verse/air@latest"
    exit 10
fi

# Check required files and directories
if [[ ! -d "$BACKEND_DIR" ]]; then
    error "Backend directory is missing: \"${BACKEND_DIR}\""
    exit 14
fi
if [[ ! -d "$FRONTEND_DIR" ]]; then
    error "Frontend directory is missing: \"${FRONTEND_DIR}\""
    exit 14
fi
if [[ ! -f "$CADDYFILE" ]]; then
    error "Caddyfile is missing: \"${CADDYFILE}\""
    exit 14
fi

# Stop all services by terminating their whole sessions
function stop_services {
    local pid attempt running
    for pid in "$CADDY_PID" "$FRONTEND_PID" "$BACKEND_PID"; do
        if [[ -n "$pid" ]]; then
            pkill --signal TERM --session "$pid" 2>/dev/null || true
        fi
    done
    for attempt in {1..100}; do
        running=false
        for pid in "$CADDY_PID" "$FRONTEND_PID" "$BACKEND_PID"; do
            if [[ -n "$pid" ]] && pgrep --session "$pid" >/dev/null 2>&1; then
                running=true
            fi
        done
        if [[ "$running" == false ]]; then
            break
        fi
        sleep 0.1
    done
    for pid in "$CADDY_PID" "$FRONTEND_PID" "$BACKEND_PID"; do
        if [[ -n "$pid" ]] && pgrep --session "$pid" >/dev/null 2>&1; then
            pkill --signal KILL --session "$pid" 2>/dev/null || true
        fi
    done
    wait 2>/dev/null || true
}

# Clean up everything when the script exits
function cleanup {
    local status=$?
    stop_services
    if [[ -n "$WORKING_DIR" ]]; then
        rm --recursive --force -- "$WORKING_DIR"
    fi
    case "$status" in
        0|1|10|11|13|14|16) ;;
        *) status=1 ;;
    esac
    if [[ "$status" -eq 0 ]]; then
        log "${SCRIPT_NAME} finished."
    else
        error "${SCRIPT_NAME} failed with exit code ${status}."
    fi
    rm -f -r "$WORKING_DIR"
    exit "$status"
}

mkdir "$WORKING_DIR"

# Start the backend with hot reload
function start_backend {
    (
        cd "$BACKEND_DIR"
        export PORT="$BACKEND_PORT"
        exec 9>&-
        exec setsid "$AIR_BIN" \
            --build.cmd "go build -o $WORKING_DIR/subprime ./cmd/subprime" \
            --build.full_bin "${WORKING_DIR}/subprime" \
            --build.entrypoint "${WORKING_DIR}/subprime" \
            --color.mode never
    ) &
    BACKEND_PID=$!
}

# Start the frontend dev server
function start_frontend {
    (
        cd "$FRONTEND_DIR"
        export VITE_API_PROXY="http://127.0.0.1:${PROXY_PORT_FIRST}"
        exec 9>&-
        exec setsid npm run dev -- --host 127.0.0.1
    ) &
    FRONTEND_PID=$!
}

# Start the reverse proxy
function start_reverse_proxy {
    (
        cd "$SCRIPT_DIR"
        export SUBPRIME_BACKEND_URL="127.0.0.1:${BACKEND_PORT}"
        export SUBPRIME_FRONTEND_URL="127.0.0.1:${FRONTEND_PORT}"
        exec 9>&-
        exec setsid caddy run --config "$CADDYFILE" --adapter caddyfile
    ) &
    CADDY_PID=$!
}

# Wait until a service exits on its own
function wait_for_service_exit {
    local status=0
    wait -n || status=$?
    error "A service exited unexpectedly (status ${status}); stopping the rest."
}


trap cleanup EXIT
trap 'exit 0' INT TERM HUP

# Start services
log "Note: Caddy and the backend bind all interfaces; use trusted networks only."
log "Starting backend."
start_backend
log "Starting frontend."
start_frontend
log "Starting reverse proxy."
start_reverse_proxy

log "Development stack is started:"
log "    http://127.0.0.1:8000 Arteon"
log "    http://127.0.0.1:8001 Beetle"
log "    http://127.0.0.1:8002 Challenger"
log "    http://127.0.0.1:8003 Ducato"
log "Press Ctrl+C to stop."

# Supervise services
wait_for_service_exit
exit 1
