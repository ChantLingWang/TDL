#!/bin/bash
# S3 (ai 节点) 部署脚本：git pull -> 重启 ai
set -e

cd /opt/chant/repo
git pull --ff-only

systemctl restart chant-ai
systemctl status chant-ai --no-pager | head -5
echo "ai deploy done"
