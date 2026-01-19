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
COPY requirements.txt .

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
    PYTHONPATH=/app \
    PATH="/opt/venv/bin:$PATH"

# 更换 apt 源
RUN sed -i 's/deb.debian.org/mirrors.aliyun.com/g' /etc/apt/sources.list.d/debian.sources

# 从 builder 阶段复制虚拟环境
COPY --from=builder /opt/venv /opt/venv

# 复制应用代码
COPY app/ ./app/

# 启动命令
CMD ["python", "-m", "app.main"]
