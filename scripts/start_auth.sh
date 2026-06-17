#!/bin/bash
set -e

PROJECT_ROOT="/Users/lianlinghao/PycharmProjects/Chant"
AUTH_DIR="$PROJECT_ROOT/services/auth_service"

export PYTHONPATH="$AUTH_DIR:$PROJECT_ROOT:$PROJECT_ROOT/infrastructure_sdk/grpc/token_auth_grpc/proto:$PROJECT_ROOT/infrastructure_sdk/grpc/last_offline_time_grpc/proto"

cd "$AUTH_DIR"

echo "=== Starting auth gRPC servers ==="
python3 -m app.infrastructure.grpc.token_auth_server &
TOKEN_PID=$!
python3 -m app.infrastructure.grpc.last_offline_time_server &
OFFLINE_PID=$!
sleep 1

echo "=== Starting auth HTTP on :9030 ==="
uvicorn app.main:app --host 0.0.0.0 --port 9030 --reload

kill $TOKEN_PID $OFFLINE_PID 2>/dev/null
