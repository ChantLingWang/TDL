# Proto step 流契约（已落地）

> 状态：**已实现**。
> 2026-11-23：typed oneof + protobuf 二进制已落地；旧的 `AiReplyDelta` / `AiReplyGenerated` 已删除。

## 1. 目标

1. 同一会话内所有事件有唯一、确定的顺序；
2. at-least-once 投递下可幂等处理；
3. 支持同一轮重试、取消旧回合；
4. 所有 AI 输出统一成一种事件：`AiTurnStep`。

## 2. 轮与步

- **轮（round）**：用户一条消息 + AI 从开始处理到最终回复；
- **步（step）**：轮内的一个逻辑阶段，轮内从 1 递增；
- AI 的输出只有两类：`THINK`（思考）和 `RESULT`（正文或工具调用）；
- 工具执行本身不产生事件；
- 工具返回值是填回 AI 的新输入，必须独立成 `TOOL_RESULT` 步。

四种 step：

```
THINK        思考
TOOL_CALL    工具调用（一批算一步，没有 status）
TOOL_RESULT  工具返回（一批算一步，每个工具带 status / duration）
RESULT       最终回答 + 本轮终止
```

天气例子：

```
step 1  THINK
step 2  TOOL_CALL    weather(city, date)
step 3  TOOL_RESULT  weather 返回 {temperature: 25, type: "windy"}
step 4  THINK
step 5  RESULT       result: "今天天气……", status: COMPLETED
```

## 3. Kafka 事件

Kafka value 是 **EventEnvelope 的 protobuf 二进制**；
`EventEnvelope.data` 是具体事件的 **protobuf 二进制**。
没有 protojson、没有 base64、没有 JSON 字符串嵌套。

只有两种事件：

| 事件 | 生产者 | 消费者 | 作用 |
|---|---|---|---|
| `MessageSent` | gateway | ai_service | 用户消息，开启一轮 |
| `AiTurnStep` | ai_service | gateway | AI 的每一步 |

## 4. MessageSent

| Tag | 字段 | 类型 |
|---|---|---|
| 1 | sender_id | string |
| 2 | target_user_id | string |
| 3 | conversation_id | string |
| 4 | content | string |
| 5 | message_id | string |
| 6 | message_type | string |
| 9 | round | int64 |

7/8 已 reserved。

## 5. AiTurnStep

| Tag | 字段 | 类型 | 说明 |
|---|---|---|---|
| 1 | sender_id | string | AI 用户 ID |
| 2 | target_user_id | string | 推给谁 |
| 3 | conversation_id | string | 会话 ID，Kafka key |
| 4 | reply_to_msg_id | string | 回复哪条用户消息 |
| 5 | message_id | string | AI 回复 ID（ai-<用户消息ID>） |
| 6 | round | int64 | 轮号 |
| 7 | step | int64 | 步号，轮内从 1 递增 |
| 8 | attempt | int32 | 重试次数，默认 0 |
| 11-14 | oneof payload | — | think / tool_call / tool_result / result |

9/10 已 reserved（原 kind / payload 字符串）。

```proto
oneof payload {
  ThinkPayload think = 11;
  ToolCallPayload tool_call = 12;
  ToolResultPayload tool_result = 13;
  ResultPayload result = 14;
}
```

### payload messages

```proto
message ThinkPayload { string think = 1; }

message ToolCall {
  string id = 1;
  string name = 2;
  google.protobuf.Value input = 3;
}
message ToolCallPayload { repeated ToolCall calls = 1; }

message ToolResult {
  string id = 1;
  string name = 2;
  google.protobuf.Value payload = 3;
  ToolStatus status = 4;
  int64 duration_ms = 5;
  string error = 6;
}
message ToolResultPayload { repeated ToolResult results = 1; }

message ResultPayload {
  string result = 1;
  ResultStatus status = 2;
  string reason = 3;
}
```

枚举：

```proto
enum ResultStatus {
  RESULT_STATUS_UNSPECIFIED = 0;
  RESULT_STATUS_COMPLETED = 1;
  RESULT_STATUS_CANCELED = 2;
  RESULT_STATUS_FAILED = 3;
}

enum ToolStatus {
  TOOL_STATUS_UNSPECIFIED = 0;
  TOOL_STATUS_COMPLETED = 1;
  TOOL_STATUS_FAILED = 2;
  TOOL_STATUS_TIMEOUT = 3;
}
```

- 一批工具是一个 TOOL_CALL + 一个 TOOL_RESULT；
- 每个工具带模型的 tool_call id，前端按 id 配对；
- 工具结果沿用 8000 字符截断；
- 每轮必须有且只有一个 RESULT；
- 取消/失败用 RESULT 收尾，带 status + reason。

## 6. 字段分类

| 分类 | 字段 |
|---|---|
| 归属/身份 | conversation_id、reply_to_msg_id、message_id |
| 顺序 | round、step、attempt |
| 类型 | oneof payload 的分支 |
| 内容 | typed payload messages |

## 7. 字段归属

| 字段 | 写者 | 规则 |
|---|---|---|
| conversation_id | gateway | 为空时生成 UUID |
| message_id（用户） | 客户端 / gateway | 为空时生成 UUID |
| round | gateway | 每会话递增，单 goroutine 分配 |
| reply_to_msg_id | ai_service | 从 MessageSent.message_id 复制 |
| message_id（AI） | ai_service | ai-<用户消息ID> |
| step | ai_service | 轮内从 1 递增 |
| attempt | ai_service | 重试时 +1 |
| oneof payload | ai_service | 按实际阶段填 |

## 8. 排序

- Kafka key：所有会话事件统一用 `conversation_id`；
- 跨轮：`round`；
- 轮内：`step`；
- 一条 step 一条 Kafka 消息，不需要 seq；
- timestamp 只用于展示。

## 9. 幂等

| 层 | 唯一键 |
|---|---|
| 用户消息 | (conversation_id, message_id) |
| 一轮执行 | (conversation_id, reply_to_msg_id) |
| 一步 | (conversation_id, reply_to_msg_id, attempt, step) |
| 终止 | (conversation_id, reply_to_msg_id, attempt) 只能有一个 RESULT |

round 只负责顺序，不作幂等键。

## 10. 生命周期

```
MessageSent(round)
  → THINK? / TOOL_CALL? / TOOL_RESULT? ...（0..N 步）
  → RESULT(status=COMPLETED | CANCELED | FAILED, reason)
```

- 每一轮必须有且只有一个 RESULT；
- 取消/失败也用 RESULT 收尾，不能只断流；
- 工具失败是 TOOL_RESULT 里的一个工具状态，不一定是整轮失败。

## 11. 重试与取消

### 重试

- 同一个 reply_to_msg_id + round，attempt + 1；
- 每个 attempt 的 step 从 1 重新开始；
- 前端看到 attempt 变化时清掉旧 attempt 的步骤；
- 消费端开跑前用 (conversation_id, reply_to_msg_id, attempt) 去重。

### 取消

- 同一会话的新消息（更大 round）可以取消旧回合（后续实现）；
- 旧回合发 RESULT(status=CANCELED, reason=superseded)；
- 当前仍是同会话排队。

## 12. gateway 职责

1. 单 goroutine 分配 round，并在同一个串行段里发 Kafka；
2. 消费 AiTurnStep，把 typed oneof 转成 WS 的 kind + payload JSON；
3. 按 target_user_id 推本地 WS；
4. 幂等（后续）：按 message_id 去重；
5. 归属校验（后续，storage 阶段）。

round 表在内存，单实例有效，重启归零。

## 13. 已知缺口

- Kafka 消费仍是自动提交；
- 没有 message_id 幂等；
- 没有 conversation 归属校验；
- round 在内存，多实例/重启会重复；
- 取消旧回合未实现；
- 重试 attempt 未实现；
- 没有 ack/心跳/超时终止；
- payload 二进制，Kafka UI 不可直接读。

## 14. 迁移状态

- Phase 1：加 round + AiTurnStep（完成）
- Phase 2：ai_service 只发 AiTurnStep，前端只渲染 step（完成）
- Phase 3：删除旧事件/旧 producer/旧前端逻辑（完成）
