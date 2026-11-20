# Chant 链路文档

本目录持久化记录 Chant 的消息链路、字段定义与跨服务契约，方便后续开发与排查。

## 文档列表

- [message-chain.md](./message-chain.md) —— 消息链路与字段传递（当前实际运行版本）
- [storage-architecture.md](./storage-architecture.md) —— 存储现状：Kafka / MongoDB / PostgreSQL / Redis，以及尚未实现的聊天消息存储
- [proto-turn-fields.md](./proto-turn-fields.md) —— round / turn_id / step / attempt / status 字段与幂等/有序设计（草案）

## 当前服务

| 服务 | 职责 |
| --- | --- |
| `chat_service` | AI-only gateway：WebSocket 收发 + Kafka 转发，不落库 |
| `ai_service` | 消费 `MessageSent`，跑 AgentLoop（工具调用），把回复发回 Kafka |
| `auth_service` | 注册/登录/Token，MongoDB + Redis |
| `orchestrator_service` | Saga 编排，PostgreSQL |

## 配置文件布局

| 位置 | 内容 |
| --- | --- |
| `docker-compose.yml` | 开发全量编排（基础设施 + auth + gateway + orchestrator） |
| `docker-compose.prod.yml` | 生产编排（含 ai-service，资源限制） |
| `docker-compose.ai.yml` | 独立 AI 节点 |
| `docker-compose.chat.yml` | 独立 gateway 节点 |
| `docker-compose.edge.yml` | 独立 auth/nginx 节点 |
| `.env.example` | compose 生产环境变量示例 |
| `services/chat_service/config.yaml` | gateway 默认配置 |
| `services/orchestrator_service/config/config.yaml` | orchestrator 默认配置 |
| `services/auth_service/.env.example` | auth_service 环境变量示例 |
| `services/ai_service/.env.example` | ai_service 环境变量示例 |
| `services/ai_service/llm_relay/config/config.json` | LLM provider / model / routing / 价格 |
| `front_code/.env.example` | 前端环境变量示例 |
| `deploy/nginx/` | nginx 单机/多节点配置 |

## 快速总览

```
前端 WebSocket
   │  发送 {type:"chat", content:{conversation_id, text, message_id, message_type}}
   ▼
chat gateway（鉴权 + WS + Kafka，不落库）
   │  EventEnvelope{ MessageSent }（key = conversation_id）
   ▼
Kafka topic: chat_group_message
   │
   ├──► ai_service（AgentLoop + llm_tools）──► DeepSeek
   │         │  AiReplyDelta / AiReplyGenerated（key = target_user_id）
   │         ▼
   │   Kafka topic: chat_group_message
   │         │
   │         └──► chat gateway（WebSocket 推送，不落库）
```

> **注意**：当前没有 storage 消费者，用户消息和 AI 回复都不会落库；
> 刷新页面后消息丢失。详细现状见 [storage-architecture.md](./storage-architecture.md)。

字段与事件定义见 [message-chain.md](./message-chain.md)。
