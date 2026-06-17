#!/bin/bash
echo "=== 1. Check if chat-service is running ==="
curl -s -o /dev/null -w "chat-service HTTP: %{http_code}\n" http://localhost:8080/api/v1/groups 2>&1 || echo "chat-service HTTP: UNREACHABLE"

echo ""
echo "=== 2. Check if auth gRPC is running ==="
python3 -c "
import grpc, sys, os
sys.path.insert(0, 'infrastructure_sdk/grpc/token_auth_grpc/proto')
sys.path.insert(0, '.')
from auth_pb2 import VerifyTokenRequest
from auth_pb2_grpc import AuthServiceStub
channel = grpc.insecure_channel('localhost:50051')
stub = AuthServiceStub(channel)
try:
    resp = stub.VerifyToken(VerifyTokenRequest(token='test'), timeout=2)
    print('gRPC VerifyToken: reachable (response valid=%s)' % resp.valid)
except Exception as e:
    print('gRPC VerifyToken: FAILED - %s' % e)
" 2>&1

echo ""
echo "=== 3. Test WS upgrade (via curl) ==="
curl -s -o /dev/null -w "WS upgrade response: %{http_code}\n" -H "Connection: Upgrade" -H "Upgrade: websocket" -H "Sec-WebSocket-Version: 13" -H "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==" "http://localhost:8080/api/v1/ws?token=test" 2>&1 || echo "WS upgrade: UNREACHABLE"

echo ""
echo "=== 4. Check if chat-service-native binary exists ==="
ls -la services/chat_service/chat-service-native 2>&1
