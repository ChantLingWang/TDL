# Chant 项目交接文档

日期：2026-11-27  
仓库：github.com/ChantLingWang/TDL  
当前 main：912d976 feat: step 流（typed oneof + protobuf 二进制） (#3)  
用途：在新 harness / 新机器上继续本会话。先读本文，再读 docs/message-chain.md、docs/proto-turn-fields.md。

## 1. 项目目标

- Chant 是一个 AI 聊天后端项目，用于求职作品集和真实压测。
- 目标不是只做 demo，而是设计一套能讲清楚的中等规模架构，然后在服务器上压测，拿到可解释的 QPS / p99 数据。
- 允许适度过度设计，但每个设计都要能说明：为什么分层、一致性模型是什么、瓶颈在哪里。
- 明确不做 token 级流式输出。AI 输出是 step 级事件，前端按 step 分段渲染。

## 2. 当前仓库状态

- 当前分支 main，commit 912d976。
- PR #3 已合并（squash）。融合了：
  - step 流
  - typed oneof
  - protobuf 二进制 Kafka
  - MessageSent 的 ContentPart parts
- 工作区干净。
- 已验证：
  - Python 44 个测试通过
  - Go / SDK 测试和构建通过
  - 前端 tsc / eslint / vite build 通过
  - buf lint 通过
- 如果新环境 main 落后，先 git pull。

## 3. 当前链路

    Frontend
      → WS /api/v1/ws
    Gateway（chat_service，Go）
      → Kafka chat_group_message（protobuf 二进制 EventEnvelope）
      → MessageSent
    AI service（ai_service，Python）
      → LLM
      → Kafka AiTurnStep
    Gateway
      → WS ai_step
    Frontend

关键点：

- Kafka key 统一用 conversation_id，保证同会话同分区有序。
- gateway 用单 goroutine 的 RoundAllocator 分配 round，并在同一个串行段发 Kafka。
- ai_service 分配 step（轮内从 1 递增）和 attempt（当前恒 0）。
- 每个 step 一条 Kafka 消息，不需要 seq。
- 一轮 = 一条 MessageSent + N 条 AiTurnStep。

## 4. 协议（已完成，不要重新引入旧设计）

### 4.1 MessageSent

字段：

- sender_id：用户 ID
- target_user_id：AI 用户 ID（如 ai-assistant），不是 Kafka topic
- conversation_id
- message_id：用户消息 ID
- message_type
- round：gateway 分配
- parts：repeated ContentPart（字段号 10）
- 原 content 字段（4）已 reserved，不再使用

ContentPart：

- type：text / image / audio / video
- text：文本
- data：bytes（图片/音频/视频；前端 base64，gateway 解码成 bytes）
- mime_type
- metadata：google.protobuf.Struct，任意 JSON，透传用

规则：

- 文本最多一个 part，必须放最后。
- 图片/音频/视频可多个，顺序即 image0、image1。
- 空字段省略；文本 part 的 data / mime_type / metadata 为空。

### 4.2 AiTurnStep

公共字段：

- sender_id：ai-assistant
- target_user_id：用户 ID
- conversation_id
- reply_to_msg_id：回复的用户消息 ID
- message_id：ai-<用户消息ID>
- round
- step
- attempt
- oneof payload（字段号 11-14）

oneof 分支：

- think：ThinkPayload { think }
- tool_call：ToolCallPayload { calls: ToolCall { id, name, input: Value } }
- tool_result：ToolResultPayload { results: ToolResult { id, name, payload: Value, status, duration_ms, error } }
- result：ResultPayload { result, status, reason }

枚举：

- ToolStatus：TOOL_STATUS_COMPLETED / TOOL_STATUS_FAILED / TOOL_STATUS_TIMEOUT
- ResultStatus：RESULT_STATUS_COMPLETED / RESULT_STATUS_CANCELED / RESULT_STATUS_FAILED

注意：

- proto 里没有 kind 字段，oneof 的分支就是类型。
- gateway 转成 WS 的 kind + payload。
- 每轮必须有且只有一个 RESULT。
- 旧 AiReplyDelta / AiReplyGenerated 已删除，不要恢复。

### 4.3 Kafka

- Kafka value 是 EventEnvelope 的 protobuf 二进制。
- EventEnvelope.data 是具体事件的 protobuf 二进制。
- 没有 protojson、没有 base64、没有 JSON 字符串嵌套。
- MessageSent 的 event_id 被 gateway 用用户 message_id 覆盖。
- AiTurnStep 的 event_id 是 chant.chat.v1.AiTurnStep-<uuid>。

### 4.4 gateway → 前端 WS

示例（实际是单行 JSON，这里为阅读换行）：

    {
      "type": "ai_step",
      "conversation_id": "conv_20261127_001",
      "sender": "ai-assistant",
      "reply_to_msg_id": "m_20261127_001",
      "message_id": "ai-m_20261127_001",
      "round": 1,
      "step": 2,
      "attempt": 0,
      "kind": "TOOL_CALL",
      "payload": {
        "calls": [
          {
            "id": "call_abc123",
            "name": "get_current_time",
            "input": { "readable": true }
          }
        ]
      },
      "time": 1795773023000
    }

- kind 大写：THINK / TOOL_CALL / TOOL_RESULT / RESULT。
- status 小写：completed / canceled / failed；工具还有 failed / timeout。
- payload 是对象，不是字符串。

## 5. 已完成的关键文件

- proto/chant/chat/v1/event.proto
- services/chat_service/app/services/round_allocator.go
- services/chat_service/app/infrastructure/kafka/services/event_handlers.go
- services/ai_service/runtime/agent_loop/loop.py
- services/ai_service/infra/proto_adapter.py
- services/ai_service/infra/kafka/producer.py
- services/ai_service/apps/chat/service.py
- services/ai_service/memory/context.py
- front_code/src/pages/ChatPage.tsx
- front_code/src/components/StepTimeline.tsx
- front_code/src/components/MessageList.tsx
- front_code/src/types/chat.ts
- docs/message-chain.md
- docs/proto-turn-fields.md

## 6. 当前限制（已知，不要假装已解决）

- 没有 storage：assemble_context 的 history 恒为空；刷新页面消息丢失。
- 图片链路只预留：前端没有选图/压缩/预览；ai_service 收到 image/audio/video 会返回失败 RESULT；没有多模态 LLM 适配。
- 没有 message_id / step 幂等。
- ai_service Kafka 仍是自动提交 + auto_offset_reset=latest；崩溃/重启可能丢消息。
- 没有 ack / 心跳 / 超时终止；ai_service 挂了前端会一直等。
- 没有取消旧回合；attempt 只有字段没有行为。
- round 在 gateway 内存，单实例有效。
- gateway 全局 RoundAllocator 会串行所有会话。
- 没有 conversation 归属校验。
- 没有 metrics / tracing；Kafka 二进制排查要解码。
- 本机运行现状（2026-11-27）：
  - Docker daemon 没启动；
  - 8080 被 nginx 占用；
  - ai-service 不在 dev docker-compose；
  - chat/ai 两个 Dockerfile 都缺 proto 生成代码；
  - 根目录没有 .env；
  - 旧 Kafka topic 里可能有 JSON 消息，需要清掉或换 topic。

## 7. 下一阶段方案：独立 storage 服务（已定，不要改成 ai_service 直连）

核心原则：

- storage 是独立微服务，同时拥有 Mongo 和 Redis。
- Mongo 是唯一真相源。
- Redis 只缓存最近历史，有 TTL。
- ai_service 不直接连 Mongo/Redis，只调 gRPC。
- Kafka 只传当前消息，不传历史，消息不膨胀。
- step 流设计完全不动。

建议 storage 用 Go 实现，因为 gRPC 高并发、低延迟，和 gateway 技术栈一致，Mongo/Redis 的 Go 驱动成熟。如果坚持 Python 也可以，但 gRPC API 不变。

### 7.1 gRPC

service chant.storage.v1.HistoryService，端口 50053。

方法：

- AppendUserMessage(conversation_id, round, user)
  - 写 Mongo，status=pending。
- GetRecentMessages(conversation_id, limit)
  - 只返回 completed 历史。
  - Redis 命中直接返回；miss 查 Mongo 并回填。
- CompleteTurn(conversation_id, round, assistant)
  - 写 AI 回复，status=completed，更新 Redis。
- FailTurn(conversation_id, round, error)
  - 标记失败，避免半轮。
- Health
  - 检查 Mongo ping、副本集状态、Redis ping。

### 7.2 Mongo

- DB：chat
- 集合：conversation_turns
- 一轮一条文档：

    {
      "conversation_id": "conv_20261127_001",
      "round": 1,
      "status": "completed",
      "user": {
        "role": "user",
        "sender_id": "u_1001",
        "message_id": "m_20261127_001",
        "parts": [{ "type": "text", "text": "现在几点了？" }],
        "created_at_ms": 1795773021000
      },
      "assistant": {
        "role": "assistant",
        "sender_id": "ai-assistant",
        "message_id": "ai-m_20261127_001",
        "parts": [{ "type": "text", "text": "现在是 2026年11月27日 17:50:22（北京时间）。" }],
        "created_at_ms": 1795773026000
      },
      "error": "",
      "created_at_ms": 1795773021000,
      "updated_at_ms": 1795773026000
    }

索引：

- unique (conversation_id, round)
- (conversation_id, status, round)

### 7.3 Redis

- key：chat:history:{conversation_id}
- list，最多 100 条，TTL 1800 秒。
- 只缓存 completed 的消息。
- 写：RPUSH + LTRIM -100 -1 + EXPIRE，用 Lua / pipeline 原子。
- 读：LRANGE；条数不够请求的 limit 时回源 Mongo。

### 7.4 ai_service 每轮顺序

1. 解析 MessageSent parts。
2. gRPC AppendUserMessage（pending）。
   - 失败：重试；仍失败 → 发 failed RESULT，停止，不调 LLM。
3. gRPC GetRecentMessages(limit=20)。
   - 失败：记录 metric，降级为空历史。
4. assemble_context(history, current_text)。
5. LLM。
6. gRPC CompleteTurn(assistant)。
   - 失败：重试/记录，不阻塞最终回答。
7. 发 RESULT step。
8. LLM 异常：FailTurn，避免半轮。

### 7.5 一致性

- 先写 Mongo，后写 Redis。
- Mongo 成功、Redis 失败：DEL key，仍返回 success。
- Redis 挂：读 Mongo，写 Mongo，跳过缓存。
- Mongo 挂：写失败，不把 Redis 当真相源。
- 不追求强一致 / 2PC；目标是最终一致 + 正常路径读己之写。
- AppendUserMessage / CompleteTurn 必须幂等，靠 (conversation_id, round) 唯一索引。
- GetRecentMessages 默认按 round / created_at_ms 升序返回，供 assemble_context 使用。

## 8. 文件路径

已有：

- proto/chant/chat/v1/event.proto
- proto/gen/go/chant/chat/v1/
- proto/gen/python/chant/chat/v1/
- services/chat_service/
- services/ai_service/
- front_code/
- docs/message-chain.md
- docs/proto-turn-fields.md

需要新增：

- proto/chant/storage/v1/history.proto
- proto/gen/go/chant/storage/v1/
- proto/gen/python/chant/storage/v1/
- services/storage_service/cmd/server/main.go
- services/storage_service/app/config/config.go
- services/storage_service/app/history/model.go
- services/storage_service/app/history/repository.go
- services/storage_service/app/history/mongo_repository.go
- services/storage_service/app/history/redis_cache.go
- services/storage_service/app/history/service.go
- services/storage_service/app/server/grpc.go
- services/storage_service/config.yaml
- services/storage_service/Dockerfile
- services/storage_service/go.mod
- services/ai_service/infra/storage/client.py
- services/ai_service/config/settings.py（新增 storage_grpc_addr）
- services/ai_service/apps/chat/service.py（接 history）
- services/ai_service/tests/test_storage_client.py
- docker-compose.yml / docker-compose.prod.yml（新增 storage-service）
- loadtest/k6/

## 9. 实施顺序

1. 写 storage proto + 生成 Go/Python。
2. storage_service 骨架 + Mongo repository + Health。
3. Redis cache。
4. storage gRPC + 单测（幂等、缓存命中、miss）。
5. ai_service gRPC client + handle_message 接入。
6. Dockerfile / docker-compose / .env。
7. 本地跑通一轮：发“你好”，第二轮能看到第一轮。
8. 压测：
   - Mongo-only 基线
   - 加 Redis cache
   - storage 1/2/4 实例
   - 报告 QPS / p99 / 缓存命中率 / Mongo 复制延迟

## 10. 工作约束

- 不要恢复 AiReplyDelta / AiReplyGenerated。
- 不要做 token 流式。
- 不要把历史塞进 MessageSent.parts；历史由 storage 管理。
- 不要让 ai_service 直连 Mongo/Redis。
- 不要改 step 协议。
- 图片只保留结构；前端选图/多模态等 storage 链路跑通后再做。
- 不要输出或提交任何密钥、公网 IP、内网地址。
- 改代码后至少跑验证命令。

## 11. 验证命令

    cd proto
    buf lint

    cd ../services/ai_service
    .venv/bin/python -m pytest tests -q

    cd ../chat_service
    GOCACHE=$PWD/.gocache go test ./...
    GOCACHE=$PWD/.gocache go build ./...

    cd ../../front_code
    npm run build

## 12. 第一步

先读：

- docs/message-chain.md
- docs/proto-turn-fields.md
- proto/chant/chat/v1/event.proto
- services/ai_service/runtime/agent_loop/loop.py
- services/ai_service/apps/chat/service.py
- services/chat_service/app/services/round_allocator.go
- front_code/src/pages/ChatPage.tsx

确认 main 状态后，创建：

- proto/chant/storage/v1/history.proto
- services/storage_service/ 骨架

然后按第 7、9 节实现。不要先改前端图片。

## 13. 常见坑

- MessageSent 的 target_user_id 是 AI 身份，不是 Kafka topic。
- MessageSent.sender_id 是用户；AiTurnStep.sender_id 是 AI。
- proto 里没有 kind；WS 才有 kind。
- proto 里没有 content；MessageSent 只有 parts。
- 文本 part 的 data 是空 bytes，不是 null。
- AiTurnStep 的 event_id 是类型-UUID；MessageSent 的 event_id 被覆盖成用户 message_id。
- status 在 proto 是枚举，在 WS 是小写字符串。
- Kafka 是二进制；旧 JSON 消息和新消费者不兼容，压测前清 topic。
- ai_service 的 history 当前恒为空，这是 storage 要解决的问题。
