# Chant 项目逐句精读计划 v2（函数级 · 可勾选清单）

> **目标**：逐句读完整个项目，**读懂 + 能复述**，面试达到初中级开发水平。
> **用法**：按顺序一节一节读；每节结构固定为 ① 前置概念（读代码前先懂）→ ② 文件与函数指引（按序精读）→ ③ 目标/重难点 → ④ 自测题（只给题目，读完先自测）→ ⑤ 验收提示词（把该段复制发给 AI 当考官，考完让它点评）。
> **节奏**：弹性安排。每节约 1~2 小时精读量；带 ⚠️ 的章节信息量大，可拆 2~3 次读完。
> **版本说明**：v2 基于 2026-08 当前工作区代码（前端组件已重构为 ChatInput/MessageList/Sidebar 等；proto 新增 AiReplyDelta 流式事件；ai_service 记忆实现为 chat/memory/context.py）。若后续代码有变，以代码为准。

---

## 项目速览（开工前先读 3 分钟）

**这是什么**：Chant 是一个「AI 聊天 + 智能体研究」平台（类 ChatGPT 的群聊/私聊 + 联网研究型 Agent），个人全栈作品。核心故事：用户发消息 → chat_service 落库并广播到 Kafka → ai_service 消费（普通聊天或跑"研究图"）→ 回复经 Kafka 回推 → chat_service 落库 + WebSocket 实时推送（含流式思考过程）。另有独立的 Saga 编排器负责跨服务分布式事务。

**技术栈一览**：

| 层 | 技术 |
| --- | --- |
| 后端 | Go 1.26（chat / orchestrator）、Python 3.11/3.13（auth / ai） |
| 前端 | React 19 + TypeScript + Vite（WebSocket 实时聊天） |
| 消息 | Kafka（protobuf 事件信封 + 旧 JSON 双格式并存） |
| 存储 | MongoDB（消息/用户）、PostgreSQL（群组/会话/Saga/成本）、Redis（验证码/缓存）、Qdrant（向量记忆） |
| AI | LangGraph 状态图、DeepSeek/OpenAI 兼容 LLM、embedding+rerank、SearXNG/Wikipedia/Wikidata 检索 |
| 通信 | WebSocket（读写双泵）、gRPC（跨服务鉴权/离线时间）、REST |
| 部署 | Docker Compose（4 种拓扑）、nginx（WS 反代/负载均衡）、systemd |

**关键词**：微服务 · 事件驱动 · Kafka 分区保序 · 幂等写入 · WebSocket 双泵 · JWT 双 token · gRPC 远程鉴权 · LangGraph 多智能体 · RAG 混合检索 · 流式 delta 协议 · Saga 状态机 · 乐观锁/租约锁 · SSRF 防护 · 成本追踪

**重点要学什么**（初中级面试定位）：
1. **Go 并发模型**：channel / goroutine / 锁的最佳教材在 3.2（WS 双泵 + Hub 单协程模式）
2. **消息链路的可靠性**：分区键保序、$ne 幂等、DLQ、消费组（2.1 / 3.3 / 3.4）
3. **认证体系**：JWT 双 token + 验证码三层防护 + gRPC 远程校验（4.x）
4. **分布式事务**：Saga 状态机 + 逆序补偿 + 乐观锁 + 租约锁（6.2）
5. **AI Agent 工程**：研究图编排、工具调用、记忆系统、成本控制（5.x）

**难点地图**（读到这些章节时放慢、可拆多次）：
- 🔴 **5.6 Agent 研究图**——最复杂：九节点 + 审核-修订循环 + 引用编号机制
- 🔴 **5.3 记忆折叠** + **5.5 混合检索**——概念新、细节多
- 🔴 **6.2 Saga**——分布式事务概念最重（🔍 该模块未实际部署，仅作原理学习，见 6.1 定位说明）
- 🟡 **3.4 MongoDB 分桶与幂等**——细节最多、含静默丢消息的坑

**精彩之处**（读到这里值得停下来品味）：
- "代码做确定性路由，LLM 做语义填充"——方法论映射、引用清洗用纯代码，语义理解才用 LLM（5.6）
- finalize 节点零 LLM，自动审计引用、清洗伪造参考文献（5.6）
- 前端流式 delta 拼装 + Kafka 重投去重 + 乐观渲染（7.3）
- 每机器独立消费组实现多实例横向扩展（3.1）
- 装饰器工厂注册 LLM provider，新增模型三步搞定（5.2）
- 文末附 13 处真实代码坑位——"主动讲问题"是面试加分项

---

## 〇、阅读顺序的设计逻辑

```
契约层(proto) ──► 基础设施(SDK) ──► chat_service ──► auth_service
                                          │
                                          ▼
                                     ai_service(最深) ──► orchestrator(Saga)
                                          │
                                          ▼
                                  前端(不熟,放慢) ──► 部署(不熟,放慢)
```

1. **契约先行**：proto 是全局"词典"，所有服务都按它说话。先读它，后面每个文件里的字段你都认识。
2. **基础设施次之**：Kafka SDK 是"邮局"——先知道消息怎么封装、怎么发收，再读业务。
3. **chat_service 第三**：消息链路入口（前端 → chat → Kafka → AI），读完你就有全局坐标系。
4. **auth_service 第四**：chat 的 WS 鉴权依赖它，先当黑盒，读完 chat 再回来拆。
5. **ai_service 最深，放中后段**：研究图 + 流式 delta + 长期记忆是项目最复杂的部分。
6. **orchestrator 独立**：Saga 与聊天主链路无关，放最后。
7. **前端、部署放最后且放慢**：从基础概念补起，能看懂即可。

---

## 一、全局地图（一页总表）

| 节 | 主题 | 核心文件 | 精读量 | 完成 |
|---|---|---|---|---|
| 0.1 | 项目地图与链路文档 | docs/ 全部 | ~600 行 | ☐ |
| 0.2 | Go / Python 语法热身 | 附录 C 清单 | 自查 | ☐ |
| 1.1 | proto 契约层 | proto/chant/** | ~200 行 | ☐ |
| 2.1 | 基础设施 SDK | infrastructure_sdk/** | ~1900 行 | ☐ |
| 3.1 | chat 入口与配置 | main.go, config/** | ~350 行 | ☐ |
| 3.2 | WebSocket 实时层 | ws_handler/hub/connection | ~500 行 | ☐ |
| 3.3 | 消息处理与 Kafka | chat_message_service, kafka/** | ~600 行 | ☐ |
| 3.4 | MongoDB 消息存储 | mongodb/** | ~1300 行 | ☐ |
| 3.5 | 鉴权中间件与 REST | middleware, router, handlers, pgsql, redis | ~800 行 | ☐ |
| 4.1 | auth 入口与存储 | main.py, core/**, database/** | ~700 行 | ☐ |
| 4.2 | 认证业务与 gRPC | auth.py, jwt_service, grpc/** | ~600 行 | ☐ |
| 5.1 | ai 入口与消息总线 | main.py, settings, kafka, proto_adapter | ~500 行 | ☐ |
| 5.2 | LLM 抽象层 | shared/llm/** | ~500 行 | ☐ |
| 5.3 | chat 模式与记忆折叠 | chat/**, memory/context.py ⚠️ | ~600 行 | ☐ |
| 5.4 | Embedding 与 Qdrant | embedding/**, qdrant/** | ~400 行 | ☐ |
| 5.5 | Agent 工具与长期记忆 | agent/tools/** ⚠️ | ~800 行 | ☐ |
| 5.6 | Agent 研究图 | agent/graphs/**, agent/service.py ⚠️ | ~1200 行 | ☐ |
| 6.1 | orchestrator 入口与消费 🔍 | main.go, kafka/** | ~400 行 | ☐ |
| 6.2 | Saga 状态机与补偿 🔍 | saga/**, 核心 handlers | ~900 行 | ☐ |
| 7.1 | 前端骨架 | package.json, vite.config, main/App | ~300 行 | ☐ |
| 7.2 | 登录注册与请求封装 | utils/**, api/**, Login/Register | ~500 行 | ☐ |
| 7.3 | 聊天页与组件 | ChatPage.tsx, components/**, hooks/** ⚠️ | ~900 行 | ☐ |
| 8.1 | Docker 编排 | docker-compose*.yml, Dockerfile×4 | ~800 行 | ☐ |
| 8.2 | nginx 与生产部署 | deploy/**, .deploy/** | ~700 行 | ☐ |

---

## 二、逐节详情

---

### 0.1 项目地图与链路文档

**⏱ 前置概念**
- 事件驱动架构：服务间不直接调用，而是"发事件到 Kafka，谁关心谁消费"（消息闭环见下）。
- 六种存储分工：Kafka=事件总线、MongoDB=消息/用户、PostgreSQL=群组/会话/Saga/成本、Redis=验证码/离线时间缓存、Qdrant=AI 长期记忆向量、SearXNG=联网搜索。

**📄 文件与阅读要点**
- `docs/README.md`：链路总览图 + 配置文件布局表。先看图，再背布局。
- `docs/message-chain.md`：WS 上行/下行 JSON 字段、Kafka 事件字段、Mongo 文档字段。**这是全项目的字段字典，建议读 2 遍**。
- `docs/storage-architecture.md`：六种存储的读写方、分桶规则、幂等约定、生产双 MongoDB。
- `docs/reading-plan.md`：本文件，先通读一遍结构再开工。
- 根目录 `.env.example`：生产环境变量全集（资源限制 MEMORY_LIMIT_*、LLM 配置、KAFKA_EXTERNAL_HOST）。
- `go.work`：Go 多模块工作区，use 了 orchestrator/chat/infrastructure_sdk/proto-gen。

**🎯 目标**：能徒手画出全项目架构图（4 服务 + 6 存储 + 消息闭环），背出每存储"谁写谁读"。

**🔑 重难点**
1. 消息闭环：前端 WS → chat_service 落库+发 Kafka → ai_service 消费 → 回复回 Kafka → chat_service 落库+WS 推送（群聊与私聊两条支线）。
2. 分区键规则：群聊按群 ID 奇偶分区、私聊/AI 回复按目标用户 ID——保证同会话有序（读 message-chain 时对着代码想一遍）。
3. 新协议要点：AiReplyDelta 流式分块（kind=thinking/progress/content/done）是 v2 新增，docs 若未更新以 proto 为准。

**📝 自测题**
1. 一条群聊消息从发送到 AI 回复回推，经过哪几个服务和哪些存储？
2. 为什么 AI 回复也要按目标用户 ID 做分区键？
3. 生产环境为什么用双 MongoDB？

**🗣 验收提示词**（读完复制发给 AI）
> 你是 Chant 项目的陪练考官。我已逐句读完 docs 三篇文档和 .env.example、go.work。请考核我：① 让我默画完整消息闭环并逐段讲解；② 考我 6 种存储的职责与读写方；③ 考我 Kafka 分区键规则及其目的；④ 追问 AiReplyDelta 流式协议的设计意图。每道题等我回答后指出错误与含糊处。

- 完成：☐ 日期：____

---

### 0.2 Go / Python 语法热身（面试初中级重点）

**⏱ 前置概念**：无（本节是自查）。

**📄 无文件**——对照附录 C 清单自查。项目里实际用到的语法点，逐条确认"能看懂 + 能写"。

**🎯 目标**：读代码时不被语法卡住，注意力留给业务逻辑。

**🔑 重难点（Go）**
1. goroutine + channel：`go func(){}`、`make(chan T, n)`、`select`、`<-ch` / `ch <- x`、close 与 range。
2. `sync.Mutex`/`sync.RWMutex`/`sync.Once`/`sync.WaitGroup` 的适用场景（ws_hub 里全都有）。
3. `defer`、`interface{}`/`any`、类型断言 `x.(T)` 与逗号 ok、指针 vs 值接收者。
4. json 标签 `json:"..."` `omitempty`、`context.WithTimeout`。

**🔑 重难点（Python）**
1. `async def`/`await`/`asyncio` 事件循环；`asyncio.gather`、`asyncio.to_thread`、`asyncio.create_task`。
2. 装饰器（`@register("deepseek")` 装饰器工厂）、`@asynccontextmanager`、`@retry`。
3. TypedDict / dataclass / 类型注解 / `Optional`；dict 解包与 get 默认值。
4. pydantic-settings 的环境变量映射（大写字段名）。

**🗣 验收提示词**（对照附录 C 自查后发给 AI）
> 你是 Go 与 Python 语法陪练。我将逐句阅读一个 Go+Python 微服务项目，目标是初中级开发面试。请用项目实战风格考我：① Go channel/select/goroutine 基础题 5 道；② Go Mutex/RWMutex/sync.Once 场景题 3 道；③ Python asyncio 事件循环与 await 语义 4 道；④ Python 装饰器原理 2 道。每题先让我答，再讲解。

- 完成：☐ 日期：____

---

### 1.1 proto 契约层（全局词典）

**⏱ 前置概念**
- proto3 语法：所有字段有默认值（int64 默认 0、string 默认 ""），字段编号不可变（改了会破坏兼容）；`oneof` 表示多选一；`bytes` 可装任意字节。
- buf 工具链：`proto/buf.yaml` 管 lint/breaking 规则，`buf.gen.yaml` 管生成（managed mode 统一 go_package 前缀，生成到 proto/gen/go 与 proto/gen/python）。
- protojson：用 JSON 表示 proto 消息；Kafka 里信封整体是 protojson **文本**，不是二进制。

**📄 文件与函数指引**（无函数，纯消息定义，按此顺序精读）
- `proto/chant/common/v1/envelope.proto`：`EventEnvelope`（9-22 行）6 字段——event_id/event_type/source/timestamp/trace_id/data(bytes)。要点：data 里实际装的是内层事件的 **protojson 文本**；event_type 是全限定名（如 "chant.chat.v1.MessageSent"），消费端靠它路由；timestamp 是 Unix 毫秒。
- `proto/chant/chat/v1/event.proto`（核心，背下来）：
  - `MessageSent`（10-29 行）：sender_id/target_user_id/group_id/content/message_id/message_type/conversation_type/timestamp_ms/metadata。要点：私聊填 target_user_id、群聊填 group_id；timestamp_ms 由 **ai_service** 填写（与信封 timestamp 不同源）；metadata 是 map<string,string>，新字段不改 proto。
  - `AiReplyGenerated`（34-53 行）：AI 回复终态事件；reply_to_msg_id 关联原消息；message_id 与 delta 流共用。
  - `AiReplyDelta`（68-89 行）：流式分块；kind ∈ thinking/progress/content/done；seq 从 0 递增；**仅 WS 转发不落库**。
- `proto/chant/chat/v1/ws.proto`：`WsIncoming`（8-16 行）type="chat"/"ping" + oneof payload；`ChatContent` 的 conversation_type 取 "group"/"ai"（注意与 event.proto 的 "private"/"group" 语义不完全一致，跨协议对比容易混）。
- `infrastructure_sdk/grpc/**/proto/*.proto`（两个）：token_auth（VerifyToken/GetUserByID）与 last_offline_time（Update/Get），供 chat_service ↔ auth_service 远程调用。

**🎯 目标**：默写 EventEnvelope 6 字段、MessageSent 9 字段、AiReplyDelta 4 种 kind，说出每个字段谁填、给谁用。

**🔑 重难点**
1. 信封模式的价值：data 字节 + event_type 路由 = 新增事件类型不用改信封。
2. 扩展字段 metadata 的设计意图（报告类型/域名/方法论/摘要都走它）。
3. AiReplyDelta 与 AiReplyGenerated 的协作：done 之后必有终态事件；message_id 一致用于前端累积。
4. proto3 的坑：int64 默认 0 无法区分"没填"与"填了 0"；字段编号重排 = 灾难。

**📝 自测题**
1. EventEnvelope 的 data 字段类型是什么？里面装的是什么格式的数据？
2. MessageSent 里 group_id 与 target_user_id 是同时存在还是互斥？conversation_type 有哪几种取值？
3. AiReplyDelta 的 kind 有哪四种？前端靠什么字段把分块拼回一条回复？
4. 为什么新增 metadata 字段不需要改 proto？这解决了什么问题？
5. ws.proto 的 conversation_type 与 event.proto 的取值差异在哪？

**🗣 验收提示词**
> 你是 Chant 项目的陪练考官。我已逐句读完 proto 契约层全部文件与 buf 配置。请考核我：① 默写 EventEnvelope 字段并解释为什么 data 用 bytes 而不是嵌套 message；② MessageSent 每个字段的生产者/消费者；③ AiReplyDelta 的四种 kind 与前端累积机制；④ metadata 扩展设计好在哪；⑤ proto3 默认值与字段编号的坑；⑥ 两个 gRPC 服务的用途。逐题讲解。

- 完成：☐ 日期：____

---

### 2.1 基础设施 SDK（Kafka 邮局）

**⏱ 前置概念**
- Kafka 分区与 Key：producer 按 key 哈希选分区；**同 key 同分区 = 有序**；无 key 则轮询。
- 消费者组与 offset：组内分区互斥消费（同组多实例分工）；offset 提交时机决定"崩溃后从哪续读"——先提交后处理会丢消息，先处理后提交会重复消费。
- 指数退避重试与死信队列（DLQ）：重试耗尽的失败消息投 DLQ，避免无限重试卡死消费。
- 本 SDK **同时存在新旧两套事件格式**：旧 BusinessEvent（JSON，orchestrator 用）与新 EventEnvelope（proto，chat/ai 用）——读的时候注意区分。

**📄 文件与函数指引**（按此顺序）
- `infrastructure_sdk/config/loader.go`：`LoadConfig(path, target)`（11-28 行）三步走——Stat 判存在 → ReadFile → yaml.Unmarshal；错误 %w 包装；target 必须是可写指针。
- `infrastructure_sdk/kafka/model.go`：`NewBusinessEvent`（25-46 行）——[]byte 直接复用为 json.RawMessage（避免二次 marshal 变 base64）；Timestamp 用 RFC3339 字符串（旧格式）；EventID 由调用方传。
- `infrastructure_sdk/kafka/envelope.go`（新格式核心）：
  - `NewEventEnvelope`（28-41 行）：event_id = "{eventType}-{UnixNano}"（不保证全局唯一，只用于去重）；marshalOpts 用 snake_case 字段名。
  - `SendEnvelope`（44-50 行）：protojson 文本发送。
  - `ParseEnvelope`（53-59 行）：unmarshalOpts 带 DiscardUnknown=true（旧数据多字段不报错）。
  - `StartProto`（71-117 行）：消费主循环；解析失败直接提交 offset 跳过；handler 失败最多 3 次、退避 1s/2s/4s；**重试耗尽仅记日志、不投 DLQ、照常提交 offset**（与 consumer.go 的 executeWithRetry 行为不同！）。
- `infrastructure_sdk/kafka/producer.go`：`writeMessage`（38-62 行）——topic 空报错；key 空则不设 msg.Key；每条消息 5 秒超时 context。
- `infrastructure_sdk/kafka/consumer.go`（旧格式）：`Start`（103-134 行）——"失败也提交、跳过避免阻塞队列"是显式设计决策；`executeWithRetry`（137-170 行）i 从 0 到 maxRetries 共 4 次尝试，全败投 DLQ；`performBackoff`（53-68 行）指数退避 1s/2s/4s；`sendToDLQ`（173-188 行）外层 EventType 是 "sys.dlq.message"，key 复用原 EventID。
- `infrastructure_sdk/kafka/kafka_manager.go`：`NewKafkaConnection`（17-55 行）——Reader MinBytes 10KB/MaxBytes 10MB/CommitInterval 1s/StartOffset **LastOffset**；Writer BatchSize=1（立即发）、RequiredAcks=RequireAll、Async=false（同步）、**Topic 不设在 Writer 上**（逐消息设置）。
- `infrastructure_sdk/kafka/dlq.go`：`NewDLQPayload`（18-25 行）+ 常量 "sys.dlq.message"。

**🎯 目标**：说清一条消息从业务结构体到 Kafka 字节再到对端业务结构体的完整路径；新旧两套格式的区别；DLQ 何时触发。

**🔑 重难点**
1. 序列化链路：struct → protojson → EventEnvelope{data} → Kafka bytes（消费端反向）。
2. **两套格式的行为差异**：StartProto 重试耗尽不投 DLQ，Start（旧）会投——面试可讲点。
3. offset 提交策略："处理失败也提交" vs "先处理后提交"的取舍。
4. Writer 配置：RequiredAcks=RequireAll + Async=false = 强一致但低吞吐。
5. Go 并发：manager 里 Reader/Writer 的创建与 Close。

**📝 自测题**
1. ParseEnvelope 失败时 StartProto 做了什么？与 handler 重试耗尽时的处理有何不同（是否提交 offset、是否投 DLQ）？
2. NewEventEnvelope 的 event_id 格式是什么？保证全局唯一吗？
3. kafka_manager 里 Writer 为什么不在 WriterConfig 配 Topic？BatchSize=1 与 RequiredAcks=RequireAll 各意味着什么？
4. 同 key 消息为什么有序？无 key 时会怎样？
5. marshalOpts UseProtoNames=true 对 Kafka 消息字段名形状有何影响？DiscardUnknown 解决什么问题？

**🗣 验收提示词**
> 你是 Chant 项目的陪练考官。我已逐句读完 infrastructure_sdk 的 config 与 kafka 全部文件。请考核我：① 描述消息从发送到接收的完整序列化链路；② 新旧两套事件格式的区别与各自消费者；③ 分区键与消息顺序的关系；④ 消费失败与 DLQ 的完整流程；⑤ offset 提交时机选错会丢消息还是重复消费；⑥ 指数退避的实现细节。逐题讲解。

- 完成：☐ 日期：____

---
### 3.1 chat_service 入口与配置

**⏱ 前置概念**
- Go 服务启动模式：main 里按依赖顺序初始化（配置 → 数据库 → Kafka → 路由 → 消费者），最后阻塞等信号做优雅退出。
- 每机器独立消费组：Kafka GroupID 拼接 hostname，使每台实例都消费 topic 全量消息、各自做本地 WS 广播——多实例横向扩展的关键设计。

**📄 文件与函数指引**（按此顺序）
- `services/chat_service/main.go`：
  - `main`(106-203)：初始化顺序固定为 config.InitConfig → initPostgreSQL → initMongoDB → initMessageService → 注册 3 个本地广播回调 → consumer runner goroutine → HTTP；**消费者 goroutine 是"死循环 + 3 秒重启"模式**（Run 返回错误就 sleep 3s 重跑，唯一退出条件是 ctx.Done()）；最后 `<-sigchan` 阻塞，收到 SIGINT/SIGTERM 后 cancel()。
  - `initPostgreSQL`(27-40)：注释明确"不要 close，因为是长连接"；`dbManager.Initialize()` 建表。
  - `initMongoDB`(43-48)：只 Connect 不建库（driver 懒创建）。
  - `initMessageService`(51-65)：创建 Kafka 连接 + `kafka.NewKafkaProducer(conn, topic)`（内部设包级单例）。
  - `createApp`(68-89)：gin.Engine + routes.NewRouter().SetupRoutes()（含 WS 路由与鉴权中间件）。
  - `startServer`(92-104)：`TLSNextProto: make(map[...])` 是禁用 HTTP/2 的惯用法。
- `services/chat_service/config.yaml`：kafka.topic=chat_group_message（**群/私聊/AI 共用一个 topic，靠 EventType 区分**）；kafka.group_id 会被追加 hostname；internal_api_key=chant-internal-2026（ai_service 内部调用密钥）；postgres db_name=orchestrator（与其他服务共用库）。
- `services/chat_service/app/config/config.go`：
  - `InitConfig`(81-132)：yaml 解析到全局变量；每个地址过 `subEnv` 支持环境变量覆盖；**GroupID 逻辑（113-124 行）**：优先 CHAT_GROUP_ID 环境变量，未设置则用 `group_id + "_" + hostname`。
  - `subEnv`(135-139)：`os.Getenv` 非空才覆盖（空视为未设置）。
- `services/chat_service/app/const/message_type.go`：MessageType（text/image/file/voice/video）与 ConversationType（private/group/ai/ai-research）两组常量；ai/ai-research 在 HandleChat 中按群聊路径处理。
- `services/chat_service/app/api/models/ws_message.go`：IncomingMessage 外层信封（type+content）；ChatMessageRequest 的 sender_id 带 omitempty 但**实际以登录态为准**；conversation_id 仅 AI 会话用。

**🎯 目标**：说清启动初始化顺序、每个组件用途、消费者崩溃重启机制、GroupID 后缀设计。

**🔑 重难点**
1. 消费者"死循环+3 秒重启"模式：为什么能自愈？为什么主动 cancel 时不重启（consumer_runner.Run 检查 ctx.Err 返回 nil）？
2. GroupID = group_id + hostname 的设计：所有实例都消费全量消息 → 每台各自广播给本地 WS 连接（无跨机路由）。
3. 回调注入解耦：main.go 用 RegisterXxxBroadcast 把"查群成员+Hub 广播"逻辑注入 kafka 包，避免 kafka 包反向依赖 services 包（3.3 节会用到）。

**📝 自测题**
1. main.go 中 Kafka 消费者崩溃后间隔多久重启？goroutine 真正退出的两个条件？
2. 最终 Kafka GroupID 取值规则？CHAT_GROUP_ID 与 hostname 后缀分别解决什么问题？为什么不能所有实例共用同一 group_id？
3. config.yaml 里 topic 只有一个，群聊/私聊/AI 消息怎么区分？

**🗣 验收提示词**
> 你是 Chant 项目的陪练考官。我已逐句读完 chat_service 的 main.go、config.yaml、config.go、const、ws_message.go。请考核我：① 启动初始化顺序及各组件用途；② 消费者崩溃重启机制与退出条件；③ GroupID+hostname 的设计意图；④ 回调注入解耦模式；⑤ conversation_type 四常量与 message_type 的关系。逐题讲解。

- 完成：☐ 日期：____

---

### 3.2 WebSocket 实时层（Go 并发核心）

**⏱ 前置概念**
- gorilla/websocket 的并发约束：底层连接**禁止并发写**（一次只能有一个 goroutine 在写），读也只能有一个 goroutine——所以必须拆成读写两个循环。
- "通过通信共享内存"：Hub 主循环单 goroutine 串行消费 channel，外部不直接碰 map——这是 Go 并发经典模式（勿用锁直接操作共享 map）。
- ping/pong 心跳：服务端定时发 Ping（周期 < 客户端读超时），客户端回 Pong 刷新读 deadline，防止死连接。

**📄 文件与函数指引**
- `app/api/websocket/ws_handler.go`：
  - `HandleWebSocket`(18-97)：①`c.MustGet("userInfo")` 强依赖鉴权中间件注入（缺失会 panic）；②升级后 `SetReadLimit(64*1024)` 防超大帧；③顺序：NewWSConnection → GetWSHub → NewWSClient → Register → `go client.WritePump()` → 发欢迎消息 → ReadLoop(回调)（阻塞到断开）；④断开后 Unregister + 独立 goroutine 用 5 秒超时 context 调 gRPC UpdateLastOfflineTime。
  - ReadLoop 回调(55-74)：json.Unmarshal 失败仅记日志 return nil（不断开）；按 type 分发：chat → HandleChat、ping → 只打日志；未知类型 default 日志。
- `app/services/ws_hub.go`（核心）：
  - `GetWSHub`(67-77)：sync.Once 懒加载单例 + `go hubInstance.Run()`。
  - `NewWSClient`(57-64)：`Send` 通道缓冲 256 条防阻塞。
  - `WritePump`(81-109)：select 监听 Send 与 pingPeriod=50s ticker；Send 被 close 发 Close 帧退出；写失败即 return，defer 关连接。
  - `Run`(113-122)：Hub 主循环，只消费 register/unregister 两 channel 串行执行。
  - `Register`/`Unregister`(125-132)：无缓冲 channel 投递。
  - `BroadcastToUser`(136-152)：先 RLock 查 map；在线则 select 非阻塞写；**通道满走 default——异步 Unregister 强制断开该用户，保护 Hub 不被阻塞**。
  - `KickUser`(183-195)：写锁删除 + close(Send) 触发 WritePump 退出。
  - `registerClient`(198-211)：**单端登录策略**——同 userID 已有旧连接先 close 旧 Send 踢掉再覆盖。
  - `unregisterClient`(214-224)：`currentClient == client` 指针相等才删除——防快速重连时旧连接晚到的注销误删新连接。
- `app/services/ws_connection.go`：
  - `WSUpgrader`(16-31)：CheckOrigin 读 WSAllowedOrigins 白名单；Origin 为空（非浏览器）放行。
  - `WriteMessage`(49-53)：sync.Mutex 串行化底层写（所有下行写必须过锁）。
  - `ReadLoop`(56-86)：SetPongHandler 刷新 deadline；每轮 SetReadDeadline(now+60s)；`IsUnexpectedCloseError` 区分正常/异常关闭；回调返回 error 即 break。

**🎯 目标**：画出"一个客户端连接的一生"；说出 Hub 用 channel 而非锁的原因；解释单端登录与快速重连防误删。

**🔑 重难点**
1. 读写双泵为什么必须分离（gorilla 禁止并发读写）。
2. Hub 的 register/unregister channel + 单协程模式与直接加锁的对比。
3. 心跳参数关系：pingPeriod(50s) < pongWait(60s)——违背则连接被误杀或僵死。
4. BroadcastToUser 通道满时的"牺牲单个用户保 Hub"策略。
5. registerClient/unregisterClient 的两个防竞态细节（踢旧连、指针相等判断）。

**📝 自测题**
1. WSClient.Send 通道缓冲多大？BroadcastToUser 发现通道满时执行什么？保护什么？
2. registerClient 实现什么登录策略？旧连接 WritePump 如何感知退出？
3. unregisterClient 的 `currentClient == client` 针对什么场景？去掉会怎样？
4. pingPeriod 与 pongWait 各是多少？必须满足什么关系？违背的后果？
5. ReadLoop 中回调返回 error 时发生什么？json 解析失败时连接断不断？
6. CheckOrigin 对无 Origin 请求返回什么？ws_handler 的 MustGet("userInfo") 前提是什么？

**🗣 验收提示词**
> 你是 Chant 项目的陪练考官。我已逐句读完 ws_handler.go、ws_hub.go、ws_connection.go。请考核我：① 读写双泵为什么必须分离；② Hub 为什么用 channel 而非直接加锁；③ 心跳时间参数关系；④ 断开清理流程（含 gRPC 上报）；⑤ 单端登录与快速重连防误删的实现细节；⑥ 通道满时的降级策略。逐题讲解。

- 完成：☐ 日期：____

---

### 3.3 消息处理与 Kafka 收发（主链路核心）

**⏱ 前置概念**
- 事件信封分发：消费端按 env.EventType 路由（MessageSent/AiReplyGenerated/AiReplyDelta 三种）。
- 分区键=顺序：群消息按群 ID 奇偶分区（同群有序）、私聊按目标用户 ID（同会话有序）、AI 回复按用户 ID（与私聊同分区保序）。
- 进度消息 vs 最终回复：kind=progress 的 AI 回复**不落库**，只实时 WS 转发。

**📄 文件与函数指引**
- `app/services/chat_message_service.go`（最重要）：
  - `getGroupPartitionKey`(20-27)：TrimPrefix("G") 后转 int 取奇偶；解析失败返回 "0"（保证同群同分区）。
  - `HandleChat`(32-120)：①空文本 return；②**sender 一律取登录态 userID，忽略客户端 sender_id**（防伪造）；③群/AI 分支必须先 IsUserInGroup 校验（防向任意群注入）；④落库失败仅记日志不阻断 Kafka；⑤私聊校验 TargetID 非空且非自己；⑥发 Kafka：群消息 key=partitionKey、私聊 key=TargetID。
- `app/services/conversation_service.go`：`GetConversationService`(21-26) 手写单例（**非 sync.Once，有并发竞态隐患**）；MarkMessageAsRead(36-65) 用 gorm gen 查询，ErrRecordNotFound 走 Create；UpdateLastReadTimeWhenOffline(109-117) 批量更新；ListConversations(140-162) 原生 SQL 联表过滤 ai_ 前缀群 ID；CreateAIConversation(165-184) 用 ai_+UnixNano 生成群 ID。
- `app/infrastructure/kafka/const.go`：WSMsgTypeChat="chat"、WSMsgTypePing="ping"。
- `app/infrastructure/kafka/event_producer.go`：`SendProtoEvent`(47-58)——NewEventEnvelope(eventType, "chat-service", msg)；若 msg 实现 GetMessageId() 且非空则覆盖 envelope.EventId；手动设 Timestamp 毫秒。
- `app/infrastructure/kafka/event_consumer.go`：
  - `HandleProtoEnvelope`(15-27)：支持三种 EventType；未知类型只打日志返回 nil（不阻塞消费）。
  - `handleAiReplyDelta`(30-37)：**只实时 WS 转发、不落库**。
  - `handleAiReply`(56-90)：组装 mongodb.Message（TimestampMs 取 proto、Timestamp 取本地）；**Metadata["kind"]=="progress" 不落库**；群聊 SaveMessage("ai",...)、私聊 SaveMessage("private",...)。
- `app/infrastructure/kafka/consumer_runner.go`：`Run`(26-51)——用含 hostname 的 GroupID 创建连接；ctx.Err()!=nil 返回 nil（正常退出不触发重启）。
- `app/infrastructure/kafka/services/event_handlers.go`：RegisterGroupMessageLocalBroadcast(25-27)/RegisterPrivateMessageLocalBroadcast(30-32)/RegisterAiReplyBroadcast(148-150) 注册回调；HandleGroupChatMessageEvent(35-62) 构造 type=group_chat JSON；HandlePrivateChatMessageEvent(65-91) conversation_id 用 GenerateSessionID；BroadcastAiReply(94-116) 含 reply_to_msg_id 与 metadata；BroadcastAiReplyDelta(119-143) 带 kind/seq。
- `app/infrastructure/grpc/auth_client.go`：GetAuthClient(32-41) sync.Once；地址优先 AUTH_GRPC_ADDR 默认 localhost:50051；**初始化失败返回 nil 会被缓存（once 已执行）**；VerifyToken(63-85) nil 保护返回 Valid:false 不 panic。
- `app/infrastructure/grpc/last_offline_time_client.go`：默认 localhost:50052；UpdateLastOfflineTime(63-83) nil 保护 Success:false。

**🎯 目标**：完整讲述"用户发一条群聊消息"发生的一切（校验→落库→Kafka→AI 回复回推→落库→WS 推送）。

**🔑 重难点**
1. 防伪造双保险：sender 取登录态 + 群成员校验（越权防护）。
2. 落库与发 Kafka 的一致性：落库失败只记日志不阻断（消息可能只在 Kafka 侧存在）——了解这个取舍。
3. 进度消息不落库：避免污染历史，只做实时推送。
4. 回调注入解耦：kafka 包不知道 Hub，main 注入"查群成员+广播"逻辑。
5. gRPC 客户端 nil 保护：失败不 panic 而是返回 Valid:false。

**📝 自测题**
1. HandleChat 的 sender 由谁决定？客户端传的 sender_id 如何处理？
2. 群消息/AI 消息发送前必须通过哪个函数校验？校验失败（含 DB 错误）时发生什么？
3. `getGroupPartitionKey("G12")`、`getGroupPartitionKey("abc")` 各返回什么？
4. HandleProtoEnvelope 支持哪几种 EventType？未知类型返回什么、对消费循环有何影响？
5. handleAiReply 中什么条件下 AI 回复不写 MongoDB？
6. GetAuthClient 初始化失败后 VerifyToken 会 panic 吗？返回什么？

**🗣 验收提示词**
> 你是 Chant 项目的陪练考官。我已逐句读完 chat_message_service.go、conversation_service.go、kafka 目录与 grpc client。请考核我：① 完整复述群聊消息从 WS 到 AI 回复回推的全链路；② sender 取登录态与群成员校验的双重防护；③ 分区键选择如何影响 AI 回复与用户消息的相对顺序；④ 进度消息不落库的设计；⑤ 回调注入如何解耦；⑥ gRPC nil 保护。逐题讲解。

- 完成：☐ 日期：____

---

### 3.4 MongoDB 消息存储（数据设计重点）

**⏱ 前置概念**
- 分桶设计：按月分 Mongo collection（xxx_history_YYYYMM）、按日分文档（date_identifier=YYYYMMDD）、群聊每桶上限 500 条——避免单文档无限膨胀。
- `$setOnInsert`：upsert 时只在文档不存在时写入字段；配合"追加不带 upsert"避免重复建桶。
- `$ne` + message_id 去重：追加时过滤已存在的 message_id，Kafka 重试/重启回放的幂等保护。
- `$elemMatch`：匹配文档内数组"至少一个元素满足条件"，配合转义后的 `$regex` 缩小待内存过滤的桶集合。

**📄 文件与函数指引**
- `app/database/mongodb/models.go`：`Message`(8-21)——PrivateID 与 GroupID 互斥；`GroupMessageHistory`(24-31)——Count 用于分桶上限（MaxMessagesPerBucket=500）；`PrivateMessageHistory`(34-38)——**没有 Count/StartTime/EndTime 字段**（私聊无分桶、单文档无限增长；但查询代码仍按 end_time 排序——坑）。
- `app/database/mongodb/mongodb_service.go`：GetMongoDBManager(27-32) sync.Once 只保证实例非 nil 不保证 Connect 成功；Connect(35-80) 拼 URI（authSource=admin）+ 六项池参数；GetDatabase(83-88) client nil 时返回 nil（上层用 db==nil 防御）。
- `app/database/mongodb/chat_history.go`：`SaveMessage`(10-20)——group/ai/ai_research 走群聊服务、private 走私聊（生成 SessionID）。
- `app/database/mongodb/history_helpers.go`：`historyCollectionNames`(11-22) 逐日回退 Format("200601") 再 map 去重——解决"当月 7 号查 7 遍重复"；`IsPrivateParticipant`(27-36) 按第一个 `_` 拆分 SessionID（用户 ID 含下划线会误判）；`regexpQuote`(39-41) QuoteMeta 防正则注入。
- `app/database/mongodb/mongodb_group_message_history_service.go`（476 行，细读）：
  - `AddGroupMessageByUser`(56-117)：两步写入——①`$setOnInsert`+upsert 确保当日存在未满桶（count<500）；②`messages.message_id $ne` + `$push/$inc/$set end_time` 原子追加（**故意不带 upsert**，否则 $ne 不匹配时会新建重复桶）；**桶满时更新匹配 0 条但不报错不重试（消息静默丢失风险点）**。
  - `GetUnreadMessages`(169-239)：遍历月集合、每集合按 end_time 倒序取 limit 桶、内存逐条过滤 After(afterTimestamp)；totalCount 是过滤后真实条数，与切片长度可能不一致。
  - `GetHistoryMessagesByCursor`(254-334)：游标=Unix 秒，`$lt` 取更早；Mongo 层 $elemMatch+$regex 缩小 + 内存层 Contains 二次过滤（两层重复）。
  - `GetHistoryMessagesByTimeRange`(337-476)：只有 startTime 时 end_time **升序**（从早到晚），其余倒序；范围过滤内存层再执行一次；最终 sort.Slice 倒序截断。
- `app/database/mongodb/mongodb_private_message_history_service.go`（342 行）：`GenerateSessionID`(51-56) 两 ID 字典序排序后 `_` 拼接（保证双向同桶）；`AddPrivateMessage`(59-108) 同款 $setOnInsert+$ne+$push（无分桶）；**SetSort(bson.M{"end_time": -1}) 对缺失字段按 null 排序——行为靠运气**。

**🎯 目标**：解释三层存储结构、500 条滚动策略、幂等三件套、私聊 session_id 规则、30 天跨月查询。

**🔑 重难点**
1. 幂等三件套的完整逻辑与"为什么不用 upsert"。
2. 桶满静默丢消息的风险点（面试可讲"已知局限"）。
3. 私聊 session_id 排序拼接的意义；字符串字典序 vs 数值序的差异。
4. 两层过滤（Mongo + 内存）的重复设计。

**📝 自测题**
1. AddGroupMessageByUser 第二步为什么不能带 upsert？带上会发生什么？
2. historyCollectionNames 解决什么问题？怎么避免当月重复查询？
3. GetUnreadMessages 里 totalCount 与返回长度何时不一致？
4. GenerateSessionID("10","2") 的结果？IsPrivateParticipant 的格式假设？
5. 私聊文档没有 end_time 字段但按它排序——结果是什么？
6. 桶满时消息去哪了？代码做了什么（或没做什么）？

**🗣 验收提示词**
> 你是 Chant 项目的陪练考官。我已逐句读完 chat_service 的 mongodb 目录全部文件。请考核我：① 画出三层存储结构并解释为什么按月+按天；② 500 条滚动与桶满丢消息风险；③ 幂等三件套分别防什么、为什么不用 upsert；④ 私聊 session_id 排序拼接的意义；⑤ 30 天查询如何跨月去重；⑥ 私聊按缺失字段排序的问题。逐题讲解。

- 完成：☐ 日期：____

---

### 3.5 鉴权中间件与 REST（补全 chat_service）

**⏱ 前置概念**
- 双通道认证：前端走 `Authorization: Bearer <JWT>`（auth_token 中间件经 gRPC 调 auth_service 校验）；ai_service 走 `X-Internal-Key` 内部密钥（authOrInternal）——后者路径下 gin Context **没有** userInfo。
- sqlc/gorm-gen：由表结构自动生成的类型安全查询代码（query/*.gen.go），看签名不看实现。

**📄 文件与函数指引**
- `app/middleware/auth_token/auth.go`：`Auth`(18-64)——①Header 为空回退 `?token=` 且自动补 "Bearer " 前缀（浏览器 WS 不能自定义 Header 的兼容设计）；②TrimPrefix 后 `token != authHeader` 判断前缀存在（大小写敏感）；③失败 c.JSON(401)（resp 可能为 nil 的 panic 隐患）；④成功 c.Set("userInfo", &services.UserInfo{...})。
- `app/middleware/get_user_chat_info/auth.go`：`Middleware`(16-52)——Redis Hash `userID:username` → last_offline_time；**对匿名结构体做类型断言（与 *services.UserInfo 类型不同，字段一致才行）**；gRPC 失败或 <=0 静默不缓存；注意该中间件**未在 router.go 使用**。`GetLastOfflineTime`(55-71)——HGet miss 则 gRPC 回源。
- `app/api/routes/router.go`：`SetupRoutes`(22-71)——CORS 由配置驱动；OPTIONS 204 短路；**/messages/history 不挂 auth_token 而用 authOrInternal**；ws 路由在认证组内。`authOrInternal`(76-84)——X-Internal-Key 匹配直接放行（恒等比较，无时间窗），否则手动执行 Auth()。
- `app/api/handlers/group_handler.go`：`CreateGroup`(29-84)——AI 群 ID 用 ai_+UnixNano、普通群走 GenerateGroupID()（PG sequence → "G%d"）；创建者取自 userInfo **忽略请求体伪造字段**；AddUserToGroup 失败回滚 DeleteGroup；成功后 MarkMessageAsRead 建会话记录；AI 群拉机器人入群（错误被吞）。`AddGroupMember`(88-130)——校验群主身份（403 防凭群号入群）+ gRPC 校验目标用户存在。`RemoveGroupMember`(154-180)——不能移群主本人。`GetUserGroups`(182-194)——忽略路径参数只返回登录用户群组。
- `app/api/handlers/message_handler.go`：`GetMessages`(53-148)——lastOfflineTime==0 返回空；会话 ID 列表做**成员过滤**（防被移出群后读到残留未读；查不到群记录的视为私聊保留）；cutoff=max(lastOfflineTime, lastRead)；limit=1001 探测溢出；total 封顶 1000、返回封顶 100。`GetMessageHistory`(150-202)——GetGroupByID 判断群/私聊；越权校验只在 userInfo 存在时执行（内部调用跳过）；**先调群聊服务再覆盖为私聊服务（冗余调用）**。
- `app/api/handlers/user_handler.go`：`GetUser`(11-23) 逗号 ok 形式断言 *services.UserInfo。
- `app/database/pgsql/pgsql_service.go`：Connect(35-72)——池 MaxIdle=10/MaxOpen=100/ConnMaxLifetime=0；**关键 `query.SetDefault(db)` 绑定 gorm-gen 查询包**。Initialize(92-121)——AutoMigrate + 裸 SQL 建 `group_id_seq`（sequence 不是表，AutoMigrate 不管）。
- `app/database/pgsql/pgsql_user_group_service.go`：AddUserToGroup(51-86)——注释写 FirstOrCreate 实现是"先查后建"（并发竞态窗口，联合主键兜底）；DeleteGroup(124-145)——两删之间**无事务**；GenerateGroupID(189-199)——`nextval` 取序列值 → "G%d"。
- `app/database/pgsql/model/model.go`：Group/UserGroup（联合主键）/PrivateChat/TempChat/Conversation（已读追踪）5 张表 + TableName 显式表名。
- `app/infrastructure/redis/redis_client.go`：GetRedisClient(22-27)——**非线程安全懒加载单例**（未加锁）。
- `integration_test/main.go`（`//go:build integration`）：双实例 E2E——手工实现 HS256 JWT（secret 是硬编码 sha256 hex，必须与 auth 配置一致）；直连 Mongo upsert 测试用户；A 发 WS 消息 B 收；PASS 判定只要收到 type 即可。

**🎯 目标**：说清 WS/REST 鉴权链、双通道认证、群组与消息 REST 接口、sqlc 使用模式。

**🔑 重难点**
1. WS 的 token 传递：浏览器 WS 不支持自定义 Header → query 参数兼容。
2. authOrInternal 双通道：内部密钥 vs JWT，下游如何兼容（"userInfo 存在才校验"）。
3. group_handler 的防伪造与权限校验（群主 403、不能移群主）。
4. GetMessages 的授权过滤逻辑（群/私聊区分）。
5. PG sequence 与 gorm-gen SetDefault 两个"隐性依赖"。

**📝 自测题**
1. 一个 WS 连接请求经过的完整鉴权链？
2. authOrInternal 放行路径下 GetMessageHistory 为何不报 forbidden？靠什么判断？
3. CreateGroup 里 ai 群与普通群 ID 分别怎么生成？创建后调 MarkMessageAsRead 的目的？
4. GetMessages 的成员过滤：不在 GetUserGroups 的会话何时保留为私聊？何时丢弃？
5. Initialize 为什么单独建 group_id_seq？
6. integration_test 的手工 JWT 密钥来自哪里？不一致时哪一步失败？

**🗣 验收提示词**
> 你是 Chant 项目的陪练考官。我已逐句读完 chat_service 的 middleware、router、handlers、pgsql、redis、integration_test。请考核我：① WS 完整鉴权链与 query token 兼容设计；② 双通道认证与下游兼容；③ 群组接口的权限矩阵；④ GetMessages 授权过滤逻辑；⑤ sequence 与 SetDefault 隐性依赖；⑥ E2E 测试的密钥来源。逐题讲解。

- 完成：☐ 日期：____

---
### 4.1 auth_service 入口与存储

**⏱ 前置概念**
- FastAPI lifespan：应用启动/关闭钩子（async context manager），在 import 后执行一次。
- 双 gRPC 服务：auth_service 同时暴露 HTTP（FastAPI，9030）与两个 gRPC（50051 token 校验 / 50052 离线时间）——chat_service 通过 gRPC 远程校验 token。
- JWT 双 token：短效 access（默认 30 分钟）+ 长效 refresh（30 天）；MongoDB UserTokens 存 refresh token 记录（业务过期时间 365 天——与 JWT 自身 30 天不一致，已知问题）。

**📄 文件与函数指引**
- `app/main.py`：`lifespan`(14-29)——先 await db_manager.connect() 再在 daemon 线程启动 token_auth_server（阻塞式 grpc.server）；关闭时只关 Mongo。`create_app`(31-63)——CORS allow_origins 默认仅 localhost:5173；auth 路由挂 /api/v1/auth；root 提示的 health 路径与实际不一致（实际是 /api/v1/auth/health）。
- `app/core/config.py`：Settings 类（pydantic-settings，环境变量大写映射）；secret_key 默认 get_secret_key() 固定派生，SECRET_KEY 可覆盖；algorithm 硬编码 HS256（与 jwt_service 重复定义）。
- `app/core/secret_key.py`：`get_secret_key()`(5-18)——写死的 name/fixed_number/fixed_field 三元组做 SHA256，确定性输出 64 位 hex——**安全性取决于这些明文常量，生产风险点**。
- `app/database/mongodb_service.py`：`connect`(80-139)——AsyncIOMotorClient + ping 验证；异常分支统一清理并 raise；except 元组里 Exception 兜底使前两个分支形同虚设。`get_collection`(165-168)——database 为 None 抛"数据库未连接"。模块级单例 db_manager(171)。
- `app/database/mongodb_user_service.py`（重点）：`_normalize_user_id`(22-30)——纯数字字符串转 int（Mongo 里 user_id 可能是 int）；**部分方法漏调它**（update/delete/get_by_email_with_password/get_user_status），字符串数字 id 可能查不到——已知隐患。`create_user`(35-45)——异常被吞只抛"创建用户失败"。`get_next_value`(172-185)——**Mongo 自增序列**：find_one_and_update + $inc + upsert + ReturnDocument.AFTER，$inc 原子性保证并发不重复。`get_next_user_id`(188-195)。`update_last_offline_time`/`get_last_offline_time`(247-286)——秒级 int，供 gRPC 用；成功判定 modified>0 or matched>0。
- `app/database/mongodb_user_token_service.py`：`create_user_token`(20-53)——refresh_token 由 JWTUtils 生成，过期 +365 天入库，is_valid=True，返回 refresh_token 字符串。`update_user_refresh_token`(56-77)——upsert + $setOnInsert 兜底。`update_user_token_is_valid`(94-105)——吊销（改密/登出）。
- `app/database/redis_service.py`：模块级 ConnectionPool(max_connections=20, retry_on_timeout)；set/get 封装。
- `app/database/redis_user_service.py`：`set_code`/`get_code`（**bytes 要 decode("utf-8") 否则与 str 比较恒不等**）/`delete_code`；`try_set_nx`(19-21)——SET NX EX 原子限流。
- `app/models/auth_model.py`：VerifyCodeRequest（username 3-10 位、code 强制 6 位、password 无强度约束——强度在 auth.py 校验）；ResetPasswordRequest 同时要求 user_id 与 email；RefreshTokenRequest 带 email。

**🎯 目标**：说清用户/token 在 Mongo 与 Redis 的分层、refresh token 生命周期、自增序列实现、_normalize_user_id 隐患。

**🔑 重难点**
1. 分层原因：Mongo 持久（用户+refresh token），Redis 短期（验证码+限流，TTL 自动过期）。
2. Mongo 自增序列的原子性（$inc + find_one_and_update）。
3. JWT 30 天 vs 库中 365 天的语义不一致。
4. secret_key 确定性派生的安全含义。

**📝 自测题**
1. 用户资料与 token 在 Mongo/Redis 的分布及原因？
2. get_next_value 用 find_one_and_update+$inc+upsert+AFTER 返回什么？为什么并发不重复？
3. 哪些方法漏调 _normalize_user_id？字符串数字 user_id 会出什么问题？
4. 为什么 register 里 create_user_token 失败只 warning 不阻断？
5. secret_key 如何保证重启不变？

**🗣 验收提示词**
> 你是 Chant 项目的陪练考官。我已逐句读完 auth_service 的 main.py、core、database、models。请考核我：① 存储分层与原因；② refresh token 完整生命周期；③ Mongo 自增序列的并发安全实现；④ _normalize_user_id 的漏调用隐患；⑤ 两个 gRPC 服务的启动方式；⑥ secret_key 派生机制。逐题讲解。

- 完成：☐ 日期：____

---

### 4.2 认证业务与 gRPC 服务

**⏱ 前置概念**
- bcrypt：加盐哈希；**只使用前 72 字节输入**，超长静默截断——所以密码必须限制字节数。
- 恒定时间比较：secrets.compare_digest 防时序侧信道（验证码/refresh_token 比较都用它）。
- gRPC 阻塞版 vs aio 版：token_auth_server 是 grpc.server + ThreadPoolExecutor + asyncio.run（同步 handler 里跑异步 Mongo）；last_offline_time_server 是 grpc.aio + async handler。

**📄 文件与函数指引**
- `app/api/v1/auth.py`（核心，逐函数细读）：
  - `_validate_password`(26-31)：<8 位拒绝；utf-8 字节数 >72 拒绝（bcrypt 截断）。
  - `_verify_code_or_raise`(34-62)：secrets.compare_digest 恒时比较；失败计数 code_attempts 达 5 抛 429；成功同时删验证码与计数（一次性）；key 就是 email。
  - `get_user_service`(72-77)：请求前 ping DB，失败抛 DATABASE_CONNECTION_ERROR。
  - `send_code`(88-113)：try_set_nx("rate:code:{email}") 原子限流 60 秒；**asyncio.to_thread 包同步 smtplib**；发送失败 delete_code 释放限流标记。
  - `register`(116-185)：顺序=密码校验→验码→查重→get_next_user_id→bcrypt.hashpw→先建 token（失败仅 warning）→create_user；初始 status=PENDING。
  - `verify_code_login`(188-222)：按 email 查用户（不存在抛 USER_NOT_FOUND）→验码→造双 token→落库。
  - `login`(225-268)：get_user_by_email_with_password 取含密码文档→bcrypt.checkpw（双方 bytes）→再取干净数据→签发。
  - `reset_password`(271-324)：user_id 先 int() 归一化；邮箱查出的 user_id 必须与请求一致（防篡改）；新旧密码相同抛 USER_PASSWORD_SAME；改密后吊销旧 refresh token。
  - `refresh_token`(327-403)：**校验链**——token 记录存在→verify_token success→is_valid→**payload.type=="refresh"（防用 access 换新）**→compare_digest 与库一致→payload user_id 与请求一致→用户存在→签发新双 token。
  - `logout`(406-449)：用户不存在返回成功（幂等）；校验携带 token 与库一致（防仅凭 user_id 登出他人）→置 is_valid=False。
- `app/api/v1/const.py`：Status.SUCCESS/PENDING（注册初始中间态）。
- `app/services/jwt_service.py`：`create_access_token`(18-45)——payload 先放 exp/iat/jti/type="access" 再 update 用户字段；exp=time.time()+分钟*60。`create_refresh_token`(48-75)——type="refresh"，默认 30 天。`verify_token`(78-104)——成功返回 {"status","payload","user_info":{user_id,username,email}}（gRPC 只取这三个）；ExpiredSignatureError 与 InvalidTokenError 分开捕获；异常把 traceback 写 /tmp/jwt_error.log（临时调试代码）。
- `app/services/email_service.py`：`generate_secure_code`(17-20)——大写字母+数字 6 位（secrets.choice）。`send_email`(23-52)——同步 smtplib（starttls→login→sendmail）；验证码以 to_email 为 key、600 秒 TTL 写 Redis。
- `app/infrastructure/grpc/token_auth_server.py`：`VerifyToken`(21-43)——委托 JWTUtils，user_id 转 str。`GetUserByID`(45-70)——**asyncio.run() 在同步 handler 里跑异步 Mongo（同一线程重复调用有事件循环冲突隐患）**。`serve`(73-80)——ThreadPoolExecutor(10) + add_insecure_port("[::]:50051")。
- `app/infrastructure/grpc/last_offline_time_server.py`：`UpdateLastOfflineTime`(20-44)——async handler 直接 await（aio 版无需 asyncio.run）；user_id 空返回 False。`GetLastOfflineTime`(46-73)——无记录返回 0，异常也返回 0（调用方无法区分）。`serve`(76-88)——grpc.aio.server，端口 50052。
- `app/utils/error_code.py`：ErrorCode 值对象 + ErrorCodeEnum——分段编号 10xxx 用户/认证、20xxx 数据库、30xxx 邮箱、40xxx 改密、50xxx 刷新、60xxx redis。

**🎯 目标**：完整复述注册/登录/刷新/登出四个流程；说清 refresh 校验链；两个 gRPC server 的差异。

**🔑 重难点**
1. refresh_token 校验链的每一环（面试爱考：为什么不能拿 access token 换新）。
2. 验证码：恒时比较 + 失败计数 429 + 原子限流（三层防护）。
3. bcrypt 72 字节截断与密码长度校验的对应。
4. asyncio.run 在同步 handler 里的隐患；aio 与阻塞版 gRPC 的区别。
5. 错误码分段设计。

**📝 自测题**
1. 注册流程完整顺序？create_user_token 失败会发生什么（取舍）？
2. refresh_token 校验链至少列 4 个条件？为什么 type 必须为 refresh？
3. 为什么用 secrets.compare_digest 而非 ==？验证码正确时删除了哪两个 key？
4. verify_token 返回结构？token_auth_server 取哪几个字段？异常写哪个文件？
5. reset_password 为什么先 int() 归一化？改密后对旧 token 做什么？
6. 两个 gRPC server 的实现风格差异？GetUserByID 的隐患是什么？

**🗣 验收提示词**
> 你是 Chant 项目的陪练考官。我已逐句读完 auth.py、const.py、jwt_service、email_service、两个 gRPC server、error_code。请考核我：① 注册/登录/刷新/登出四流程；② refresh 校验链与防 access 换新；③ 验证码三层防护；④ bcrypt 72 字节问题；⑤ 两个 gRPC server 风格差异与隐患；⑥ 错误码体系。逐题讲解。

- 完成：☐ 日期：____

---

### 5.1 ai_service 入口与消息总线

**⏱ 前置概念**
- asyncio 消费者模式：main 里一个 asyncio 事件循环跑 Kafka 消费循环 + LLM 长调用（协程交替执行，非多线程）。
- 优雅停机：信号 → stop_event → cancel 消费任务 → finally 停 consumer/producer/连接池。
- 双 AI 身份：AI_USER_ID="ai-assistant"（chat 模式）、AI_RESEARCH_USER_ID="ai-research"（agent 模式）——消息按 target/群类型分流。

**📄 文件与函数指引**
- `main.py`：`dispatch`(31-68)——①先判 is_agent（conversation_type=="ai-research" **或** target==AI_RESEARCH_USER_ID，用 or——命中即走 agent，避免被 chat 分支截走）；②chat 分支条件 target==AI_USER_ID **或** group_id.startswith("ai_")；③AiReplyGenerated 显式忽略（防 AI 回复再触发 AI）；④传给下游的是重新组装的 dict。`main`(71-124)——L10 先 sys.path.insert proto 生成目录再加其它 import；L14-15 显式 import provider 文件触发 @register；cost_store.init_pool() 与 init_qdrant() 各自 try/except 降级；finally 依次停。`_signal_handler`(83-85)。
- `config/settings.py`：pydantic-settings，环境变量 > .env > 默认；memory_* 配置是 context.py 折叠逻辑的数值来源（thresholdRatio=0.8 / retainRatio=0.16 / maxTokens=8192 / 窗口 1e6）。
- `shared/kafka/producer.py`：`create_producer`(19-26) value_serializer 直发 bytes。`send_ai_reply`(29-58)——**message_id=f"ai-{message_id}"**；reply_to_msg_id or message_id 兜底；**key=user_id.encode()**（同用户同分区保序）。`send_ai_reply_delta`(61-93)——同 key + seq/kind。`send_error_reply`(96-105)——message_id 传 f"err-{message_id}"。
- `shared/kafka/consumer.py`：`create_consumer`(14-30)——auto_offset_reset="latest" + enable_auto_commit=True（崩溃可能丢最新消息）。`consume_loop`(33-45)——async for 逐条；解析/处理异常只 logger.exception 后 continue（单条失败不中断）；CancelledError 后 finally consumer.stop()。
- `shared/proto_adapter.py`：parse_envelope/parse_message_sent/parse_ai_reply/parse_ai_reply_delta 反序列化；`envelope_to_json`(36-38)——MessageToJson(preserving_proto_field_name=True)。`new_envelope`(41-50)——event_id=f"{event_type}-{uuid.uuid4()}"，timestamp 毫秒。`new_ai_reply`(67-86)——**proto 可选字段用 kwargs 条件传参**（直接传 0/None 会因字段未定义报错）；sender_id 硬编码 "ai-assistant"。`new_ai_reply_delta`(89-109) 同款技巧。

**🎯 目标**：说清消息从 Kafka 字节到业务 dict 的转换、chat/agent 分流、异常恢复、AI 双账号。

**🔑 重难点**
1. dispatch 的分流顺序与 or 条件（为什么 agent 先判）。
2. proto_adapter 的 kwargs 条件传参技巧（proto 可选字段的坑）。
3. 消费异常不中断循环 + 至少一次投递语义。
4. key=user_id 保证 delta 与最终回复同分区有序（前端流式拼接依赖）。

**📝 自测题**
1. dispatch 里 is_agent 的 or 条件改成 and 会怎样？
2. sys.path.insert 必须发生在其他 import 之前吗？为什么？
3. send_ai_reply 与 send_error_reply 的 message_id 前缀各是什么？key 为什么必须用 user_id？
4. new_ai_reply 为什么用 kwargs 条件传参？直接传 0/None 会怎样？
5. 消费者异常时如何保证不丢消息？（诚实回答：auto_commit 会丢哪些）

**🗣 验收提示词**
> 你是 Chant 项目的陪练考官。我已逐句读完 ai_service 的 main.py、settings、shared/kafka、proto_adapter。请考核我：① 消息转换链路；② chat/agent 分流与 or 条件；③ 消费者异常恢复与至少一次语义；④ proto 可选字段的 kwargs 技巧；⑤ 双 AI 账号设计；⑥ key=user_id 的保序作用。逐题讲解。

- 完成：☐ 日期：____

---

### 5.2 LLM 抽象层（装饰器工厂模式）

**⏱ 前置概念**
- 抽象基类契约：AbstractLLM 定义 chat / chat_stream / langchain_model / get_pricing 四个方法——业务代码只依赖接口。
- 装饰器注册模式：@register("name") 把类写进全局注册表，get_llm() 按配置名查表实例化——新增模型零改动（开放封闭原则）。
- OpenAI 兼容协议：DeepSeek 复用 /v1/chat/completions，差异只在 base_url/model/key。

**📄 文件与函数指引**
- `shared/llm/base.py`：LLMMessage(18-22) role/content；LLMResponse(25-31) content+reasoning+model+usage；**LLMStreamChunk(34-45)——kind ∈ reasoning|content，usage 只在流末尾空 text 结算块携带**（chat_stream 调用方靠此约定收尾）；AbstractLLM(48-91) 四抽象方法，get_pricing 返回 USD/百万 token。
- `shared/llm/factory.py`：`register`(18-29)——装饰器工厂（必须先 @register("name") 调用再装饰类）；`get_llm`(32-45)——provider or settings.llm_provider 查表，未知抛 ValueError 并列出已注册；**每次都 cls() 新实例（无单例缓存）**；L48 注释：provider 注册靠 main.py 显式 import 触发（避免循环引用）。
- `shared/llm/router.py`：`route_chat`(11-12)——仅两行（v1 极简占位，预留多模型路由）。
- `shared/llm/providers/openai_compatible.py`：`chat`(64-97)——**@retry 仅 HTTPStatusError（429/5xx）重试 3 次指数退避**；kwargs.get 允许覆盖 max_tokens/temperature；思考模型读 reasoning_content。`chat_stream`(99-149)——line[6:] 剥 "data: " 前缀，[DONE] return；**usage 结算块 yield 空 text**（对应 base 约定）；reasoning_content 与 content 产出不同 kind；单 chunk 解析失败 continue。`langchain_model`(151-171)——延迟 import ChatOpenAI；http_proxy 注入 httpx 代理。`get_pricing`(173-175)——默认 0 定价子类覆盖。
- `shared/llm/providers/deepseek.py`：PRICING(22-25) 两档；`get_pricing`(34-38)——**docstring 说返回 tuple 实际返回 dict**（compute_cost 用 .get，dict 才对）。

**🎯 目标**：默写装饰器注册模式；说清新增一个 provider 的三步；流式结算块约定。

**🔑 重难点**
1. 装饰器注册模式完整实现与"为什么 import 触发注册"。
2. LLMStreamChunk 的 kind + 空 text 结算块约定（chat/agent 两处调用方都依赖）。
3. 重试策略只针对 HTTPStatusError——其他异常不重试。
4. 每次 get_llm() 都新实例（无缓存）的影响。

**📝 自测题**
1. 默写装饰器注册模式并解释设计？新增"通义千问"要改哪些文件？
2. 流式响应里 usage 怎么传回调用方？空 text 结算块的意义？
3. @retry 的触发条件是什么？网络超时（非 HTTPStatusError）会重试吗？
4. get_llm() 每次返回新实例还是单例？影响是什么？
5. deepseek.get_pricing 实际返回什么类型？谁在用？

**🗣 验收提示词**
> 你是 Chant 项目的陪练考官。我已逐句读完 shared/llm 全部文件。请考核我：① 默写装饰器注册模式并解释；② 新增 provider 的三步；③ 抽象基类契约；④ 流式结算块约定；⑤ 重试策略边界；⑥ get_llm 无缓存的影响。逐题讲解。

- 完成：☐ 日期：____

---

### 5.3 chat 模式与记忆折叠（chat/ 目录）

**⏱ 前置概念**
- 短期记忆 = DSH 折叠（compaction）：上下文 = 系统提示 + 折叠摘要 + 检查点(watermark)之后的原文尾巴 + 当前消息；超过预算阈值(0.8×窗口)时把旧段送 LLM 摘要，摘要连 watermark 幂等写入 Mongo 检查点（conversation_memory 集合），重启可恢复。
- "真相在历史"：消息本体只在 chat_service 的 Mongo，ai_service 只存摘要检查点。
- 代理设置：内部 HTTP 调用（chat_service 拉历史）trust_env=False 直连不走代理；外部 LLM API 走代理。

**📄 文件与函数指引**
- `chat/service.py`：`_fetch_history`(45-65)——@retry(httpx.HTTPError, 2 次)；cursor=int(time.time()) 秒级；trust_env=False；X-Internal-Key 内部鉴权头；返回新→旧。`handle_private_message`(68-182)——①安全断言（不是发给 AI 的 return）；②conversation_id = group_id or 排序拼接（与 chat_service GenerateSessionID 对齐）；③history.reverse() 摆正；④内嵌 `_flush` 闭包 nonlocal seq，同 kind 缓冲拼块、约 16 字符合块（流式体验 vs Kafka 吞吐的平衡）；⑤流末尾 usage 结算块用于成本记录；⑥空 content 发错误回复；⑦reply_id=f"ai-{msg_id}" 与所有 delta 共用；⑧思考全文拼 metadata["reasoning"] 随最终回复落库。
- `chat/schemas.py`：ChatRequest(4-9) 疑似遗留（实际走 dict+proto_adapter）。
- `chat/prompts.py`：SYSTEM_PROMPT 常量。
- `chat/memory/context.py`（**注意：没有 sliding_window.py，折叠逻辑在这里**）：`_get_db`(30-37) motor 惰性初始化、连接失败降级；`estimate_tokens`(51-54)——CJK 1 字 1 token，其余 (len-cjk+3)//4；`_to_epoch_ms`(57-67)——ISO 字符串 Z→+00:00；`load_checkpoint`/`save_checkpoint`(73-98)——Mongo upsert 幂等写 watermark_ms/summary；`_summarize`(112-119)——独立 LLM 调用，history 截 [-32000:]，max_tokens=8192；`format_history`(125-131)——按 sender 标"用户/AI"；**`assemble_context`(134-197) 核心**——①budget=1e6，trigger=0.8*budget、retain=0.16*budget；②tail=时间戳>watermark 且排除当前消息；③触发条件 estimate(summary)+tail_tokens > trigger；④保留尾巴从最新往回扫，`if kept+t > retain and i < len(tail)-1: break`（最后一条必然保留）；⑤折叠成功 watermark 推进并 save_checkpoint；⑥摘要失败只警告按原文继续（折叠失败不阻塞）；⑦拼装顺序 system → 摘要(user) → tail → 当前消息。

**🎯 目标**：说清 chat 模式完整流程；解释折叠触发条件、保留策略、检查点持久化。

**🔑 重难点**
1. assemble_context 的预算与触发公式；break 条件的另一半为什么必要。
2. _flush 闭包：nonlocal、16 字符合块、seq 递增——流式协议的工程细节。
3. 折叠失败降级：不阻塞对话。
4. 与 chat_service 的 SessionID 对齐规则。

**📝 自测题**
1. 触发折叠的条件表达式？保留尾巴的 break 为何还要 i < len(tail)-1？
2. conversation_id 生成规则？拉历史后为什么 reverse()？
3. _flush 里 nonlocal seq 的作用？16 字符合块的意义？
4. 摘要失败时会发生什么？
5. 检查点存到哪个集合？幂等靠什么？

**🗣 验收提示词**
> 你是 Chant 项目的陪练考官。我已逐句读完 chat/ 目录（service、schemas、prompts、memory/context.py）。请考核我：① chat 模式完整流程；② 折叠触发与保留策略的公式推导；③ _flush 闭包与流式合块；④ 检查点持久化与恢复；⑤ 折叠失败的降级；⑥ 与 chat_service SessionID 的对齐。逐题讲解。

- 完成：☐ 日期：____

---

### 5.4 Embedding 与 Qdrant 向量库

**⏱ 前置概念**
- 向量化与相似度：文本 → embedding API → 4096 维向量；Qdrant 用 COSINE 距离算相似度。
- 幂等建集合：init 时检查 collection 是否存在，存在则不重建（维度/距离配置只生效一次）。
- 同步客户端包异步：QdrantClient 是同步库，异步代码里用 asyncio.to_thread 包装。

**📄 文件与函数指引**
- `embedding/config.py`：EMBEDDING_DIM=4096（**必须与 Qdrant collection 维度一致**）、BASE_URL、两个独立 API key（embedding/rerank）。
- `embedding/client.py`：`embed`(21-46)——3 次尝试；_RETRY_STATUSES={429,500,502,503} 命中且 attempt<2 时 backoff 后 continue；**通用 except 分支同样重试（非状态码异常也重试 3 次才 raise）**；返回 data["data"] 每项 embedding。`rerank`(49-81)——空 documents 返回 []；top_n 默认 5；返回 results（index/relevance_score）。`_backoff`(84-85)——0.5*2^attempt。
- `qdrant/client.py`：`init`(16-34)——幂等建 collection；VectorParams(size=4096, distance=COSINE)。`get_client`(37-41)——未初始化 raise RuntimeError。`close`(44-48)。

**🎯 目标**：说清"文本→向量→Qdrant"路径、维度一致性、重试与退避策略。

**🔑 重难点**
1. 维度 4096 的双处配置一致性（embedding/config 与 qdrant/client）。
2. embed 的异常重试策略（状态码与非状态码都重试）。
3. to_thread 包装同步客户端。

**📝 自测题**
1. 维度配置在哪两处？不一致会怎样？
2. embed 对 500 与对 KeyError 的重试行为各是什么？
3. rerank 返回什么结构？空输入返回什么？
4. get_client 未初始化时抛什么？

**🗣 验收提示词**
> 你是 Chant 项目的陪练考官。我已逐句读完 embedding/ 与 qdrant/ 目录。请考核我：① 文本到向量的路径；② 维度一致性要求；③ embed 重试策略细节；④ COSINE 距离与集合配置；⑤ to_thread 包装的必要性。逐题讲解。

- 完成：☐ 日期：____

---

### 5.5 Agent 工具与长期记忆（agent/tools/）

**⏱ 前置概念**
- 混合检索：召回(recall) → 混合排序（语义分×0.6 + 时间衰减 exp(-0.005×小时)）→ 可选 rerank 精排——"粗召回再精排"漏斗策略。
- SSRF 防护：对外部 URL 必须校验——拒绝内网/环回/链路本地/保留/组播地址；DNS 解析后逐一校验 A/AAAA；禁重定向。
- 预取（prefetch）：graph 启动前先触发记忆检索（后台 task），首个节点直接消费结果——与图并行。

**📄 文件与函数指引**
- `agent/tools/long_term_memory.py`（最重要）：`store_memory`(42-90)——embedding 文本 "Q: q\nA: report[:800]"；payload 含 question/report_summary(500 截断)/report_full/domain/methodology/created_at(CST)；**asyncio.to_thread(get_client().upsert)**；失败各自降级。`retrieve_memories`(95-182)——①recall_limit=limit*4，query_filter 按 user_id 限定本人；②混合分 semantic*0.6 + time_score*0.4，time_score=exp(-0.005*age_hours)；③候选>limit 时 rerank，rerank_n=min(limit*3, len)，**成功则用 relevance_score 覆盖 score 并 pop 掉 semantic/time_score**；④rerank 失败回退混合排序 scored[:limit]。`_truncate`(192-202)——回溯最近句末符，要求 idx>0.6*max_len。`_calc_age_hours`(205-213)——naive 补 CST；解析失败返回 9999（老数据快速衰减）。`format_memories`(227-248)——含相对时间标签。`trigger_prefetch`(271-278)/`consume_prefetch`(281-290)——**pop(key, None) 保证同 key 只消费一次（幂等）**。
- `agent/tools/searxng.py`：`_is_safe_url`(37-84)——SSRF 核心：仅 http/https；IP 直接判、域名 getaddrinfo 解析所有 A/AAAA 逐一校验（防 DNS 指向内网）；ipv4_mapped 还原；拒绝 private/loopback/link_local/reserved/multicast/unspecified。`search_searxng`(87-116)——GET /search 带 UA；解析 raw 后 asyncio.gather(return_exceptions=True) 并行抓正文，zip 还原顺序；content 非 str 置空。`_parse_results`(119-133)——BeautifulSoup 找 article.result。`_fetch_content`(136-158)——**先 _is_safe_url 再抓，follow_redirects=False（防重定向到内网）**；trafilatura 提取取前 800 字。
- `agent/tools/wikipedia.py`：`_get_client`(15-29) 延迟单例 + 代理；`search_wikipedia`(39-79)——list=search 拿标题/pageid，再批量取 extract；quote(title.replace(" ","_"))。`_batch_extracts`(82-104)——prop=extracts&exintro&explaintext。
- `agent/tools/wikidata.py`：`search_wikidata`(21-44)——搜索→取属性→拼 summary 行；`_MATERIAL_PROPERTIES`(10-13) 把 P 号映射可读标签。`_get_properties`(65-100)——SPARQL VALUES 绑定 + SERVICE wikibase:label；POST 到 query.wikidata.org/sparql，Accept 必须 application/sparql-results+json；quantityUnit 拼单位。
- `agent/tools/time_tool.py`：get_current_time / get_current_time_readable；CST=UTC+8。
- `agent/schemas.py`：ResearchState(9-54) TypedDict——**messages 用 Annotated[..., add_messages] 归约器、knowledge_entries 用 operator.add 自动累加**（search/revise 每轮返回值拼接，这就是引用编号不断增长的原因）；**L54 坑：user_id: str 被行尾注释吞掉**——实际未声明为状态字段，靠 state.get("user_id") 读取（运行时 OK，静态类型缺失）。

**🎯 目标**：复述长期记忆写入/检索两条路径、混合排序公式、SSRF 防护、预取机制。

**🔑 重难点**
1. 混合排序公式与参数含义（0.6 权重、0.005 衰减、×4 召回、×3 rerank）。
2. SSRF 防护的完整链条（协议→IP→DNS→重定向）。
3. prefetch 的幂等消费（pop）。
4. LangGraph 归约器对引用编号的影响。

**📝 自测题**
1. retrieve_memories 混合得分公式？rerank 成功后对 score 做了什么？
2. _is_safe_url 如何防 DNS 指向内网？为什么 follow_redirects=False？
3. consume_prefetch 幂等靠什么？
4. knowledge_entries 的归约器是哪个？它如何影响引用编号？
5. store_memory 的 embedding 文本怎么拼？失败时怎么办？
6. schemas.py 里 user_id 字段的坑是什么？

**🗣 验收提示词**
> 你是 Chant 项目的陪练考官。我已逐句读完 agent/tools 全部文件与 schemas.py。请考核我：① 长期记忆写入与检索两条路径；② 混合排序公式与参数；③ 粗召回再精排的漏斗策略；④ SSRF 防护链条；⑤ prefetch 幂等；⑥ LangGraph 归约器与引用编号；⑦ schemas 的 user_id 坑。逐题讲解。

- 完成：☐ 日期：____

---

### 5.6 Agent 研究图（核心中的核心）

**⏱ 前置概念**
- LangGraph 状态图：StateGraph(TypedDict) 上节点返回 dict 增量合入状态；条件边按返回值路由（route_after_critique）。
- bind(response_format=json_object)：LLM 强制 JSON 输出（intent/classify/critique 节点用 llm_json，自由文本节点用普通 llm）。
- 流式执行：graph.astream(state, stream_mode=["updates","values"])——updates 给出每节点输出（用于进度推送），values 给出累积状态（用于最终结果）。
- 审核-修订循环：critique 输出 {passed, issues}，不过则 revise 重写，最多 max_revisions(默认 2) 轮；仍不过则报告加免责声明。
- "代码做确定性路由，LLM 做语义填充"：方法论映射/引用编号/参考文献生成用代码，语义理解用 LLM。

**📄 文件与函数指引**（按依赖顺序读）
- `agent/graphs/methodology.py`：`get_methodologies`(39-44)——METHODOLOGY_MAP 子串匹配（if key in domain），未命中回退 DEFAULT_METHODOLOGIES；`is_analytical_domain`(47-52)——决定 classify 的 bias 与 cognitive 走法；`get_dimension_hints`(55-59)——仅"经济"返回 MACRO_ECONOMY_DIMENSIONS。
- `agent/graphs/prompts.py`：12 个常量（无逻辑）。要点：统一 str.format 模板，占位符与节点代码一一对应；**ANALYZE_FACTUAL_PROMPT 的 knowledge_text 占位符出现两次（复制残留，format 时第二次覆盖）**；引用纪律规则（[N] 编号、多源写 [1][3]、禁止无引用数字）是 finalize 审计与 critique 校验的契约；CRITIQUE_PROMPT 的 passed 标准"无 high 且 medium ≤ 2"。
- `agent/graphs/research.py`（最长最关键）：
  - `_build_chat_model`(39-42)：get_llm().langchain_model()。`_parse_json`(45-50)：剥三反引号再 loads。`_format_knowledge`(64-82)：固定 "[N] [source] title | url" 格式——引用编号的权威来源。
  - `_finalize_report`(141-209)：**finalize 节点（纯程序零 LLM）**——①引用审计：invalid（越界）/empty_cited（引用无正文条目）/uncited（未被引用）；②`_strip_preamble`(100-121) 删开场白角色扮演；③`_strip_fabricated_references`(124-138) 用 **re.search 定位"## 参考文献"再截断**（而非 re.sub+DOTALL——贪婪匹配会误删正文）；④删除非法 [N] 用 re.sub 转义方括号；⑤按 sorted(cited) 生成真实参考文献节。
  - `build_research_graph`(214-679)：图内节点：
    - `intent_node`(222-257)：先 consume_prefetch 消费预取记忆，空则回退 retrieve_memories；JSON 解析失败有默认兜底 {"question_type":"rigorous"}。
    - `casual_search_node`(275-295)：仅 SearXNG（轻量路径）。
    - `search_node`(398-445)：**两层 asyncio.gather**——内层每 query 并发 wiki+searxng+wikidata，外层所有 query 并发；按 _order={"wikipedia":0,"wikidata":1,"web":2} 排序保证引用编号稳定可复现。
    - `critique_node`(480-551)：两轮——第一轮 LLM critique（报告截前 8000 字 + citation_audit）；第二轮对 high 问题用 searxng 核查，命中事实则降为 medium；**revision_count+1 在此节点内返回**。
    - `revise_node`(554-617)：从审稿意见生成 3-5 个补充搜索词→并发搜索→all_entries = existing + new_entries（**新条目编号从 len(existing)+1 连续**）→REVISE_PROMPT 重写。
    - `route_after_critique`(626-636)：passed or rev >= max_revisions(2) → "end"，否则 "revise"。
    - `route_after_intent`(655-661)：question_type=="casual" 走轻量路径。
    - 图接线(639-679)：注意 llm_json = llm.bind(response_format={"type":"json_object"})(217)。
- `agent/service.py`：`_send_delta`(27-36)——kind="thinking" 同时 append 进 thinking_parts。`_format_search_thinking`(39-49)——统计"检索完成：N 条资料 + 前 5 标题"。`handle_agent_message`(52-234)——①reply_id=f"ai-{msg_id}"、seq_counter=itertools.count()；②trigger_prefetch 在 graph 启动前（并行）；③astream 双模式：values 更新 last_state、updates 按 node_name 分流发 thinking/progress；④异常/无状态/无报告三条错误路径；⑤metadata（report_type/domain/methodology/summary[:500]），thinking_parts 拼 metadata["reasoning"]；⑥审核未过加"（注：本报告经 N 轮审核仍存在问题）"前缀 + 追加 critique 附件；⑦成本：cost_cb → get_pricing → compute_cost → insert_cost；⑧asyncio.create_task(store_memory) 后台写记忆不阻塞；⑨最后 done 空块 + send_ai_reply。
- `shared/cost/tracker.py`：`compute_cost`(7-26)——(prompt/1e6)*input + (completion/1e6)*output；缺失按 0。
- `shared/cost/store.py`：`_ensure_partitions`(40-67)——**按月自动补建分区** {table}_{YYYYMM}，边界每月 1 日 +08 到次月 1 日 +08（divmod 技巧算年月）；`init_pool`(70-88) asyncpg 池(min=1,max=5) + 建表；`insert_cost`(98-119)——cost_tracking_enabled=False 直接 return；$1..$10 占位。
- `shared/cost/agent_callback.py`：`CostTrackingCallback`(9-28)——on_llm_end 从 llm_output["token_usage"] 累加；total_tokens 为 property。

**🎯 目标**：默画九节点图，说清每节点输入/输出/是否用 LLM；审核-修订退出条件；finalize 为何零 LLM；成本统计链路。

**🔑 重难点**
1. 图结构与条件边（casual 分流、critique 回路、max_revisions 退出）。
2. 引用编号机制：search 排序 → LLM 写 [N] → finalize 审计/清洗/生成参考文献。
3. 双层 gather 的并发模型与排序稳定性。
4. 流式进度：updates/values 双模式。
5. 成本：Callback → 定价表 → 分区表落库。
6. 记忆：prefetch 并行 + 完成后 create_task 写库。

**📝 自测题**
1. 默画九节点图，逐节点说明输入/输出/是否用 LLM？
2. route_after_critique 的退出条件？revise 新条目的编号从哪开始？
3. _strip_fabricated_references 为什么用 re.search 截断而非 re.sub？
4. search_node 的双层 gather 如何保证引用编号稳定？
5. critique_node 两轮分别做什么？revision_count 在哪递增？
6. 成本从 token 到落库的完整链路？
7. updates 与 values 模式的区别？为什么都要？

**🗣 验收提示词**
> 你是 Chant 项目的陪练考官。我已逐句读完 methodology.py、prompts.py、research.py、agent/service.py 与 shared/cost 全部文件。请考核我：① 默画九节点图并逐节点说明输入/输出/是否用 LLM；② 审核-修订循环退出条件与"仍不过"处理；③ 引用编号从生成到清洗的完整旅程；④ 双层 gather 与排序稳定性；⑤ 代码路由+LLM 填充的体现；⑥ 成本统计落库链路；⑦ 记忆预取与写入的接入点；⑧ prompts 里 knowledge_text 重复占位符的隐患。题目量大，可分两轮。逐题讲解。

- 完成：☐ 日期：____

---
### 6.1 orchestrator 入口与消费 🔍（原理学习型）

> 📌 **模块定位（重要，先读）**：orchestrator_service 是早期开发的 Saga 编排**原型模块**，生产环境并未真正使用——部署脚本/systemd 服务里没有它，且**没有任何业务服务发送 Saga 启动事件（start-event）**，它即使运行也是空转。它在本项目里的价值 = 一本"Saga 实现教材"。
> 因此本节与 6.2 的定位是：**读懂 Saga 的原理与核心代码**（状态机、补偿、乐观锁、租约锁），**不要求逐句精读全部工程细节**，更不要求初中级开发者能开发 Saga（那是高级工程师的领域，面试时能讲清原理已是加分项）。

**⏱ 前置概念**
- Saga 编排模式：把大事务拆成多个步骤，全部成功即完成；任一步失败就反向对"已成功执行"的步骤做补偿回滚。
- 事件驱动分发：入站 5 类事件（start-event/event-success/event-failed/event-recover-success/event-recover-fail）→ 出站指令（saga.step.execute / saga.step.compensate / saga.initiated / saga.completed / saga.compensated）——event_consumer.go 的 switch 与 ARCHITECTURE.md 一一对应。
- 三层锁：全局 sagasMutex（RWMutex）+ 单实例 Saga.Mu + Compensating 标志/compensationRetryTracker 去重 map。

**📄 文件与函数指引**（🔑=必读核心，👀=流程理解，⏭=浏览即可）
- 🔑 `ARCHITECTURE.md`：只读两章——**章节 6**（Saga 完整生命周期：全部成功→completed；某步失败→重试→重试耗尽→补偿→全成功→compensated；补偿失败→指数退避最多 10 次→发 saga-dlq）、**章节 7**（三层锁）。
- 👀 `kafka/consumer/event_consumer.go`：`HandleEvent`(37-74)——switch 分发 5 类事件；default 忽略未知；**handler 返回 error 会被 SDK 重放 → 处理逻辑必须幂等**。
- 👀 `kafka/producer/event_producer.go`：`SendEvent`(20-27)——**key 用 sagaID**（同一 Saga 的事件同分区、有序）。
- 👀 `kafka/handlers/start_event.go`：只看 `HandleSagaStartEvent`(55-196) 的**骨架**——加载模板→建 Saga→落库→AcquireLock（30s 租约）→按 execution_mode 启动步骤（串行/并行）。
- ⏭ `main.go` / `config` / `consumer_runner.go` / `kafka_manager.go`：浏览即可（启动装配与配置，套路与 chat_service 相似，不考细节）。

**🎯 目标**：说清"一个 Saga 从启动到结束经历哪些事件"；理解 handler 幂等要求与 sagaID 分区键。

**🔑 重难点**
1. 入站 5 类事件 → 出站指令的映射（背下来，6.2 全靠它）。
2. handler 幂等（SDK 重放失败事件）。
3. sagaID 做 key 保证单 Saga 有序。

**📝 自测题**
1. 入站 5 类事件分别触发什么动作？
2. handler 返回 error 后会发生什么？为什么必须幂等？
3. 为什么 SendEvent 用 sagaID 做 key？

**🗣 验收提示词**
> 你是 Chant 项目的陪练考官。我已读完 orchestrator 的 ARCHITECTURE.md（章节 6/7）、event_consumer、event_producer、start_event 骨架。请考核我：① 默画 Saga 完整生命周期图（成功/失败/补偿失败三条路径）；② 入站 5 类事件与出站指令的映射；③ handler 幂等要求；④ sagaID 分区键的作用。**注意：本模块未实际部署，请以讲清原理为目标，不要考工程外围细节。**逐题讲解。

- 完成：☐ 日期：____

---

### 6.2 Saga 状态机与补偿（分布式事务）🔍

> 📌 **模块定位**：同 6.1——本节是**原理学习**。重点吃透四件事：① Saga 状态机（状态/转换表/事件）；② 补偿机制（Executed/Compensating 标志 + 逆序回滚）；③ 乐观锁 + 分布式租约锁（为什么需要、SQL 怎么写）；④ 崩溃恢复（PostgreSQL 持久化）。**工程外围（YAML 模板、雪花 ID、超时检查调度细节）了解即可，不考。**

**⏱ 前置概念**
- 乐观锁（Version）：Save 用 WHERE id=? AND version=? 更新，冲突返回 ErrVersionConflict 由上层重试。
- 分布式租约锁：Saga 表 LockedBy/LockExpiry 字段，实例用 uuid 抢占（30 秒租约），**每个事件处理前必须 RenewLock**，防多实例重复处理。
- Executed 与 Compensating 标志：Executed=true 表示"成功执行过、需要补偿"；Compensating=true 防同一补偿重复触发。
- 状态机合法转换表：pending→{running,failed,cancelled}；running→{completed,failed,compensating}；failed→{compensating,cancelled}；compensating→{compensated,failed}；终态空集。

**📄 文件与函数指引**（🔑=必读核心，👀=流程理解，⏭=了解即可）
- 🔑 `orchestrator/saga/saga_model.go`：状态常量(14-27)；**事件类型常量(34-70)是全项目事件字典**；SagaStep(129-150)——**Executed（是否需要补偿）与 Compensating（防重复补偿）是补偿逻辑核心开关**；Saga(153-179)——Version 乐观锁、LockedBy/LockExpiry 分布式锁、Mu 实例锁。
- 🔑 `orchestrator/saga/saga.go`：`NewSaga`(10-22)——Pending、CurrentStep=-1、MaxRetryCount=3。`SetStatus`(25-30)/`setStatusLocked`(33-44)——**SetStatus 会重新加 Mu，持锁时只能调 setStatusLocked**。`isValidStatusTransition`(47-69)——合法转换表（默写对象）；SetStatus 返回 false 时调用方处理失败分支。`IsCompleted/IsFailed/IsRunning`(153-171)——每个查询自己加锁，**持锁调用会死锁**。⏭ `AdvanceToNextStep`(83-111)/`MarkStepFailed`(114-134)/`RetryStep`(137-150)——未被事件流调用，属冗余 API，扫一眼即可。
- 🔑 `orchestrator/saga/repository.go`：ErrVersionConflict/ErrSagaNotFound 哨兵错误；SagaRepository 接口(17-49)——Create/Save/Get/Delete/Exists/AcquireLock/ReleaseLock/RenewLock 的语义（锁方法"只有持有锁的实例能成功"）。
- 🔑 `database/pgsql/saga_repository.go`：`Save`(39-79)——**乐观锁核心**：currentVersion→+1→WHERE id=? AND version=? 执行 Updates；RowsAffected==0 用 Exists 区分 ErrVersionConflict/ErrSagaNotFound；失败路径回滚 Version（供上层重试）。`AcquireLock`(209-226)——WHERE id=? AND (locked_by IS NULL OR lock_expiry < now) 原子抢占；`ReleaseLock`(230-246)——WHERE id=? AND locked_by=?（只有持有者能释放）；`RenewLock`(250-267)——+ lock_expiry > now 续租。`modelToSaga`(149-186)——**Mu 重置为 sync.Mutex{}（崩溃恢复的关键一步）**。
- 👀 `kafka/handlers/failure_event.go`：`HandleStepFailureEvent`(13-134)——重试 vs 进入补偿的决策（RetryCount<MaxRetries 则重发，否则**不置 Executed** 并走补偿）；`TriggerSagaCompensation`(138-159)——**只有 Executed && !Compensating 的步骤才补偿；失败步骤自身跳过；加锁置位后异步并发触发**。
- 👀 `kafka/handlers/success_event.go`：`HandleStepSuccessEvent`(15-173)——正常完成置 Executed；allCompleted 时**持锁直接赋值 Status=Completed+Version++（持锁调 SetStatus 会死锁）**；完成清理流程（发事件→删内存→ReleaseLock→删库）。
- 👀 `kafka/handlers/compensation_success_event.go`：`HandleStepRecoverySuccessEvent`(13-120)——Executed 置 false，全 false 即补偿完成 → SetStatus(Compensated) + 清理；**Compensating 标志不清（防重放再次触发）**。
- ⏭ `kafka/handlers/compensation_failure_event.go`（补偿失败：进程内去重 + 退避 10 次 + 投 saga-dlq——看结论即可）、`compensation_event.go`、`orchestrator_execution.go`（CheckTimeouts 扫一眼：只查 Running、超时进补偿）、`templates/`（知道"YAML 模板驱动步骤"即可）。

**🎯 目标**：默写状态转换表；讲清"第三步失败后发生了什么"（重试→补偿→完成）；解释乐观锁与租约锁为什么需要、SQL 怎么写。

**🔑 重难点**
1. 状态机合法性校验与"持锁不调 SetStatus"的死锁规避。
2. 补偿语义：只有 Executed 的步骤才补偿；失败步骤自身不补偿；补偿异步并发。
3. 乐观锁三连：Version 递增、WHERE 条件、失败回滚供重试。
4. 分布式锁：抢占/释放/续租的 WHERE 条件差异。
5. 崩溃恢复：为什么必须持久化到 PostgreSQL；modelToSaga 重初始化 Mu。

**📝 自测题**
1. 默写状态转换表并解释每个转换的合理性？为什么 compensated/cancelled 是空集？
2. 第三步失败时补偿如何执行？哪些步骤被回滚、为什么失败步骤被跳过？
3. Save 的乐观锁 WHERE 条件？RowsAffected==0 如何区分两种错误？为什么回滚 Version？
4. AcquireLock/ReleaseLock/RenewLock 的 WHERE 条件与目的？
5. 为什么持锁时不能调 SetStatus？（死锁分析）
6. 为什么 Saga 状态必须持久化？进程崩溃后如何恢复？modelToSaga 里 Mu 怎么处理？

**🗣 验收提示词**
> 你是 Chant 项目的陪练考官。我已读完 saga_model.go、saga.go、repository.go、saga_repository.go 与三个关键 handler（failure/success/compensation_success）。请考核我：① 默写状态转换表并逐条解释；② 第三步失败时补偿的完整执行过程；③ 乐观锁的 SQL 语义与错误区分；④ 租约锁三个方法的 WHERE 差异；⑤ 死锁规避；⑥ 崩溃恢复机制。**注意：本模块未实际部署，以讲清 Saga 原理为目标，不考 YAML 模板/雪花 ID/超时调度等外围细节。**题目量大可分两轮。逐题讲解。

- 完成：☐ 日期：____

---

### 7.1 前端骨架（React/TS 入门）

> 你不熟 React/TS，本节约 2~3 次读完，重在概念：组件、props/state、路由、Hooks。

**⏱ 前置概念**
- React 组件树：main.tsx 用 createRoot 挂载 App；App 用 react-router 定义路由；页面组件内部再用 useState/useEffect 管理状态与副作用。
- JSX = 组件即函数：props 传参、JSX 里写表达式；`.module.scss`（CSS Modules）使类名局部作用域，避免全局污染。
- 开发 vs 生产代理：开发走 Vite 代理（/api/v1/auth→9030、其余→8080），生产走 nginx，前端只写相对路径。

**📄 文件与函数指引**
- `package.json`：scripts——build 先 `tsc -b`（类型错误阻断构建）再 vite build；dependencies——React 19 + react-router-dom 7 + axios + **react-markdown/remark-gfm（AI 回答 Markdown 渲染）** + sass。
- `vite.config.ts`：`defineConfig`(4-18)——port 5173；**代理顺序敏感：/api/v1/auth 必须声明在 /api 之前**；changeOrigin 改写 Host。
- `index.html`：`#root` + `<script type="module" src="/src/main.tsx">`。
- `src/main.tsx`：createRoot().render(<StrictMode><App/></StrictMode>)；**StrictMode 下开发环境 effect 执行两次（预期，别当 bug）**。
- `src/App.tsx`（根组件）：路由——/login、/register、/ → 重定向 /login、/dashboard → ChatPage。**`refresh`**（14-29）：每 60 秒检查 access token 是否 5 分钟内过期，过期调 refreshToken 换新；**刷新失败 = localStorage.clear() + 整页跳 /login（强制登出）**。useEffect 依赖空数组只建一次定时器。
- `src/styles/global.scss` + `styles/tokens.scss`：@use 设计令牌（--chant-* 变量）；暗色主题 color-scheme: dark；button 重置后各组件自行覆盖。

**🎯 目标**：说出应用"入口→路由→页面"组织；认识 useState/useEffect；理解 token 自动刷新机制。

**🔑 重难点**
1. React 组件树与渲染流程（index.html → main.tsx → App → 路由 → 页面）。
2. token 自动刷新：expiresWithin 语义（**已过期也返回 true**）+ 失败强制登出。
3. StrictMode 双执行 effect。
4. CSS Modules 与全局样式的关系。

**📝 自测题**
1. React 应用从 index.html 到页面的加载链？
2. expiresWithin(token, 5) 对已过期 token 返回什么？刷新失败时 App 做了什么（两件事）？
3. 为什么 build 要先跑 tsc -b？
4. /api/v1/auth 代理为什么要声明在 /api 前面？

**🗣 验收提示词**
> 你是 React 入门陪练考官。我已读完前端骨架（package.json、vite.config、index.html、main.tsx、App.tsx、样式）。请考核我：① 加载链与组件树；② 路由定义；③ token 自动刷新机制与失败处理；④ JSX/props 基础；⑤ CSS Modules；⑥ 3 道 Hooks 基础题。逐题讲解，讲基础。

- 完成：☐ 日期：____

---

### 7.2 登录注册与请求封装

**⏱ 前置概念**
- axios 拦截器剥层：响应拦截器直接 `return response.data`——**API 封装的返回值就是 HTTP body 本身**（不是 axios response）；登录响应是 {message, data:{...}}，所以取 token 要 response.data.access_token。
- localStorage 会话三件套：access_token / refresh_token / user_info——前端会话唯一事实源，key 名全局一致。
- 受控组件：input 的 value 由 state 驱动、onChange 更新 state——React 表单标准做法。

**📄 文件与函数指引**
- `src/utils/request.ts`：axios.create({baseURL: "/api/v1", timeout: 10000})；响应拦截器剥层；错误分支保留 error（调用方用 error.response?.data?.detail 取 FastAPI 错误）。**此实例无请求拦截器（不发 Authorization）**。
- `src/utils/token.ts`：`parseJwt`(2-7)——payload 段 base64url→atob 解码，**只解不验**；异常返回 null。`expiresWithin`(10-13)——(exp*1000 - now) < minutes*60_000；已过期也 true；无 exp 返回 true（保守）。
- `src/api/auth.ts`：authApi 对象——sendCode/verifyCodeLogin/login/register/refreshToken；LoginResponse 含 user + 双 token。
- `src/api/chat.ts`：**独立 chatRequest**（与 request.ts 不同——多了请求拦截器从 localStorage 取 access_token 加 Authorization: Bearer，失败则不带头）；getUserGroups/createGroup（默认 normal，ChatPage 传 "ai"/"ai-research"）/getMessages（含 total_unread_count）/markMessagesAsRead/getHistory（**返回 latest 在前**，HistoryItem 字段 sender_id/timestamp 与 WS 消息不同）。
- `src/pages/LoginPage.tsx`：倒计时 useEffect 依赖 [countdown]（每次变化重建 interval）；`handleSendCode`(24-34) 限流 60s；`handleLogin`(36-55)——成功写三件套 + 800ms 延迟跳转；**118 行用 message.includes("成功") 判断消息盒样式（字符串约定）**。
- `src/pages/RegisterPage.tsx`：`handleRegister`(37-60)——全字段校验 + password !== repeatPassword（**只在客户端**）；写三件套（**refresh_token 判空才写**，与登录页不同）；注册即登录。

**🎯 目标**：说清登录/注册流程、请求封装的两个实例差异、token 存取。

**🔑 重难点**
1. 拦截器剥层后的类型语义（泛型第二个参数即 body）。
2. request 与 chatRequest 的差异（有无鉴权头）。
3. localStorage 三件套的写入时机差异（注册页判空）。
4. 前端只解不验 JWT——安全边界在服务端。

**📝 自测题**
1. request.ts 剥层后 authApi.login 的 Promise 解析值形状？为什么 response.data.access_token 恰好取到 token？
2. chatRequest 与 request 的关键差异（至少 2 点）？token 不存在时发生什么？
3. LoginPage 倒计时 effect 从 60 到 0 重建多少次 interval？代价？
4. 注册页与登录页写 refresh_token 的差异？为什么？
5. parseJwt 对两段式 token 返回什么？

**🗣 验收提示词**
> 你是 React 入门陪练考官。我已读完 request.ts、token.ts、auth.ts、chat.ts、LoginPage、RegisterPage。请考核我：① 登录流程前端完整动作；② 拦截器剥层语义；③ 两个 axios 实例差异；④ 受控组件；⑤ localStorage 三件套；⑥ 注册页校验。逐题讲解，讲基础。

- 完成：☐ 日期：____

---

### 7.3 聊天页与组件（前端核心，⚠️ 建议拆 2 次读）

**⏱ 前置概念**
- WebSocket 流式 delta 协议：AI 回复以 kind=thinking/progress/content/done 的分块消息到达，前端按 message_id 累积成流式条目；最终整块消息到达时清掉流式条目。
- Kafka 至少一次投递 → 前端必须按 message_id 去重（seenMessageIdsRef Set）。
- 乐观渲染：自己发的消息先本地显示（handleWsMessage 收到自己消息直接 return 防重复）。

**📄 文件与函数指引**
- `src/pages/ChatPage.tsx`（431 行，核心装配层）：
  - 状态：会话/消息/成员管理/流式输出；ref：currentGidRef、seenMessageIdsRef；派生：`currentGid = selectedGroupId || activeGroup`（AI 会话 vs 普通群双轨）、isAIGroup 过滤。
  - **`handleWsMessage`(55-135)（最复杂，重点）**：①自己发的 return；②delta 分块（msg.kind && msg.message_id）按 kind 累积进 streams——thinking 追加 reasoning、progress 去重 push steps、content 追加、done 置 done；**不进 messages 流、不计数未读**；③旧协议进度（metadata.kind==="progress" && reply_to_msg_id）挂靠原始输入；④最终回复/错误到达清进度点；⑤**seenMessageIdsRef 按 message_id 判重丢弃（Kafka 重投）**；⑥最终整块到达清流式条目；⑦非当前会话才累加未读。
  - 初始化 effect(145-163)：initUser → getUserGroups → 自动选中第一个 ai 群 → getMessages → **删首个 AI 群未读数并 markMessagesAsRead**。
  - 切会话 effect(166-181)：getHistory(currentGid, 0, 50) → HistoryItem 映射 WsMessage（sender: m.sender_id；time 数字/字符串兼容；reasoning: metadata?.reasoning）→ **reverse() 翻成 oldest-first**。
  - `handleLoadMore`(192-212)：`cursor = Math.floor(earliestTime / 1000)`（毫秒→秒游标）；`hasMoreHistory = length === 50`。
  - `handleSend`(235-268)：msgId = userId + "-" + Date.now()；**convType 三态推导**——ai-research 群→"ai-research"；AI 群且 agentMode→"ai-research"；AI 群普通→"ai"；否则 "group"；send 返回 false（未连接）不本地渲染；成功乐观追加。
  - 成员管理(286-318)：仅群主可见（isOwner）。handleLogout：clear + navigate("/login")。
  - 渲染(325-328)：`filteredMessages = messages.filter(m => m.group_id === currentGid)`（全量存、渲染过滤）；JSX 含 Chat/Research 开关。
- `src/hooks/useChatSocket.ts`：WS 生命周期 Hook——连接/收发/断线状态机，返回 connected/statusMsg/send。
- `src/components/`（每个 .tsx + .module.scss 成对）：`ChatInput`（输入+发送，agentMode accent 差异）、`MessageBubble`（**react-markdown 渲染 AI 回答**，含 reasoning 折叠）、`MessageList`（滚动/空态）、`NewChatModal`、`Sidebar`（会话列表/未读角标/新建/登出）、`ThinkingBlock`（流式思考展示）。
- `src/types/chat.ts`：SendPayload/WsMessage/StreamEntry 等前端类型。

**🎯 目标**：说清 WS 生命周期、delta 累积与清理、去重、历史加载与游标、convType 推导。

**🔑 重难点**
1. delta 分块的完整生命周期：thinking/progress/content/done 各自累积规则；最终整块到达时清理。
2. 去重时机：只有整块消息（带 message_id）参与 seenMessageIdsRef；delta 分块不参与。
3. 历史加载：latest-first → reverse()；游标秒级转换。
4. convType 三态推导与后端 HandleChat 的对应。

**📝 自测题**
1. handleWsMessage 中带 kind 且带 message_id 的消息为什么直接 return？kind === "done" 只做了什么？
2. seenMessageIdsRef 去重发生在哪类消息上？为什么必要？
3. handleSend 的 msgId 下游用途（至少 3 处）？send 返回 false 时发生什么？
4. 历史映射 time 为何要 typeof 分支？映射完为什么 reverse()？
5. handleLoadMore 的 cursor 为什么取整成秒？length === 50 判断什么？
6. currentGid 的双轨推导逻辑？

**🗣 验收提示词**
> 你是 React 入门陪练考官。我已读完 ChatPage.tsx、useChatSocket.ts 与全部组件。请考核我：① delta 流式协议的完整前端处理（四 kind + 清理时机）；② 去重机制与 Kafka 语义；③ 历史加载与游标；④ convType 三态推导；⑤ WS 生命周期与 cleanup；⑥ 乐观渲染与防重复。逐题讲解。

- 完成：☐ 日期：____

---

### 8.1 Docker 编排全家桶（部署入门）

> 你不熟 Docker，本节约 2~3 次读完，重点是"每个容器是什么、依赖谁、端口多少"。

**⏱ 前置概念**
- compose 三大件：services（容器定义）/ networks（互联）/ volumes（持久化）；depends_on + condition: service_healthy 控制启动顺序。
- 多阶段构建：builder 阶段编译/装依赖，runner 阶段只拷产物——镜像小、层缓存友好。
- **docker-entrypoint-initdb.d 机制**：postgres 镜像首次初始化数据卷时才执行该目录脚本；**POSTGRES_MULTIPLE_DATABASES 是装饰性变量，真正建库的是脚本里硬编码的库名**。

**📄 文件与函数指引**
- `docker-compose.yml`（开发全栈）：networks chant-shared-network（容器名互访）；kafka 双监听器（INTERNAL 29092 / EXTERNAL 9094）+ AUTO_CREATE_TOPICS；postgres 挂 init 脚本 + healthcheck；auth-service 9030+50051/50052；chat-service 8080（POSTGRES_DB_NAME=orchestrator、MONGODB_DB_NAME=chat）；orchestrator-service 8081:8080 消费 saga-events；searxng 8888:8080；5 个命名卷。
- `docker-compose.prod.yml`：资源限制 ${VAR:-default} 注入；auth 新增 healthcheck（start_period 40s）；**ai-service 段（261-304）**——消费 chat_group_message（group ai_service_group）、CHAT_SERVICE_URL、QDRANT_URL、SEARXNG_BASE_URL、COST_DB_NAME=ai_audit、AI_USER_ID=ai-assistant、AI_RESEARCH_USER_ID=ai-research；chat 用 CHAT_GROUP_ID=chat-prod；restart: always。
- `docker-compose.chat.yml`（独立 chat 节点）：mongodb-chat 开 --auth；KAFKA/REDIS/AUTH 地址全部来自宿主环境变量（无默认）。
- `docker-compose.edge.yml`（edge/auth 节点）：redis --requirepass（healthcheck 必须带 -a）；mongodb-auth --auth；auth-service 的 MONGODB_URL **内嵌凭据** + SECRET_KEY/SMTP（QQ 邮箱 587）；只暴露 gRPC；nginx 挂 nginx-edge.conf 与前端构建产物。
- `docker-compose.ai.yml`（ai 节点）：qdrant 6333/6334；searxng；ai-service——QDRANT_COLLECTION=long_term_memory、成本审计表 llm_api_costs、SILICONFLOW 双 key、HTTP_PROXY。
- 四个 Dockerfile：chat/orchestrator（Go 两阶段：CGO_ENABLED=0 静态编译、GOPROXY 国内源、先 COPY go.mod 再 download 利用层缓存、GOWORK=off、-ldflags="-w -s"；chat 额外 COPY infrastructure_sdk + config.yaml）；auth（Python 两阶段：venv、PYTHONPATH 含两个 proto 目录、**直接 COPY 预生成的 pb2.py 而非现场 protoc**、CMD 一行起三个进程：token_auth_server & last_offline_time_server & uvicorn）；ai（Python 单阶段：无 venv 无镜像加速，CMD python main.py 长驻消费者）。
- `scripts/init-multiple-databases.sh`：set -e + || true；for 循环建 ai_audit/user_service（**硬编码，与 compose env 无关**）；幂等靠 2>/dev/null || true。
- `searxng/settings.yml`：use_default_settings 基底；limiter: false（AI 高频调用防 429）；**formats 必须保留 json**（ai_service 拉结构化结果）。

**🎯 目标**：说出开发环境"一条命令起哪些容器、谁连谁"；看懂一个 Dockerfile 的构建阶段。

**🔑 重难点**
1. 四种 compose 的拓扑差异（全量/chat 节点/edge 节点/ai 节点）。
2. init 脚本机制与装饰性 env 的坑。
3. auth Dockerfile 三进程启动与 pb2 拷贝。
4. Go 构建的缓存与 GOWORK 技巧。

**📝 自测题**
1. 开发环境包含哪些容器？端口与依赖关系？
2. POSTGRES_MULTIPLE_DATABASES 真的在建库吗？脚本实际建哪两个库？幂等靠什么？
3. auth Dockerfile 启动哪三个进程？PYTHONPATH 为何必须含 proto 目录？
4. 为什么 chat Dockerfile 要 GOWORK=off 且 COPY infrastructure_sdk？
5. 生产与开发 compose 的差异（至少 3 点）？
6. searxng 为什么必须保留 json 格式？limiter 为什么关？

**🗣 验收提示词**
> 你是 Docker 入门陪练考官。我已读完 docker-compose 全部五个文件、四个 Dockerfile、init 脚本与 searxng 配置。请考核我：① 四种 compose 拓扑与容器依赖；② networks/volumes/depends_on 语义；③ 两个 Go Dockerfile 的构建技巧；④ auth 三进程与 pb2 拷贝；⑤ init 脚本机制；⑥ 环境变量传递链。逐题讲解，讲基础。

- 完成：☐ 日期：____

---

### 8.2 nginx 与生产部署

**⏱ 前置概念**
- nginx WS 反代三件套：proxy_http_version 1.1 + Upgrade $http_upgrade + Connection "upgrade"——缺一 WS 握手必失败；proxy_read/send_timeout 3600s 防长连接被掐。
- ip_hash 粘性：同一客户端固定落到同一后端——WebSocket 的 Hub 状态才能一致。
- SPA 回退：try_files $uri $uri/ /index.html——/dashboard 直接刷新也能命中。

**📄 文件与函数指引**
- `deploy/nginx/README.md`（蓝图）：HTTPS→nginx；/api/v1/auth/*→auth 9030；/api/v1/*→chat 8080；/api/v1/ws→WS；其余→前端静态；多节点用 ip_hash；前端 baseURL 相对路径约定。
- `deploy/nginx/chat-loadbalancer.conf`：upstream chat_backend（ip_hash + chat-1:8080/chat-2:8080）；location /api/v1/ws 三件套 + 3600s。
- `.deploy/nginx-chant.conf`（80 端口边缘主配置）：/api/v1/auth/ → 本机 9030；/api/v1/ws 与 /api/v1/ → **远程 47.106.81.230:8080**；**`proxy_set_header Host $proxy_host` 而非 $host——阿里云未备案域名 Host 拦截规避（注释明示）**；try_files SPA 回退。
- `.deploy/nginx-chantagent-https.conf`：443 ssl http2 + certbot 证书路径；location 与 chant.conf 逐字相同（80/443 两阶段切换）。
- `deploy/scripts/deploy-chat.sh`：git pull 重试 3 次（--ff-only）；**2G 内存机：GOFLAGS=-p=1 + GOMAXPROCS=2 串行编译防内存尖峰**；go build -ldflags="-w -s"；systemctl restart chant-chat。
- `deploy/scripts/deploy-ai.sh`：独立 venv pip 安装（清华源）；systemctl restart chant-ai。
- `deploy/scripts/deploy-edge.sh`：auth pip 更新；前端 npm ci || npm install + VITE_WS_URL 构建期注入 + rsync -a --delete 同步 dist；nginx reload（不中断连接）。
- `.deploy/run-auth.sh`：PYTHONPATH 含两个 proto 目录；两 gRPC 后台 & + exec uvicorn 前台接管（exec 让 uvicorn 成为 PID 1 替身，systemd 才能管理）。
- `.deploy/chant-auth.service`（systemd）：After 依赖；User=ubuntu；EnvironmentFile=/etc/chant/auth.env（**密钥不进 git**）；Restart=always + RestartSec=3；WantedBy=multi-user.target 开机自启。

**🎯 目标**：说清生产三机拓扑、nginx 反代与 WS 三件套、systemd 自启、部署脚本流程。

**🔑 重难点**
1. WS 反代三件套与超时。
2. $proxy_host 备案规避（为什么不能转发真实 Host）。
3. 低内存编译优化。
4. systemd 的 EnvironmentFile 密钥管理与 exec 技巧。

**📝 自测题**
1. 生产拓扑：三台机器各部署了什么服务？
2. nginx 反代 WS 与普通 HTTP 的配置差异？三件套缺一会怎样？
3. 为什么用 Host $proxy_host 而非 $host？改回来会触发什么？
4. deploy-chat.sh 为什么设 GOFLAGS=-p=1？
5. run-auth.sh 为什么用 exec uvicorn？
6. systemd 单元里密钥放在哪？为什么？

**🗣 验收提示词**
> 你是部署入门陪练考官。我已读完 deploy 与 .deploy 全部配置和脚本。请考核我：① 生产三机拓扑；② WS 反代三件套与超时；③ 备案 Host 规避；④ systemd 服务定义与密钥管理；⑤ 部署脚本流程与容错；⑥ exec 与进程管理。逐题讲解，讲基础。

- 完成：☐ 日期：____

---

## 三、附录 A：通用陪练提示词模板（不想用每节定制版时）

> 把【】替换后发给任何 AI。

```
你是 Chant 项目（Go+Python 微服务聊天/Agent 平台）的陪练考官，我的面试目标是初中级开发。
我刚逐句读完：【第 X 节 名称】，文件清单：【...】。
请按这个方式考核我：
1. 先出 5 道重难点题目，覆盖概念理解 + 代码细节 + "为什么这么设计"；
2. 等我作答后再逐题点评，指出错误、含糊、遗漏；
3. 最后让我用 200 字以内"像给同事讲代码"的方式总结本节代码在项目中的职责；
4. 给出 1 个面试官可能追问的扩展问题并讲解。
```

## 四、附录 B：面试初中级高频考点（本项目可讲素材）

| 考点 | 对应章节 | 一句话答法 |
| --- | --- | --- |
| Go 并发模型 | 3.2 | 读写双泵 + channel 驱动的 Hub，goroutine 间通过 channel 通信 |
| Kafka 顺序性 | 2.1/3.3/5.1 | 同 key 同分区：群聊奇偶分区、私聊/AI 回复按用户 ID |
| 消息幂等 | 3.4/7.3 | Mongo $setOnInsert+$ne 条件 push；前端 seenMessageIdsRef 去重 |
| 分布式事务 | 6.2 | Saga 状态机 + 逆序补偿 + 乐观锁 + 租约锁 + PostgreSQL 持久化 |
| JWT 与认证 | 4.2/3.5 | access+refresh 双 token，refresh 校验链防 access 换新，gRPC 远程校验 |
| 装饰器工厂 | 5.2 | @register 注册表，新增模型三步零改动 |
| RAG 混合检索 | 5.5 | 语义分×0.6 + 时间衰减×0.4，粗召回×4 再 rerank 精排 |
| 多智能体编排 | 5.6 | LangGraph 九节点图 + 审核-修订循环 + 纯代码 finalize |
| 越权防护 | 3.3/3.5 | sender 取登录态 + 群成员校验 + 群主 403 |
| WebSocket 心跳 | 3.2 | pingPeriod(50s) < pongWait(60s) 防死连接 |
| SSRF 防护 | 5.5 | 协议→IP→DNS 解析→禁重定向 四层校验 |
| 流式协议 | 1.1/5.6/7.3 | AiReplyDelta 四 kind + seq，前端按 message_id 累积 |

## 五、附录 C：Go / Python 语法自查清单（0.2 节用）

**Go（在项目里逐个找例子确认）**
- [ ] goroutine：`go func() {...}`（ws_handler.go 到处都是）
- [ ] channel 创建/发送/接收/关闭（ws_hub.go 的 Send/register/unregister）
- [ ] select + ticker 定时器（ws_hub.go WritePump）
- [ ] sync.Once 单例（GetWSHub）、sync.RWMutex（clients map）
- [ ] defer 关闭资源（ws_connection.go）
- [ ] 结构体 + json 标签 + 指针接收者（models 系列）
- [ ] interface{} / 类型断言（saga 的 map[string]any）
- [ ] context.WithTimeout（ws_handler.go 的 gRPC 调用）
- [ ] range-over-int（orchestrator 6.2 的 for retry := range maxSendRetries）

**Python（同样逐个找例子）**
- [ ] async def / await / asyncio.gather / to_thread / create_task（ai_service 各处）
- [ ] 装饰器工厂与装饰器（shared/llm/factory.py 的 @register）
- [ ] @asynccontextmanager（auth_service main.py lifespan）
- [ ] TypedDict / dataclass / 类型注解 / Optional（agent/schemas.py、shared/models.py）
- [ ] pydantic-settings 环境变量映射（config/settings.py）
- [ ] 闭包与 nonlocal（chat/service.py 的 _flush）
- [ ] 日志模块 logging（各服务）

---

> 💡 读完整个计划后：把附录 B 当"项目讲述大纲"，用 5 分钟把整个项目讲给 AI 听，让它挑刺——这是面试前最好的模拟。

> 📌 已知代码坑位速查（读到时留意，面试可主动讲"我发现过这些问题"）：桶满静默丢消息(3.4)、私聊按缺失字段排序(3.4)、手写单例无锁(3.3)、gRPC 失败缓存 nil(3.3)、中间件匿名结构体断言(3.5)、JWT 30 天 vs 库 365 天(4.1)、_normalize_user_id 漏调用(4.1)、asyncio.run 重复调用(4.2)、sliding_window.py 不存在实际在 context.py(5.3)、user_id 被注释吞掉(5.5)、knowledge_text 重复占位符(5.6)、ServiceName vs Name Topic 不一致(6.2)、POSTGRES_MULTIPLE_DATABASES 装饰性(8.1)。