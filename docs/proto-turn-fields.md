# Proto 字段重设计：step 流

> 状态：**已定稿，开始实现**。
> 前置：PR #1 已把未使用字段清掉。

## 1. 目标

1. 同一会话内，所有事件有唯一、确定的顺序；
2. at-least-once 投递下可幂等处理；
3. 支持同一轮重试、取消旧回合；
4. 所有 AI 输出统一成一种事件：`AiTurnStep`。

## 2. 轮与步

- **轮（round）**：用户一条消息 + AI 从开始处理到最终回复；
- **步（step）**：轮内的一个逻辑阶段，轮内从 1 递增；
- AI 的输出只有两类：`THINK`（思考）和 `RESULT`（正文或工具调用）；
- 工具执行本身不产生事件；
- 工具返回值是填回 AI 的新输入，必须独立成 `TOOL_RESULT` 步。

四种 kind：

```
THINK       思考
TOOL_CALL   工具调用（一批算一步，没有 status）
TOOL_RESULT 工具返回（一批算一步，每个工具带 status / duration）
RESULT      最终回答 + 本轮终止状态
```

天气例子：

```
step 1  THINK        思考
step 2  TOOL_CALL    weather(city, date)
step 3  TOOL_RESULT  weather 返回 {temperature: 25, type: "windy"}
step 4  THINK        思考
step 5  RESULT       result: "今天天气……", status: completed
```

## 3. 最终契约

### Kafka 里只有两种事件

| 事件 | 生产者 | 消费者 | 作用 |
|---|---|---|---|
| `MessageSent` | gateway | ai_service | 用户消息，开启一轮 |
| `AiTurnStep` | ai_service | gateway | AI 的每一步，含最终 RESULT |

`AiReplyDelta` 和 `AiReplyGenerated` 在迁移完成后删除。

### MessageSent 新增

| Tag | 字段 | 类型 | 说明 |
|---|---|---|---|
| 9 | `round` | int64 | 第几轮，gateway 分配 |

### AiTurnStep（新增）

| Tag | 字段 | 类型 | 说明 |
|---|---|---|---|
| 1 | `sender_id` | string | AI 用户 ID |
| 2 | `target_user_id` | string | 推给谁 |
| 3 | `conversation_id` | string | 会话 ID，Kafka key |
| 4 | `reply_to_msg_id` | string | 回复哪条用户消息 |
| 5 | `message_id` | string | AI 回复 ID（`ai-<用户消息ID>`），前端按它归组 |
| 6 | `round` | int64 | 轮号 |
| 7 | `step` | int64 | 步号，轮内从 1 递增 |
| 8 | `attempt` | int32 | 重试次数，默认 0 |
| 9 | `kind` | string | THINK / TOOL_CALL / TOOL_RESULT / RESULT |
| 10 | `payload` | string | 该步的 JSON |

没有 seq、没有 result_type、没有 step 级 status/name/detail/duration，全部在 payload 里。

### payload 形状

THINK：

```json
{ "think": "用户问天气，应该查工具" }
```

TOOL_CALL（没有 status；一批工具是数组；必须带模型给的 tool_call id）：

```json
{
  "calls": [
    { "id": "call_1", "name": "weather", "input": { "city": "北京", "date": "2026-11-23" } }
  ]
}
```

TOOL_RESULT（每个工具带 id、name、payload、status、duration_ms；失败带 error）：

```json
{
  "results": [
    {
      "id": "call_1",
      "name": "weather",
      "payload": { "temperature": 25, "type": "windy" },
      "status": "completed",
      "duration_ms": 820,
      "error": ""
    }
  ]
}
```

RESULT（同时承担本轮终止）：

```json
{ "result": "今天天气晴，25 度……", "status": "completed", "reason": "" }
```

取消/失败时没有 result，只有 status 和 reason：

```json
{ "status": "canceled", "reason": "superseded" }
```

## 4. 字段分类

| 分类 | 字段 |
|---|---|
| 归属/身份 | conversation_id、reply_to_msg_id、message_id |
| 顺序 | round、step、attempt |
| 类型 | kind |
| 内容 | payload |

## 5. 字段归属

| 字段 | 写者 | 规则 |
|---|---|---|
| conversation_id | gateway | 为空时生成 UUID |
| message_id（用户） | 客户端 / gateway | 为空时生成 UUID |
| round | **gateway** | 每会话递增，单 goroutine 分配 |
| reply_to_msg_id | ai_service | 从 MessageSent.message_id 复制 |
| message_id（AI） | ai_service | `ai-<用户消息ID>` |
| step | **ai_service** | 轮内从 1 递增 |
| attempt | **ai_service** | 重试时 +1 |
| kind / payload | **ai_service** | 按实际阶段生成 |

## 6. 排序

- Kafka key：所有会话事件统一用 `conversation_id`；
- 跨轮：`round`；
- 轮内：`step`；
- 每个 step 就是一条 Kafka 消息，不再需要 seq；
- timestamp 只用于展示。

## 7. 幂等

| 层 | 唯一键 |
|---|---|
| 用户消息 | `(conversation_id, message_id)` |
| 一轮执行 | `(conversation_id, reply_to_msg_id)` |
| 一步 | `(conversation_id, reply_to_msg_id, attempt, step)` |
| 终止 | `(conversation_id, reply_to_msg_id, attempt)` 只能有一个 RESULT |

round 只负责顺序，不作幂等键。

## 8. 生命周期

```
MessageSent(round)
  → THINK? / TOOL_CALL? / TOOL_RESULT? ...（0..N 步）
  → RESULT(status=completed | canceled | failed, reason)
```

- 每一轮必须有且只有一个 RESULT；
- 取消/失败也用 RESULT 收尾，不能只断流；
- 工具失败是 TOOL_RESULT(status=failed) 里的一个工具状态，不一定是整轮失败。

## 9. 重试与取消

### 重试

- 同一个 `reply_to_msg_id` + `round`，`attempt + 1`；
- 每个 attempt 的 `step` 从 1 重新开始；
- 前端看到 attempt 变化时，应清掉旧 attempt 的步骤；
- 消费端开跑前用 `(conversation_id, reply_to_msg_id, attempt)` 去重。

### 取消

- 同一会话的新消息（更大 round）可以取消旧回合（后续实现）；
- 旧回合发 RESULT(status=canceled, reason=superseded)；
- 旧用户消息仍要入库并标记取消（storage 阶段）；
- 当前实现仍是同会话排队。

## 10. gateway 的职责

gateway 不是业务存储，它只做入口计数、路由和推送：

1. **round 分配**：一个 goroutine 独占 `map[conversation_id]round`，通过 channel 接收请求；
2. **同会话串行**：分配 round 和发 Kafka 在同一个串行段里完成，保证 round 顺序和 Kafka 顺序一致；
3. **消息路由**：消费 `AiTurnStep`，按 `target_user_id` 推给本地 WS 连接；
4. **幂等**（后续）：按 message_id 去重，重复的消息复用原 round，不再发 Kafka；
5. **会话归属校验**（后续，storage 阶段）：确认 conversation_id 属于当前用户。

当前阶段先不做 4 和 5；round 表放内存，单实例有效，重启归零。

## 11. 迁移步骤

为保持每一步可编译，分三阶段：

1. **Phase 1（additive）**：proto 加 `round` 和 `AiTurnStep`；gateway 加 round 分配器和 `AiTurnStep` 推送；旧 `AiReply*` 保留，旧链路照常工作；
2. **Phase 2（cutover）**：ai_service 改为只发 `AiTurnStep`；前端改为只渲染 step；`AiReplyDelta` / `AiReplyGenerated` 停止生产；
3. **Phase 3（cleanup）**：删除旧消息、旧 producer、旧前端流式代码和测试。

## 12. 已定 / 待定

已定：

- step / attempt 由 ai_service 分配；
- round 由 gateway 单 goroutine 分配；
- 工具调用/返回带 tool_call id；
- payload 沿用 8000 字符截断；
- RESULT 带 status + reason；
- 没有 seq、没有 AiReplyDelta/AiReplyGenerated（Phase 3 删除）。

待定（后续）：

- gateway 幂等表（先内存，后续 Redis/存储）；
- conversation 归属校验（等 storage）；
- 取消旧回合（后续 turn registry）；
- 前端 attempt 切换时清步骤。
