#!/bin/bash

# Ensure script runs from its directory
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$DIR"

SERVER_NAME="api-rag-demo-server"

PID=$(pgrep -f "./${SERVER_NAME}" || true)
if [ -n "$PID" ]; then
    echo "=== Stopping ${SERVER_NAME} (PID: ${PID}) ==="
    kill ${PID}
    for i in {1..10}; do
        if ! kill -0 ${PID} 2>/dev/null; then
            echo "${SERVER_NAME} stopped gracefully."
            break
        fi
        sleep 1
    done
    if kill -0 ${PID} 2>/dev/null; then
        echo "${SERVER_NAME} did not stop gracefully, force killing (SIGKILL)..."
        kill -9 ${PID}
    fi
else
    echo "${SERVER_NAME} process is not running."
fi

echo "=== Stopping Infrastructure Services (Docker Compose) ==="
#if command -v docker-compose >/dev/null 2>&1; then
#    docker-compose down
#elif docker compose version >/dev/null 2>&1; then
#    docker compose down
#fi

echo "=== System Stopped ==="
