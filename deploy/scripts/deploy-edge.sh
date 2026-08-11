#!/bin/bash
# S2 (edge / auth 节点) 部署脚本：git pull -> 重启 auth -> 构建前端 -> 刷新 nginx
set -e

cd /opt/chant/repo
for i in 1 2 3; do
    git pull --ff-only && break
    echo "git pull failed (attempt $i), retrying..."
    sleep 3
done

export PATH=/opt/node/bin:$PATH

cd services/auth_service
/opt/chant/auth-venv/bin/python -m pip install -q -r requirements.txt -i https://pypi.tuna.tsinghua.edu.cn/simple
sudo systemctl restart chant-auth

cd /opt/chant/repo/front_code
npm ci || npm install
VITE_WS_URL="${VITE_WS_URL:-ws://1.12.248.26/api/v1/ws}" npm run build
rsync -a --delete dist/ /opt/chant/frontend/

sudo systemctl reload nginx
echo "edge deploy done"
