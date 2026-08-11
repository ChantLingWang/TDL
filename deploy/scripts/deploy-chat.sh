#!/bin/bash
# S1 (chat 节点) 部署脚本：本地提交 -> 服务器 git pull -> 编译 -> 重启
set -e

cd /opt/chant/repo
git pull --ff-only

cd services/chat_service
/usr/local/go/bin/go build -ldflags="-w -s" -o /opt/chant/chat-service .

systemctl restart chant-chat
systemctl status chant-chat --no-pager | head -5
