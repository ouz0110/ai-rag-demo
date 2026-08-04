#!/bin/bash

# Ensure script runs from its directory
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$DIR"

echo "=== Starting Infrastructure Services (Redis, MySQL, Nginx) ==="
if command -v docker-compose >/dev/null 2>&1; then
    docker-compose up -d
elif docker compose version >/dev/null 2>&1; then
    docker compose up -d
else
    echo "Warning: docker-compose not found. Please start containers manually."
fi

mkdir -p log

SERVER_NAME="api-rag-demo-server"
CONFIG_FILE="configs/config.local.yaml"

PID=$(pgrep -f "./${SERVER_NAME}" || true)
if [ -n "$PID" ]; then
    echo "${SERVER_NAME} is already running (PID: ${PID})."
    exit 0
fi

echo "=== Starting ${SERVER_NAME} in Background ==="
if [ ! -f "./${SERVER_NAME}" ]; then
    echo "Error: Binary ./${SERVER_NAME} not found!"
    exit 1
fi

chmod +x "./${SERVER_NAME}"
nohup "./${SERVER_NAME}" -conf="${CONFIG_FILE}" > log/server.log 2>&1 &

sleep 1
NEW_PID=$(pgrep -f "./${SERVER_NAME}" || true)
if [ -n "$NEW_PID" ]; then
    echo "${SERVER_NAME} started successfully (PID: ${NEW_PID})."
    echo "Logs are available at log/server.log"
else
    echo "Failed to start ${SERVER_NAME}. Check log/server.log for details."
    exit 1
fi
