FROM golang:1.21-alpine

WORKDIR /app

# 安装构建依赖
RUN apk add --no-cache gcc musl-dev

# 复制go.mod和go.sum
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN go build -o main ./app

EXPOSE 8080

CMD ["./main"]