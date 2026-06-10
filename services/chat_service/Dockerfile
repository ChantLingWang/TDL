# ---------------------------------------------------------
# 第一阶段：构建
# ---------------------------------------------------------
FROM docker.m.daocloud.io/library/golang:1.26-alpine AS builder

ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    GOPROXY=https://goproxy.cn,direct

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -ldflags="-w -s" -o chat-service .

# ---------------------------------------------------------
# 第二阶段：运行
# ---------------------------------------------------------
FROM docker.m.daocloud.io/library/alpine:latest

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories && \
    apk --no-cache add ca-certificates tzdata

ENV TZ=Asia/Shanghai

WORKDIR /app

COPY --from=builder /app/chat-service .
COPY --from=builder /app/config.yaml .

CMD ["./chat-service"]
