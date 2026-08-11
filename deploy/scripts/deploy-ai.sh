#!/bin/bash
# S3 (ai 节点) 部署脚本：git pull -> 重启 ai
set -e

cd /opt/chant/repo
for i in 1 2 3; do
    git pull --ff-only && break
    echo "git pull failed (attempt $i), retrying..."
    sleep 3
done

cd services/ai_service
/opt/chant/ai-venv/bin/python -m pip install -q -r requirements.txt -i https://pypi.tuna.tsinghua.edu.cn/simple

systemctl restart chant-ai
systemctl status chant-ai --no-pager | head -5
echo "ai deploy done"
