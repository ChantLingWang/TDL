# ---------------------------------------------------------
# 第一阶段：构建阶段 (Builder Stage)
# ---------------------------------------------------------
# 使用 golang:1.23-alpine 作为基础镜像进行编译
# "AS builder" 给这个阶段起个别名，方便后面引用
FROM docker.m.daocloud.io/library/golang:1.23-alpine AS builder

# 设置环境变量
# CGO_ENABLED=0: 禁用 CGO，确保生成静态链接的二进制文件，不依赖系统库
# GOOS=linux: 目标操作系统为 Linux
# GOARCH=amd64: 目标架构为 64位 x86
# GOPROXY: 设置 Go 模块代理，加速国内下载速度
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    GOPROXY=https://goproxy.cn,direct

# 设置容器内的工作目录，后续命令都在这个目录下执行
WORKDIR /app

# 1. 先复制依赖描述文件
# 为什么不一次性复制所有文件？
# 因为 Docker 有缓存机制。只要 go.mod 和 go.sum 没变，
# 下一步的 "go mod download" 就会直接使用缓存，大大加快构建速度。
COPY go.mod go.sum ./

# 下载项目依赖
RUN go mod download

# 2. 依赖下载完后，再复制剩下的源代码
COPY . .

# 编译应用
# -ldflags="-w -s": 链接器参数
#   -w: 去掉 DWARF 调试信息
#   -s: 去掉符号表
#   作用：显著减小编译后的二进制文件体积
# -o orchestrator-service: 指定输出文件名为 orchestrator-service
RUN go build -ldflags="-w -s" -o orchestrator-service .

# ---------------------------------------------------------
# 第二阶段：运行阶段 (Run Stage)
# ---------------------------------------------------------
# 使用超轻量级的 alpine 镜像作为运行时环境
# 这样最终的镜像体积会非常小（通常只有几十MB），而不是几百MB
FROM docker.m.daocloud.io/library/alpine:latest

# 安装运行时必要的基础库
# ca-certificates: 支持 HTTPS 请求（访问外部 API 时需要）
# tzdata: 时区数据（Go 程序处理时间时需要）
# --no-cache: 不缓存安装包，减少镜像体积
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories && \
    apk --no-cache add ca-certificates tzdata

# 设置时区为上海时间
ENV TZ=Asia/Shanghai

# 设置工作目录
WORKDIR /app

# 从构建阶段 (builder) 复制编译好的二进制文件到当前镜像
COPY --from=builder /app/orchestrator-service .

# 从构建阶段复制模板文件目录
# 注意：保持目录结构与代码中读取的路径一致
COPY --from=builder /app/templates ./templates

# 容器启动时默认执行的命令
CMD ["./orchestrator-service"]
