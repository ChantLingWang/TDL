# Proto 字段重设计：round / turn / step

> 状态：**草案（draft）**，用于评审，尚未实现。
> 前置：PR #1 已把未使用字段清掉，当前 proto 只剩真正被读写的字段。

## 1. 目标

1. 同一会话内，所有事件有唯一、确定的顺序；
2. 消费者在 at-least-once 投递下可以幂等处理，不重复回复、不重复计费；
3. 支持“同一轮重试”和“取消旧回合”；
4. 为后续存储/重放留好键，不要求现在实现 storage。

## 2. 非目标

- 不实现 exactly-once；
- 不实现 storage；
- 不实现 token 级流式；
- 不新增独立的事件类型（step 先用 `AiReplyDelta(kind="progress")` 表达）。

## 3. 事件流

```
MessageSent(round=N, turn_id=T)          seq=0（隐式）
  │
  ├─ AiReplyDelta(progress, step=1, seq=1)
  ├─ AiReplyDelta(progress, step=2, seq=2)
  ├─ AiReplyDelta(thinking, step=3, seq=3..)
  ├─ AiReplyDelta(content,  step=4, seq=..)
  └─ AiReplyGenerated(step=5, seq=last, status=COMPLETED)
```

- 一个 `turn` = 一条用户消息触发的一次完整处理；
- 一个 `step` = 一次模型调用或一批工具调用；
- 一个 `seq` = 轮内一个事件（delta/progress/final 共用）。

## 4. 字段

### MessageSent

| Tag | 字段 | 类型 | 说明 |
|---|---|---|---|
| 9 | `round` | int64 | 第几轮，每会话单调递增 |
| 10 | `turn_id` | string | 本轮唯一 ID（UUID） |

> 7、8 已 reserved（原 timestamp_ms / metadata）。

### AiReplyDelta

| Tag | 字段 | 类型 | 说明 |
|---|---|---|---|
| 11 | `round` | int64 | 对应 MessageSent.round |
| 12 | `turn_id` | string | 对应 MessageSent.turn_id |
| 13 | `step` | int64 | 逻辑步骤，0=用户消息，1..=模型/工具 |
| 14 | `attempt` | int32 | 同一轮的重试次数，默认 0 |

> `seq`（Tag 6）保留，语义改为**轮内事件序号**：用户消息为 0，AI 事件从 1 开始。
> 9、10 已 reserved（原 timestamp_ms / metadata）。

### AiReplyGenerated

| Tag | 字段 | 类型 | 说明 |
|---|---|---|---|
| 9 | `round` | int64 | 同上 |
| 10 | `turn_id` | string | 同上 |
| 11 | `step` | int64 | 最后一个逻辑步骤 |
| 12 | `seq` | int64 | 本轮最后一个事件序号 |
| 13 | `attempt` | int32 | 同上 |
| 14 | `status` | TurnStatus | 终止状态 |
| 15 | `status_reason` | string | 取消/失败原因 |

> 7 已 reserved（原 timestamp_ms）。

### TurnStatus

```proto
enum TurnStatus {
  TURN_STATUS_UNSPECIFIED = 0;
  TURN_STATUS_RUNNING = 1;
  TURN_STATUS_COMPLETED = 2;
  TURN_STATUS_CANCELED = 3;
  TURN_STATUS_FAILED = 4;
}
```

`AiReplyGenerated.status` 只能是 COMPLETED / CANCELED / FAILED；RUNNING 留给未来的 step 事件。

## 5. 字段归属

| 字段 | 写者 | 规则 |
|---|---|---|
| `conversation_id` | gateway | 会话 ID，同时是 Kafka key |
| `message_id` | 客户端 / gateway | 用户消息唯一键 |
| `round` | **gateway** | 每会话原子递增，不复用 |
| `turn_id` | **gateway** | UUID，本轮唯一 |
| `seq` | **ai_service** | 轮内从 1 递增，所有事件共用 |
| `step` | **ai_service** | 0=用户消息，1..=逻辑步骤 |
| `attempt` | **ai_service** | 重试时 +1 |
| `status` / `status_reason` | **ai_service** | 只在终止事件写 |

约束：

- 同一个 turn 的 `seq` 只能有一个写者（ai_service），避免并发分配冲突；
- `round` 不能靠内存计数器，多实例/重启会重复。

## 6. 排序

- Kafka key：**所有会话事件统一用 `conversation_id`**；
- 逻辑顺序：`(round, seq)`；
- `step` 只用于 UI 分组，不参与排序；
- `timestamp` / `timestamp_ms` 只用于展示，不参与排序。

## 7. 幂等

| 层 | 唯一键 | 在哪生效 |
|---|---|---|
| 用户消息 | `(conversation_id, message_id)` | gateway / storage 唯一索引 |
| 一轮执行 | `turn_id` | ai_service 开跑前 `SETNX` |
| 事件 | `(turn_id, attempt, seq)` | storage 唯一索引 |
| 最终回复 | `(turn_id, attempt)` | storage upsert |
| Kafka 事件 | `event_id` | 消费端可选去重 |
| Kafka 生产者 | `enable.idempotence=true` | producer |

注意：

- `round` **不是**幂等键，只负责顺序；
- 重试必须复用 `turn_id` 和 `round`，只递增 `attempt`；
- 每个 `(turn_id, attempt)` 只能有一个终止事件。

## 8. 生命周期

```
MessageSent(round, turn_id)
  → RUNNING（隐式）
  → AiReplyGenerated(status=COMPLETED | CANCELED | FAILED)
```

- 每一轮必须有且只有一个终止事件；
- 取消/失败也要发终止事件，不能只断流；
- `round` 一旦分配就不回收，即使这一轮被取消。

## 9. 重试与取消

### 重试

- 同一个 `turn_id` + `round`，`attempt + 1`；
- 每个 attempt 的 `seq` 从 1 重新开始；
- storage 唯一键是 `(turn_id, attempt, seq)`，所以重试不会覆盖旧事件；
- 消费端在开跑前用 `(turn_id, attempt)` 去重，避免重复调 LLM。

### 取消

- 同一会话的新消息（更大 `round`）可以取消旧回合（后续实现）；
- 旧回合发 `AiReplyGenerated(status=CANCELED, status_reason="superseded")`，无 content；
- 旧用户消息仍要入库，并标记为已取消（storage 阶段实现）；
- 当前实现仍是“同会话排队”，取消是下一阶段。

## 10. Kafka key

| 事件 | 现在 | 建议 |
|---|---|---|
| `MessageSent` | `conversation_id` | 不变 |
| `AiReplyDelta` | `target_user_id` | 改为 `conversation_id` |
| `AiReplyGenerated` | `target_user_id` | 改为 `conversation_id` |

gateway 仍然用 `target_user_id` 字段决定推给谁，key 只影响分区。

## 11. 兼容与灰度

- 新字段全部可选；旧事件缺省 `round=0`、`turn_id=""`、`attempt=0`；
- `seq` 语义从“回复内 0 开始”改为“轮内 1 开始”，前端按 seq 排序不受影响；
- 不要把新字段塞进 `metadata`；
- proto 字段号已 reserved 的不要复用。

## 12. 实施步骤

1. proto 加字段 + `TurnStatus`，重新生成 Go/Python；
2. gateway：分配 `round` / `turn_id`，写入 `MessageSent`；
3. ai_service：解析并透传 `round`/`turn_id`，分配 `seq`/`step`/`attempt`，终态写 `status`；
4. ai_service producer：Kafka key 改为 `conversation_id`；
5. 测试：字段透传、seq 单调、同轮重试幂等键、终止事件唯一；
6. 更新 `docs/message-chain.md`；
7. 手动提交 / 幂等消费 / storage 放到后续阶段。

## 13. 待拍板

1. **`round` 谁分配**
   - gateway + Redis `INCR`（推荐，快且准）；
   - storage `max(round)+1`（storage 落地后自然）；
   - 内存计数器（多实例/重启会重复，不建议）。
2. **`seq` 语义**
   - 轮内事件序号，从 1 开始（本草案推荐）；
   - 或保留“回复内从 0 开始”，另加 `event_seq`。
3. **终止事件**
   - 扩展现有 `AiReplyGenerated`（本草案推荐）；
   - 或新增独立 `AiTurnState`。
4. **是否现在加 `client_message_id` / `schema_version`**
   - 本草案暂不加，避免再次引入噪音。
