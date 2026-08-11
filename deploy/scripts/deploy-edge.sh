#!/bin/bash
# S2 (edge / auth 节点) 部署脚本：git pull -> 重启 auth -> 构建前端 -> 刷新 nginx
set -e

cd /opt/chant/repo
git pull --ff-only

export PATH=/opt/node/bin:$PATH

cd services/auth_service
/opt/chant/auth-venv/bin/pip install -q -r requirements.txt -i https://pypi.tuna.tsinghua.edu.cn/simple
systemctl restart chant-auth

cd /opt/chant/repo/front_code
npm ci || npm install
VITE_WS_URL="${VITE_WS_URL:-ws://1.12.248.26/api/v1/ws}" npm run build
rsync -a --delete dist/ /opt/chant/frontend/

systemctl reload nginx
echo "edge deploy done"
