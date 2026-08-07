#!/bin/bash
set -e

# orchestrator 数据库已通过 POSTGRES_DB 环境变量自动创建

# 创建附加数据库（已存在时忽略错误，保证幂等）
for db in ai_audit user_service; do
    psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
        -c "CREATE DATABASE $db" 2>/dev/null || true
    psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
        -c "GRANT ALL PRIVILEGES ON DATABASE $db TO $POSTGRES_USER" 2>/dev/null || true
done
