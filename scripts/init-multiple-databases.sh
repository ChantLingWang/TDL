#!/bin/bash
set -e

# orchestrator数据库已通过POSTGRES_DB环境变量自动创建

# 创建 user_service 数据库（如果不存在）
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    CREATE DATABASE IF NOT EXISTS user_service;
    GRANT ALL PRIVILEGES ON DATABASE user_service TO $POSTGRES_USER;
EOSQL