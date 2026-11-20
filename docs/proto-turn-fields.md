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
- 不把工具执行过程本身做成事件（执行不产生数据）。

## 3. 轮与步

### 轮（round）

- 用户发一条消息 → AI 从开始处理到最终回复，算**一轮**；
- 中间所有思考、工具调用、工具返回、最终输出都属于这一轮；
- 一轮只有一条用户消息和一条最终 AI 回复（或 CANCELED / FAILED 终止事件）。

### 步（step）

AI 每一轮不是“一次思考、一次回复”，而是循环：

```
think → result(tool_call) → observation → think → result(content)
```

- **AI 的输出只有两类**：`THINK`（推理）和 `RESULT`（正文或工具调用）；
- 工具调用是 `RESULT` 的一种（`result_type=TOOL_CALL`）；
- 工具执行本身不产生数据，**不发事件**；
- 工具执行完由函数交出的返回值，是填回 AI 的**新输入**，必须独立成 `OBSERVATION` 步，能独立展示；
- 一批并行/串行工具算**一个** `RESULT(tool_call)` 步 + **一个** `OBSERVATION` 步。

天气例子：

```
Round 12: 用户问“今天天气如何？”

step 1  THINK         "用户问天气，我应该查工具"
step 2  RESULT        tool_call: weather(city=...)
step 3  OBSERVATION   weather 返回 {temp:25, sky:"晴"}
step 4  THINK         "25℃晴，我应该这样回复"
step 5  RESULT        content: "今天天气晴，25 度……"
```

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
| 13 | `step` | int64 | 这条文本属于哪一步 |
| 14 | `attempt` | int32 | 同一轮的重试次数，默认 0 |

> `seq`（Tag 6）保留，语义改为**轮内事件序号**：用户消息为 0，之后所有事件从 1 递增。
> 9、10 已 reserved（原 timestamp_ms / metadata）。

### AiTurnStep（新增）

| Tag | 字段 | 类型 | 说明 |
|---|---|---|---|
| 1 | `sender_id` | string | AI 用户 ID |
| 2 | `target_user_id` | string | 推给谁 |
| 3 | `conversation_id` | string | 会话 ID |
| 4 | `turn_id` | string | 本轮 ID |
| 5 | `round` | int64 | 轮号 |
| 6 | `step` | int64 | 步号，轮内从 1 递增 |
| 7 | `seq` | int64 | 轮内事件序号 |
| 8 | `attempt` | int32 | 重试次数 |
| 9 | `kind` | StepKind | THINK / RESULT / OBSERVATION |
| 10 | `result_type` | ResultType | kind=RESULT 时：CONTENT / TOOL_CALL |
| 11 | `name` | string | 工具名（OBSERVATION / TOOL_CALL） |
| 12 | `status` | string | requested / completed / failed / timeout |
| 13 | `detail` | string | 给用户看的摘要 |
| 14 | `payload` | string | 结构化数据（工具参数 / 工具返回 JSON） |
| 15 | `duration_ms` | int64 | 工具执行耗时 |
| 16 | `metadata` | map | 扩展字段 |

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

### 枚举

```proto
enum StepKind {
  STEP_KIND_UNSPECIFIED = 0;
  STEP_KIND_THINK = 1;
  STEP_KIND_RESULT = 2;
  STEP_KIND_OBSERVATION = 3;
}

enum ResultType {
  RESULT_TYPE_UNSPECIFIED = 0;
  RESULT_TYPE_CONTENT = 1;
  RESULT_TYPE_TOOL_CALL = 2;
}

enum TurnStatus {
  TURN_STATUS_UNSPECIFIED = 0;
  TURN_STATUS_RUNNING = 1;
  TURN_STATUS_COMPLETED = 2;
  TURN_STATUS_CANCELED = 3;
  TURN_STATUS_FAILED = 4;
}
```

## 5. 字段归属

| 字段 | 写者 | 规则 |
|---|---|---|
| `conversation_id` | gateway | 会话 ID，同时是 Kafka key |
| `message_id` | 客户端 / gateway | 用户消息唯一键 |
| `round` | **gateway** | 每会话原子递增，不复用 |
| `turn_id` | **gateway** | UUID，本轮唯一 |
| `step` | **ai_service** | 轮内从 1 递增，THINK/RESULT/OBSERVATION 共用 |
| `seq` | **ai_service** | 轮内事件序号，step 事件和 delta 共用 |
| `attempt` | **ai_service** | 重试时 +1 |
| `status` / `status_reason` | **ai_service** | 终止事件 / 工具状态 |

约束：

- 同一个 turn 的 `step` / `seq` 只能有一个写者（ai_service）；
- `round` 不能靠内存计数器，多实例/重启会重复。

## 6. 排序

- Kafka key：**所有会话事件统一用 `conversation_id`**；
- 逻辑顺序：`(round, seq)`；
- `step` 用于分组（一次思考、一次工具调用、一次工具返回、一次最终输出）；
- `timestamp` 只用于展示，不参与排序。

## 7. 幂等

| 层 | 唯一键 | 在哪生效 |
|---|---|---|
| 用户消息 | `(conversation_id, message_id)` | gateway / storage 唯一索引 |
| 一轮执行 | `turn_id` | ai_service 开跑前 `SETNX` |
| 事件 | `(turn_id, attempt, seq)` | storage 唯一索引 |
| 步 | `(turn_id, attempt, step)` | storage 唯一索引（step 事件） |
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
  → THINK? / RESULT? / OBSERVATION? ...（0..N 步）
  → AiReplyGenerated(status=COMPLETED | CANCELED | FAILED)
```

- 每一轮必须有且只有一个终止事件；
- 取消/失败也要发终止事件，不能只断流；
- `round` 一旦分配就不回收，即使这一轮被取消；
- 工具失败/超时是 `OBSERVATION(status=failed/timeout)`，不一定终止整轮。

## 9. 重试与取消

### 重试

- 同一个 `turn_id` + `round`，`attempt + 1`；
- 每个 attempt 的 `step` / `seq` 从 1 重新开始；
- storage 唯一键是 `(turn_id, attempt, seq)`，重试不会覆盖旧事件；
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
| `AiTurnStep` | 无 | `conversation_id` |
| `AiReplyDelta` | `target_user_id` | 改为 `conversation_id` |
| `AiReplyGenerated` | `target_user_id` | 改为 `conversation_id` |

gateway 仍然用 `target_user_id` 字段决定推给谁，key 只影响分区。

## 11. 兼容与灰度

- 新字段全部可选；旧事件缺省 `round=0`、`turn_id=""`、`attempt=0`；
- `seq` 语义从“回复内 0 开始”改为“轮内 1 开始”，前端按 seq 排序不受影响；
- `AiTurnStep` 是新事件类型，旧消费者忽略即可；
- 不要把新字段塞进 `metadata`；
- proto 字段号已 reserved 的不要复用。

## 12. 实施步骤

1. proto 加字段 + `AiTurnStep` + 三个枚举，重新生成 Go/Python；
2. gateway：分配 `round` / `turn_id`；消费并广播 `AiTurnStep`；AI 回复 key 改 `conversation_id`；
3. ai_service：在 AgentLoop 中按 THINK / RESULT / OBSERVATION 发 step 事件；分配 `step`/`seq`/`attempt`；终态写 `status`；
4. 前端：渲染 step 事件（思考 / 工具调用 / 工具返回 / 最终输出）；
5. 测试：step 顺序、seq 单调、同轮重试幂等键、终止事件唯一、工具失败仍可展示；
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
4. **`AiTurnStep.payload` 形态**
   - 先用 JSON 字符串，简单；
   - 或定义结构化的 `repeated ToolResult`，以后再加。
5. **是否现在加 `client_message_id` / `schema_version`**
   - 本草案暂不加，避免再次引入噪音。
