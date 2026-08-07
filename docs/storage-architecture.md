# Chant 存储架构文档

> 本文档描述 Chant 各存储组件的职责、命名规则、数据结构与读写路径。
> 所有名称以当前代码为准，改动存储结构时请同步更新本文档。

## 1. 存储总览

| 存储 | 职责 | 谁在写 | 谁在读 |
| --- | --- | --- | --- |
| Kafka | 事件总线（不落业务状态） | chat_service / ai_service / orchestrator | chat_service / ai_service / orchestrator |
| MongoDB | 消息本体、用户、Token、离线时间真源 | chat_service / auth_service | chat_service / auth_service / ai_service（经 HTTP） |
| PostgreSQL | 群组/会话等关系数据、Saga 状态、LLM 成本 | chat_service / orchestrator / ai_service | chat_service / orchestrator / ai_service |
| Redis | 验证码、last_offline_time 缓存 | auth_service / chat_service | auth_service / chat_service |
| Qdrant | AI 长期记忆（向量） | ai_service | ai_service |

---

## 2. Kafka 事件总线

| Topic | 事件格式 | 用途 |
| --- | --- | --- |
| `chat_group_message` | Proto `EventEnvelope`（JSON 序列化） | 聊天消息与 AI 回复 |
| `saga-events` | 旧版 JSON `BusinessEvent` | Saga 编排 |
| `saga-dlq` | JSON DLQ 载荷 | Saga 死信队列 |
| `user-registrations` / `user-updates` / `sync_user_fields` | 未实现 | compose 中已配置，auth_service 代码未接入 |

`chat_group_message` 的 key 规则：

- 群聊消息：`getGroupPartitionKey(groupID)`，群 ID 数字部分奇偶分区
- 私聊消息：目标用户 ID
- AI 回复：目标用户 ID（保证与私聊消息同分区有序）

---

## 3. MongoDB

### 3.1 数据库与集合

| 数据库 | 集合 | 内容 | 写入方 |
| --- | --- | --- | --- |
| `chat` | `group_message_history_YYYYMM` | 群聊 / AI 会话消息 | chat_service |
| `chat` | `private_message_history_YYYYMM` | 私聊消息 | chat_service |
| `auth` | `Users` | 用户资料、`last_offline_time` | auth_service |
| `auth` | `UserTokens` | refresh token 及有效期 | auth_service |

### 3.2 消息分桶规则

- 集合按**月份**命名：`group_message_history_202608`
- 文档按**天**分桶：`date_identifier = YYYYMMDD`
- 群聊每个桶最多 `MaxMessagesPerBucket = 500` 条，超出自动开新桶；桶内维护 `count` / `start_time` / `end_time`
- 私聊桶按 `session_id + date_identifier` 区分

### 3.3 私聊会话 ID

`GenerateSessionID(userA, userB)`：两个用户 ID **排序后**用 `_` 拼接：

```
ai-assistant + smoke-user → ai-assistant_smoke-user
```

历史接口的 `conversation_id` 私聊时也必须传这个排序后的会话 ID，不能只传单个用户 ID。

### 3.4 消息文档字段

见 [message-chain.md](./message-chain.md#32-mongo-message-结构modelsgo)。

### 3.5 写入幂等

- 先 `$setOnInsert` 确保会话/桶文档存在（幂等，不重复建文档）
- 再以 `messages.message_id: {$ne: <message_id>}` 条件 `$push`，且**不使用 upsert**
- 效果：Kafka 重放 / 服务重启重复消费时，同一 `message_id` 只落一条

### 3.6 历史读取

- 按“最近 30 天”查询，但集合按月份命名，因此查询前会对**月份集合名去重**，避免同月重复读取
- 群聊按 `group_id` 过滤，私聊按 `session_id` 过滤

---

## 4. PostgreSQL

### 4.1 数据库

| 数据库 | 用途 |
| --- | --- |
| `orchestrator` | chat_service 关系表 + orchestrator 的 `saga_map`（两个服务共用） |
| `user_service` | 历史遗留，init 脚本创建，当前无代码使用 |
| `ai_audit` | LLM 成本表 `llm_api_costs`（独立库，需手动/脚本预创建） |

### 4.2 chat_service 表（位于 `orchestrator` 库）

| 表 | 用途 | 主键 |
| --- | --- | --- |
| `groups` | 群组元数据（group_id、group_name、group_type、create_by_user_id） | `group_id` |
| `user_groups` | 用户-群组关联 | `(user_id, group_id)` |
| `conversations` | 会话已读状态 / 最后读取时间 | `(user_id, conversation_id)` |
| `private_chats` | 私聊列表（历史遗留语义） | `(user_id, add_time)` |
| `temp_chats` | 临时会话（历史遗留语义） | `(user_id, source)` |

### 4.3 orchestrator 表

| 表 | 用途 | 关键字段 |
| --- | --- | --- |
| `saga_map` | Saga 状态持久化 | `status`、`version`（乐观锁）、`locked_by` / `lock_expiry`（分布式锁）、`context` / `steps`（JSON） |

### 4.4 LLM 成本表（`ai_audit.llm_api_costs`）

- 按 `created_at` **RANGE 分区**，分区命名 `llm_api_costs_YYYYMM`
- 分区边界：每月 1 日 00:00 **+08** 至次月 1 日 00:00 +08
- ai_service 启动时自动补建**当月与下月**分区（[store.py](../services/ai_service/shared/cost/store.py)）
- 字段：user_id、provider、model、prompt_tokens、completion_tokens、total_tokens、input_price、output_price、cost_usd、message_id、created_at

---

## 5. Redis

### 5.1 auth_service：邮箱验证码

- key：用户邮箱
- value：验证码
- TTL：600 秒

### 5.2 chat_service：last_offline_time 缓存

- key：`{userID}:{username}`（Hash）
- field：`last_offline_time`
- value：Unix 秒
- 未命中时通过 gRPC 从 auth_service 读取，**真源在 MongoDB `Users` 集合**

---

## 6. Qdrant（AI 长期记忆）

- 集合：`long_term_memory`
- 向量维度：4096（`Qwen/Qwen3-Embedding-8B`）
- 距离：Cosine
- payload：`user_id`、`group_id`、`question`、`report_summary`、`report_full`、`domain`、`methodology`、`created_at`
- 检索：向量召回 → 时间衰减 × 语义分混合排序 → 可选 `Qwen/Qwen3-Reranker-8B` 精排
- 用户隔离：查询时按 `user_id` 过滤

---

## 7. 读写路径摘要

### 写路径

```
前端 WS
  → chat_service：用户消息落 MongoDB（群/私聊）
  → Kafka（MessageSent）
      ├→ ai_service：读历史(HTTP) → LLM → 成本写 PG → 回复发 Kafka
      └→ chat_service 自身消费者：广播给在线用户
Kafka（AiReplyGenerated）
  → chat_service：AI 回复落 MongoDB + WS 推送
```

Agent 模式额外：研究报告 → Qdrant 长期记忆（fire-and-forget）。

### 读路径

| 场景 | 数据源 |
| --- | --- |
| 会话历史 | MongoDB（按月份集合 + 天数桶） |
| 未读消息 | Redis last_offline_time + MongoDB 时间过滤 |
| 群成员 / 会话关系 | PostgreSQL |
| Saga 状态 | PostgreSQL `saga_map` |
| LLM 成本 | PostgreSQL `ai_audit.llm_api_costs` |
| AI 长期记忆 | Qdrant |

---

## 8. 开发环境注意

本地开发时，Docker 与宿主机可能同时存在同名服务（例如宿主机的 MongoDB/Redis 占用 `localhost` 端口，优先于 Docker 容器被访问）。表现：

- `docker exec mongodb` 里看不到聊天数据，但 chat_service 历史接口有数据 → 说明 chat_service 连的是宿主机 MongoDB
- 8080 端口被旧 Python 进程占用时，chat_service 需换端口运行

排查时先确认 `localhost` 端口实际由谁监听，再判断数据落在 Docker 还是宿主机。

---

## 9. 关键代码索引

| 存储 | 代码 |
| --- | --- |
| Mongo 消息分桶 | [mongodb_group_message_history_service.go](../services/chat_service/app/database/mongodb/mongodb_group_message_history_service.go) |
| Mongo 私聊会话 | [mongodb_private_message_history_service.go](../services/chat_service/app/database/mongodb/mongodb_private_message_history_service.go) |
| Mongo 模型 | [models.go](../services/chat_service/app/database/mongodb/models.go) |
| PG 表模型 | [model.go](../services/chat_service/app/database/pgsql/model/model.go) |
| Saga 表模型 | [saga_map.go](../services/orchestrator_service/database/pgsql/model/saga_map.go) |
| 成本分区 | [store.py](../services/ai_service/shared/cost/store.py) |
| 长期记忆 | [long_term_memory.py](../services/ai_service/agent/tools/long_term_memory.py) |
| Redis 缓存 | [redis_handler.go](../services/chat_service/app/infrastructure/redis/redis_handler.go) |
