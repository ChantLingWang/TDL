# nginx 部署说明

## 架构

```
浏览器 --HTTPS--> nginx (:443)
                   ├── /api/v1/auth/*  -->  auth_service  (:9030, HTTP)
                   ├── /api/v1/*       -->  chat_service  (:8080, HTTP)
                   ├── /api/v1/ws      -->  chat_service  (:8080, WebSocket)
                   └── 其余路径         -->  front_code/dist  静态文件
```

## 服务器上部署

```bash
# 1. 安装 nginx
sudo apt install -y nginx

# 2. 生成自签名证书（测试用，浏览器会提示不安全，忽略即可）
sudo mkdir -p /etc/nginx/ssl
sudo openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout /etc/nginx/ssl/chant.key \
  -out /etc/nginx/ssl/chant.crt \
  -subj "/CN=1.12.248.26"

# 3. 部署配置文件
sudo cp deploy/nginx/chant.conf /etc/nginx/sites-available/chant
sudo ln -s /etc/nginx/sites-available/chant /etc/nginx/sites-enabled/
sudo rm /etc/nginx/sites-enabled/default

# 4. 构建并部署前端
cd front_code && npm run build
sudo mkdir -p /home/ubuntu/front_code/dist
sudo cp -r dist/* /home/ubuntu/front_code/dist/

# 5. 测试配置并重载
sudo nginx -t
sudo systemctl reload nginx
```

## 正式上线替换 Let's Encrypt

```bash
sudo apt install -y certbot python3-certbot-nginx
sudo certbot --nginx -d your-domain.com
```

## 前端 baseURL 改动

nginx 部署后前端 API 全部改为**相对路径**：

```typescript
// front_code/src/utils/request.ts
baseURL: '/api/v1'

// front_code/src/api/chat.ts
baseURL: '/api/v1'

// WebSocket
const ws = new WebSocket(`wss://${location.host}/api/v1/ws?token=...`);
```
