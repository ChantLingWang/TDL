#!/bin/bash
# S1 (chat 节点) 部署脚本：本地提交 -> 服务器 git pull -> 编译 -> 重启
set -e

cd /opt/chant/repo
for i in 1 2 3; do
    git pull --ff-only && break
    echo "git pull failed (attempt $i), retrying..."
    sleep 3
done

export GOPROXY=https://goproxy.cn,direct
# 2G 内存机器：串行编译 + 限制并行度，避免 go build 内存尖峰
export GOFLAGS=-p=1
export GOMAXPROCS=2

cd services/chat_service
/usr/local/go/bin/go build -ldflags="-w -s" -o /opt/chant/chat-service .

systemctl restart chant-chat
systemctl status chant-chat --no-pager | head -5
