# Auth Service

用户认证微服务，提供用户注册、登录等功能。

## 功能特性

- 用户注册和登录
- JWT 令牌认证
- 密码加密存储（bcrypt）
- MongoDB 数据库支持
- 健康检查接口
- API 文档自动生成

## 技术栈

- **框架**: FastAPI
- **数据库**: MongoDB (Motor异步驱动)
- **认证**: JWT + bcrypt
- **部署**: Docker
- **Python版本**: 3.11+

## 快速开始

### 本地开发

1. 安装依赖
```bash
pip install -r requirements.txt
```

2. 配置环境变量
```bash
cp .env.example .env
# 编辑 .env 文件，修改相关配置
```

3. 启动服务
```bash
cd app
python main.py
```

4. 访问API文档
- Swagger UI: http://localhost:9030/docs
- ReDoc: http://localhost:9030/redoc

### Docker部署

1. 构建镜像
```bash
docker build -t auth-service .
```

2. 运行容器
```bash
docker run -p 9030:9030 auth-service
```

## API接口

### 认证相关

- `POST /api/v1/login_or_register` - 用户登录或注册
- `POST /api/v1/login` - 用户登录

### 健康检查

- `GET /api/v1/health` - 服务健康检查
- `GET /api/v1/health/db` - 数据库健康检查

## 项目结构

```
auth_service/
├── app/
│   ├── api/v1/          # API路由
│   ├── core/            # 核心配置
│   ├── database/        # 数据库操作
│   ├── middleware/      # 中间件
│   ├── models/          # 数据模型
│   ├── schemas/         # Pydantic模式
│   ├── services/        # 业务逻辑
│   └── utils/           # 工具函数
├── tests/               # 测试文件
├── Dockerfile          # Docker配置
├── requirements.txt    # Python依赖
└── README.md          # 项目文档
```

## 环境变量

参考 `.env.example` 文件配置以下环境变量：

- `MONGODB_URL`: MongoDB连接字符串
- `DATABASE_NAME`: 数据库名称
- `SECRET_KEY`: JWT密钥
- `PORT`: 服务端口

## 开发指南

### 添加新的API端点

1. 在 `app/api/v1/` 目录下创建新的路由文件
2. 在 `app/main.py` 中注册新的路由
3. 更新相关的模型和服务

### 数据库操作

数据库操作封装在 `app/database/` 目录中，使用 Motor 异步驱动。

### 测试

```bash
pytest tests/
```

## 许可证

MIT License
