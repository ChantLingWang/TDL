# ---------------------------------------------------------
# 第一阶段：构建阶段 (Builder Stage)
# ---------------------------------------------------------
FROM docker.m.daocloud.io/library/python:3.11-slim AS builder

WORKDIR /app

# 设置环境变量
ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

# 更换 apt 源为阿里云源，并安装构建依赖
RUN sed -i 's/deb.debian.org/mirrors.aliyun.com/g' /etc/apt/sources.list.d/debian.sources \
    && apt-get update \
    && apt-get install -y --no-install-recommends gcc python3-dev \
    && rm -rf /var/lib/apt/lists/*

# 创建虚拟环境
RUN python -m venv /opt/venv
# 激活虚拟环境
ENV PATH="/opt/venv/bin:$PATH"

# 复制依赖文件
COPY services/auth_service/requirements.txt .

# 安装依赖，使用清华源加速
RUN pip install --no-cache-dir -r requirements.txt -i https://pypi.tuna.tsinghua.edu.cn/simple

# ---------------------------------------------------------
# 第二阶段：运行阶段 (Runner Stage)
# ---------------------------------------------------------
FROM docker.m.daocloud.io/library/python:3.11-slim AS runner

WORKDIR /app

# 设置环境变量
ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PYTHONPATH=/app:/app/infrastructure_sdk/grpc/token_auth_grpc/proto:/app/infrastructure_sdk/grpc/last_offline_time_grpc/proto \
    PATH="/opt/venv/bin:$PATH"

# 更换 apt 源
RUN sed -i 's/deb.debian.org/mirrors.aliyun.com/g' /etc/apt/sources.list.d/debian.sources

# 从 builder 阶段复制虚拟环境
COPY --from=builder /opt/venv /opt/venv

# 复制应用代码
COPY services/auth_service/app/ ./app/

# Proto 文件（gRPC server 需要）
COPY infrastructure_sdk/grpc/token_auth_grpc/proto/auth_pb2.py /app/infrastructure_sdk/grpc/token_auth_grpc/proto/
COPY infrastructure_sdk/grpc/token_auth_grpc/proto/auth_pb2_grpc.py /app/infrastructure_sdk/grpc/token_auth_grpc/proto/
COPY infrastructure_sdk/grpc/token_auth_grpc/proto/__init__.py /app/infrastructure_sdk/grpc/token_auth_grpc/proto/
COPY infrastructure_sdk/grpc/last_offline_time_grpc/proto/last_offline_time_pb2.py /app/infrastructure_sdk/grpc/last_offline_time_grpc/proto/
COPY infrastructure_sdk/grpc/last_offline_time_grpc/proto/last_offline_time_pb2_grpc.py /app/infrastructure_sdk/grpc/last_offline_time_grpc/proto/
RUN mkdir -p /app/infrastructure_sdk/grpc/token_auth_grpc/proto && mkdir -p /app/infrastructure_sdk/grpc/last_offline_time_grpc/proto && touch /app/infrastructure_sdk/__init__.py /app/infrastructure_sdk/grpc/__init__.py /app/infrastructure_sdk/grpc/token_auth_grpc/__init__.py /app/infrastructure_sdk/grpc/last_offline_time_grpc/__init__.py /app/infrastructure_sdk/grpc/last_offline_time_grpc/proto/__init__.py

# 清除旧的 .pyc 缓存
RUN find /app -name '__pycache__' -type d -exec rm -rf {} + 2>/dev/null; find /app -name '*.pyc' -delete 2>/dev/null; true

# 启动命令
CMD PYTHONPATH=/app:/app/infrastructure_sdk/grpc/token_auth_grpc/proto:/app/infrastructure_sdk/grpc/last_offline_time_grpc/proto sh -c "python3 -m app.infrastructure.grpc.token_auth_server & python3 -m app.infrastructure.grpc.last_offline_time_server & uvicorn app.main:app --host 0.0.0.0 --port 9030"
