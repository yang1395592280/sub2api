#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
ENV_FILE="${ENV_FILE:-${REPO_ROOT}/.env.run.local}"

BACKEND_PID_FILE="/tmp/sub2api-local.pid"
FRONTEND_PID_FILE="/tmp/sub2api-frontend.pid"
BACKEND_BIN="/tmp/sub2api-server-local"
BACKEND_LOG="/tmp/sub2api-local.log"
FRONTEND_LOG="/tmp/sub2api-frontend.log"

BACKEND_URL="${BACKEND_URL:-http://127.0.0.1:8080/health}"
FRONTEND_URL="${FRONTEND_URL:-http://127.0.0.1:3000}"
GO_BUILD_CACHE_DIR="${GO_BUILD_CACHE_DIR:-/tmp/go-build}"
BACKEND_READY_ATTEMPTS="${BACKEND_READY_ATTEMPTS:-300}"
FRONTEND_READY_ATTEMPTS="${FRONTEND_READY_ATTEMPTS:-40}"

ensure_env_file() {
    if [ ! -f "$ENV_FILE" ]; then
        echo "Missing env file: $ENV_FILE" >&2
        echo "Create it first, or copy from .env.run.local.example if you add one later." >&2
        exit 1
    fi
}

load_env() {
    ensure_env_file
    set -a
    source "$ENV_FILE"
    set +a
}

get_go_bin() {
    if [ -x /tmp/go-toolchains/go1.26.2/bin/go ]; then
        echo "/tmp/go-toolchains/go1.26.2/bin/go"
        return 0
    fi
    command -v go
}

kill_pid_file() {
    local pid_file="$1"
    if [ -f "$pid_file" ]; then
        local pid
        pid=$(cat "$pid_file")
        if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
            kill "$pid" 2>/dev/null || true
        fi
        rm -f "$pid_file"
    fi
}

wait_for_url() {
    local url="$1"
    local label="$2"
    local attempts="${3:-40}"

    for ((i=1; i<=attempts; i++)); do
        if curl -fsS "$url" >/dev/null 2>&1; then
            echo "$label is ready: $url"
            return 0
        fi
        sleep 1
    done

    echo "$label failed to become ready: $url" >&2
    if [ "$label" = "backend" ] && [ -f "$BACKEND_LOG" ]; then
        echo "--- backend log tail ---" >&2
        tail -n 80 "$BACKEND_LOG" >&2 || true
        echo "--- end backend log tail ---" >&2
    fi
    if [ "$label" = "frontend" ] && [ -f "$FRONTEND_LOG" ]; then
        echo "--- frontend log tail ---" >&2
        tail -n 80 "$FRONTEND_LOG" >&2 || true
        echo "--- end frontend log tail ---" >&2
    fi
    return 1
}

start_backend() {
    load_env
    mkdir -p "${DATA_DIR:?DATA_DIR is required}"
    mkdir -p "${GO_BUILD_CACHE_DIR}"

    local go_bin
    go_bin="$(get_go_bin)"

    (
        cd "$REPO_ROOT/backend"
        export GOCACHE="$GO_BUILD_CACHE_DIR"
        "$go_bin" build -o "$BACKEND_BIN" ./cmd/server
    )

    kill_pid_file "$BACKEND_PID_FILE"
    : > "$BACKEND_LOG"

    (
        set -a
        source "$ENV_FILE"
        set +a
        nohup "$BACKEND_BIN" >"$BACKEND_LOG" 2>&1 &
        echo $! > "$BACKEND_PID_FILE"
    )

    wait_for_url "$BACKEND_URL" "backend" "$BACKEND_READY_ATTEMPTS"
}

start_frontend() {
    if [ ! -d "$REPO_ROOT/frontend/node_modules" ]; then
        echo "frontend/node_modules not found. Run: cd frontend && npm install" >&2
        exit 1
    fi

    kill_pid_file "$FRONTEND_PID_FILE"
    : > "$FRONTEND_LOG"

    (
        cd "$REPO_ROOT/frontend"
        nohup ./node_modules/.bin/vite --host 127.0.0.1 --port 3000 >"$FRONTEND_LOG" 2>&1 &
        echo $! > "$FRONTEND_PID_FILE"
    )

    wait_for_url "$FRONTEND_URL" "frontend" "$FRONTEND_READY_ATTEMPTS"
}

stop_all() {
    kill_pid_file "$BACKEND_PID_FILE"
    kill_pid_file "$FRONTEND_PID_FILE"
    echo "local frontend/backend stopped"
}

print_status() {
    echo "env file: $ENV_FILE"
    if [ -f "$BACKEND_PID_FILE" ] && kill -0 "$(cat "$BACKEND_PID_FILE")" 2>/dev/null; then
        echo "backend: running (pid $(cat "$BACKEND_PID_FILE"))"
    else
        echo "backend: stopped"
    fi

    if [ -f "$FRONTEND_PID_FILE" ] && kill -0 "$(cat "$FRONTEND_PID_FILE")" 2>/dev/null; then
        echo "frontend: running (pid $(cat "$FRONTEND_PID_FILE"))"
    else
        echo "frontend: stopped"
    fi

    echo "backend url: $BACKEND_URL"
    echo "frontend url: $FRONTEND_URL"
    echo "backend log: $BACKEND_LOG"
    echo "frontend log: $FRONTEND_LOG"
}

start_all() {
    start_backend
    start_frontend
    echo ""
    echo "frontend: http://127.0.0.1:3000/"
    echo "backend health: $BACKEND_URL"
}

main() {
    case "${1:-start}" in
        start)
            start_all
            ;;
        stop)
            stop_all
            ;;
        restart)
            stop_all
            start_all
            ;;
        status)
            print_status
            ;;
        *)
            echo "Usage: $0 [start|stop|restart|status]" >&2
            exit 1
            ;;
    esac
}

main "$@"
