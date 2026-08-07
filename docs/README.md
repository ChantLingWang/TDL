# Chant 链路文档

本目录持久化记录 Chant 的消息链路、字段定义与跨服务契约，方便后续开发与排查。

## 文档列表

- [message-chain.md](./message-chain.md) —— 聊天 / AI 消息完整链路与字段传递说明
- [storage-architecture.md](./storage-architecture.md) —— 存储架构（Kafka / MongoDB / PostgreSQL / Redis / Qdrant）

## 配置文件布局

| 位置 | 内容 |
| --- | --- |
| `docker-compose.yml` | 开发全量编排（基础设施 + auth + chat + orchestrator） |
| `docker-compose.prod.yml` | 生产编排（含 ai-service，资源限制） |
| `.env.example` | compose 生产环境变量示例 |
| `services/chat_service/config.yaml` | chat_service 默认配置（localhost 开发） |
| `services/orchestrator_service/config/config.yaml` | orchestrator 默认配置 |
| `services/auth_service/.env.example` | auth_service 环境变量示例 |
| `services/ai_service/.env.example` | ai_service 环境变量示例 |
| `front_code/.env.example` | 前端环境变量示例 |
| `searxng/` | SearXNG 配置 |
| `deploy/nginx/` | nginx 单机/多节点配置 |

## 快速总览

```
前端 WebSocket
   │  发送 {type:"chat", content:{...}}
   ▼
chat_service（落库 MongoDB + 发 Kafka）
   │  EventEnvelope{ MessageSent }
   ▼
Kafka topic: chat_group_message
   │
   ├──► ai_service（chat / agent 模式）──► DeepSeek / Qdrant / SearXNG
   │         │  AiReplyGenerated 事件
   │         ▼
   │   Kafka topic: chat_group_message
   │         │
   │         ▼
   └──► chat_service（落库 MongoDB + WebSocket 推送）
```

详细字段见 [message-chain.md](./message-chain.md)。
