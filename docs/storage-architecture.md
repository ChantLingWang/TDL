# 存储架构文档

> 2026-11 重构后：`chat_service` 改为 AI-only gateway，**不再落库**；
> 私聊/群聊的 Mongo/Postgres/Redis 代码已删除；AI 长期记忆（Qdrant）已下线。
>
> **聊天消息、会话列表、历史记录目前都没有存储**。本文只描述当前代码真实使用的存储，
> 未实现的部分集中在第 6 节。

## 1. 总览

| 存储 | 当前用途 | 谁在写 | 谁在读 |
| --- | --- | --- | --- |
| Kafka | 事件总线（不落业务状态） | gateway / ai_service / orchestrator | gateway / ai_service / orchestrator |
| MongoDB | 用户、Token、离线时间 | auth_service | auth_service / gateway（经 gRPC） |
| PostgreSQL | Saga 状态、LLM 成本 | orchestrator / ai_service | orchestrator / ai_service |
| Redis | 邮箱验证码等短生命周期数据 | auth_service | auth_service |

已下线：

- Qdrant 向量库与 embedding/rerank 调用；
- chat_service 的 Mongo / PostgreSQL / Redis 依赖与对应表。

## 2. Kafka

| Topic | 事件格式 | 用途 |
| --- | --- | --- |
| `chat_group_message` | Proto `EventEnvelope`（protojson） | 聊天消息与 AI 回复 |
| `saga-events` | JSON `BusinessEvent` | Saga 编排 |
| `saga-dlq` | JSON DLQ 载荷 | Saga 死信队列 |

`chat_group_message` 的 key：

- `MessageSent`：`conversation_id`
- `AiTurnStep`：`conversation_id`

## 3. MongoDB

数据库 `auth`：

| 集合 | 内容 | 写入方 |
| --- | --- | --- |
| `Users` | 用户资料、`last_offline_time` | auth_service |
| `UserTokens` | refresh token 及有效期 | auth_service |

`chat` 库当前没有代码写入。旧的 `group_message_history_YYYYMM`、
`private_message_history_YYYYMM` 集合与分桶逻辑已随社交功能删除。

## 4. PostgreSQL

| 数据库 | 用途 |
| --- | --- |
| `orchestrator` | orchestrator 的 `saga_map`（Saga 状态、乐观锁、租约锁） |
| `ai_audit` | LLM 成本表 `llm_api_costs` |

### 4.1 LLM 成本表

- 表：`ai_audit.llm_api_costs`；
- 按 `created_at` **RANGE 分区**，分区名 `llm_api_costs_YYYYMM`；
- 分区边界：每月 1 日 00:00 +08 到次月 1 日 00:00 +08；
- ai_service 启动时自动补建**当月与下月**分区（[store.py](../services/ai_service/infra/cost/store.py)）；
- 字段：`user_id`、`provider`、`model`、prompt/completion/total tokens、
  `input_price`、`output_price`、`cost_usd`、`message_id`、`created_at`。

> 部署时 `ai_audit` 库需要存在；`scripts/init-multiple-databases.sh` 会在初始化时创建。

## 5. Redis

仅 auth_service 使用，存放邮箱验证码等短生命周期数据，按 key 设置 TTL。

## 6. 未实现：聊天消息存储

当前事实：

- gateway 只把用户消息发到 Kafka，不落库；
- ai_service 只把 AI 回复发到 Kafka，不落库；
- gateway 消费 AI 回复只做 WS 推送，不落库；
- 前端只把**会话列表**写进 localStorage，消息本身不保存。

因此：

- 刷新页面后消息全部丢失；
- ai_service 的 `assemble_context` 只能拿到当前一条消息，短期记忆的折叠逻辑不会触发；
- 没有任何地方能提供历史消息或会话列表接口。

后续如果要做，需要单独确定“谁消费 Kafka、写哪个库、暴露什么查询接口”，
本文不预设实现。

> 临时说明：ai_service 的 `memory/context.py` 目前直连 Mongo 写折叠检查点
> （`conversation_memory`），这是存储链路接入前的临时实现，不属于正式存储架构。

## 7. 开发注意

1. Docker 与宿主机可能同时监听同名端口（MongoDB/Redis/PostgreSQL）；
   排查时先确认 `localhost` 实际由谁监听。
2. `ai_audit` 与 `orchestrator` 是不同数据库；ai_service 只连 `ai_audit`。
3. gateway 不再需要 Mongo/Postgres/Redis 连接；如果本地还配了这些环境变量，可以删掉。
4. 成本表分区由 ai_service 启动时自动创建；如果数据库账号没有建表权限，成本记录会静默停用。

## 8. 代码索引

| 存储 | 代码 |
| --- | --- |
| Kafka 事件信封 | [proto/chant/common/v1/envelope.proto](../proto/chant/common/v1/envelope.proto) |
| Kafka 生产/消费（ai） | [infra/kafka](../services/ai_service/infra/kafka/consumer.py) |
| 成本表与分区 | [infra/cost/store.py](../services/ai_service/infra/cost/store.py) |
| auth 的 Mongo 访问 | [services/auth_service/app/database](../services/auth_service/app/database/mongodb_service.py) |
| auth 的 Redis 访问 | [services/auth_service/app/database/redis_service.py](../services/auth_service/app/database/redis_service.py) |
| Saga 表模型 | [saga_map.go](../services/orchestrator_service/database/pgsql/model/saga_map.go) |
| gateway 的 Kafka 消费 | [event_consumer.go](../services/chat_service/app/infrastructure/kafka/event_consumer.go) |
