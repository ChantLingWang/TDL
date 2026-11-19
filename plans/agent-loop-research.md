# Agent Loop / Harness 调研报告

> 日期：2026-11-13
> 目的：为 Chant `services/ai_service/runtime/agent_loop` 的设计做前置调研
> 范围：通用 agent harness 流程；DSH、Claude Code、OpenAI Agents SDK、LangGraph、ReAct 对比；落到本项目的建议

## 0. 结论先行

1. Harness 的本质不是“框架”，而是模型外面的一层运行时：**组装上下文、调用模型、解析工具调用、执行工具、把结果写回上下文、决定何时停**。
2. 所有成熟 harness 的循环都可以归成一句话：**模型输出若含工具调用，就执行并追加 observation，再请求模型；若不含，就结束**。差异主要在上下文管理、工具执行流水线、状态持久化、取消/审批、子 agent。
3. DSH 把这件事做到了“事件溯源 + 轮次/步骤状态机 + 工具流水线 + 压缩 + 子 agent”的完整形态；Claude Code 强调 gather context / take action / verify results；OpenAI Agents SDK 是显式 Runner loop + handoff；LangGraph 把循环画成状态图；ReAct 是最小原型。
4. 对 Chant 来说，不需要一上来做事件溯源。现有 `llm_relay` + `llm_tools` + `memory/context` + Kafka 已经覆盖了模型、工具、跨轮记忆、事件输出；缺的是 **runtime/agent_loop** 这一层“有界循环 + 工具执行 + 停止条件 + 事件映射”。
5. 最大技术前提：当前 `llm_relay.stream()` 不支持 tools，且会丢弃流式 `tool_calls`。第一版循环必须用 `complete()` 做工具轮；要保留打字机效果，需要扩展 `stream()` 支持 tools/tool_call 增量，或接受“最终答案一次性返回”的折中。

## 1. 什么是 Harness / Agent Loop

Anthropic 把 agentic system 分成两类：**workflow**（LLM 和工具由预定义代码路径编排）和 **agent**（LLM 动态决定流程和工具使用）。Agent 的典型实现就是“LLM 在循环里根据环境反馈使用工具”，并设置停止条件（如最大迭代次数）。([Anthropic](https://www.anthropic.com/engineering/building-effective-agents))

Claude Code 文档把外围层称为 **agentic harness**：模型负责推理，工具负责行动，harness 提供工具并管理模型看到的上下文；循环是 **gather context → take action → verify results**，三个阶段反复交织。([Claude Code](https://code.claude.com/docs/en/how-claude-code-works))

ReAct 是最小学术原型：让模型交替产生 **Thought（推理）→ Action（工具调用）→ Observation（环境返回）**，用 observation 纠正下一步。([ReAct](https://arxiv.org/abs/2210.03629))

因此，harness 可以拆成：

```
用户输入
  → 上下文组装（system / 工具目录 / 历史 / 记忆 / 运行时信息）
  → 模型调用（流式）
  → 输出分类
      ├─ 最终回答 → 结束
      ├─ 工具调用 → 校验/执行/追加结果 → 回到模型
      ├─ handoff / 子 agent → 切换执行者 → 回到模型
      └─ 需要人类输入/审批 → 暂停/恢复
  → 停止条件 / 预算 / 取消
```

## 2. 通用流程

### 2.1 两层循环

- **外层（turn/session）**：一条用户消息或一次任务，可能包含多个模型步骤；负责会话历史、持久化、恢复、取消、steering。
- **内层（step/tool round）**：一次模型请求 + 一批工具执行；负责上下文、工具调用、observation、预算。

OpenAI Agents SDK 的 Runner 是典型内层循环：

1. 用当前 agent 和 input 调 LLM；
2. 如果输出是 final output，结束；
3. 如果请求 handoff，切换 agent，继续；
4. 如果产生 tool calls，执行、追加结果、继续；
5. 超过 `max_turns` 抛 `MaxTurnsExceeded`。([OpenAI](https://openai.github.io/openai-agents-python/running_agents/))

LangGraph 则把同样的循环建模成状态图：`agent` 节点调模型，条件边检查是否有 `tool_calls`，有则去 `tools` 节点，执行完回到 `agent`；没有则结束；用 recursion limit 兜底。([LangGraph](https://reference.langchain.com/javascript/langchain-langgraph/prebuilt/toolsCondition))

### 2.2 通用伪代码

```python
async def run_turn(user_input, session):
    messages = assemble_context(session, user_input)
    toolbox = Toolbox(session)
    for step in range(max_steps):
        result = await llm.complete(messages, tools=toolbox.specs())

        if not result.tool_calls:
            return finalize(result)

        messages.append(assistant_message(result.content, result.tool_calls))
        for call in order_calls(result.tool_calls):
            output = await toolbox.call(call.name, call.arguments)
            messages.append(tool_message(call.id, output))

        if budget_exceeded():
            break
    return force_final(messages)
```

关键点：

- assistant 的 `tool_calls` 消息必须先追加，再追加对应 `tool` 结果；
- 每个 `tool_call` 必须有且只有一个结果，错误也要变成结果；
- 工具结果写回后，下一次模型调用才能“看到”它；
- 循环必须有人为上限，不能只靠模型自觉。

### 2.3 停止条件

成熟 harness 的停止条件是分层的：

1. 模型返回不含工具调用的最终文本；
2. 达到最大步骤/轮次（OpenAI `max_turns`、LangGraph recursion limit）；
3. token/成本/时间预算耗尽；
4. 用户取消/中断；
5. 审批被拒绝或安全护栏触发；
6. 不可恢复的工具/模型错误；
7. handoff 到另一个 agent（对本 agent 是结束，对系统不是）。

DSH 默认没有内置轮次预算，明确说“限制失控轮次的策略必须从生命周期扩展点执行取消”；这是有意的设计——预算属于部署策略，不属于循环内核。（DSH 本机 README：`@deepseek-ai/dsh-agent-loop`）

## 3. Harness 的七个核心子系统

### 3.1 模型网关

- 统一消息/工具 schema；流式 token、reasoning、tool_calls、usage、finish_reason；
- provider 适配、重试、错误码、上下文窗口校验；
- 请求不可变，便于重放和审计。
- Chant 对应 `llm_relay`。

### 3.2 上下文组装

- system prompt、工具目录/schema、历史、长期记忆、运行时上下文（cwd、时间、权限）；
- 顺序稳定，利于 KV cache；
- 动态部分（工具 schema、记忆）要标注来源，便于压缩。
- Chant 对应 `apps/chat` + `memory/context` + `llm_tools.Toolbox.catalog()`。

### 3.3 工具注册与执行流水线

- 注册表：name → 定义（schema + handler + metadata）；
- 参数校验、权限/审批、超时、取消、并发/独占屏障；
- 结果规范化、错误结构化、call/result 配对；
- 工具失败不应结束整个 turn，而是变成 observation 让模型恢复。

DSH 的流水线：`tools/pre-execute` → guards → `tools/execute` → `tools/post-execute` → finalize → `tools/result`；并发默认上限 10，独占调用形成屏障。（DSH 本机 README：`@deepseek-ai/dsh-tools`、`@deepseek-ai/dsh-agent-loop`）

Chant 对应 `llm_tools`，但目前只有注册/加载/执行，没有权限、超时、并发。

### 3.4 会话状态与持久化

- 两种流派：
  - **消息列表**：简单，直接存 messages；恢复时重放；
  - **事件溯源**：append-only session log，`deriveMessages()` 投影出模型历史；支持 fork、审计、压缩不删历史。
- DSH 是事件溯源：Session 是类型化事件日志，模型历史、transcript、标题、遥测都从日志派生；压缩只遮蔽、不删除。（DSH 本机 README：`@deepseek-ai/dsh-session`）
- Claude Code 用 JSONL 会话文件，支持 resume/fork。([Claude Code](https://code.claude.com/docs/en/how-claude-code-works))
- Chant 的真相在 chat_service 的 Mongo 历史 + `memory/context` 的折叠检查点；ai_service 不需要再建一套事件日志。

### 3.5 上下文压缩

- 触发：token 压力、context overflow、手动 `/compact`；
- 做法：把较早范围替换成一条摘要消息，保留近期 tail；压缩边界不能切断未完成的 tool_call/result 对；
- DSH：`compaction/start` → summary → `user/message` replace → `compaction/end`；摘要消息带标记，原始事件保留。（DSH 本机 README：`@deepseek-ai/dsh-compaction`）
- Chant 已有 DSH 式折叠：`memory/context.assemble_context()` 按 0.8/0.16 预算折叠，检查点写 Mongo。agent loop 内部还需要处理“单轮工具结果过大”的问题。

### 3.6 流式事件协议

- token 级 content/reasoning；工具调用开始/参数/结束；步骤进度；最终 done；
- 实时事件用于 UI，持久消息用于回放；两者不能混为一谈。
- Chant 的 Kafka delta 目前有 `progress` / `thinking` / `content` / `done`；工具调用可以先用 `progress` 文本表达，或新增 kind（需要前端配合）。

### 3.7 护栏、可观测与成本

- 审批/沙箱/权限、URL/命令安全、输出大小限制；
- 每步 token/成本、工具延迟、失败率、循环步数；
- 失败恢复：重试只针对可重试错误；不要在已输出部分文本后重试。
- Chant 有成本表，但 agent loop 需要按“整条消息”累计多次模型调用。

## 4. 现有 Harness 对比

| 维度 | DSH | Claude Code | OpenAI Agents SDK | LangGraph | ReAct |
|---|---|---|---|---|---|
| 控制流 | turn/step 状态机 + 工具轮 | gather/action/verify 循环 | Runner 循环 | agent↔tools 状态图 | Thought/Action/Observation |
| 状态 | 事件溯源 Session，可 resume/fork | JSONL 会话，可 resume/fork | RunState/Sessions | graph state + checkpointer | 无 |
| 工具 | 注册表 + 流水线 + 并发/独占 | 内置工具 + MCP/skills | function tools + handoffs | ToolNode | actions |
| 上下文 | system 节点 + runtime context + compaction | context window + 自动压缩 | sessions + context management | state messages | prompt |
| 人类介入 | inbox steering/followup/审批 | 随时 interrupt/steer | HITL/approvals | interrupt | 无 |
| 停止 | 模型 final、cancel；无内置轮次预算 | 任务完成/interrupt | final、max_turns、handoff | 无 tool_calls、recursion limit | max iterations |
| 流式 | token chunks + 事件 | 流式 | streamed events | 流式 | 无 |
| 子 agent | 有（命名、深度、容量） | 有（subagents） | handoffs | subgraphs | 无 |

补充：

- DSH 的工具批次保持模型顺序结果，并行安全工具用有界滚动池，独占工具形成屏障；失败步骤会为每个未完成的 tool_call 补错误结果（`TOOL_OUTCOME_UNKNOWN` / `TOOL_NOT_STARTED`），避免下一次请求出现“有调用无结果”的非法历史。
- Claude Code 的文档强调 harness 是“模型外面的层”，负责工具和上下文；会话/压缩/子 agent 都是这一层的扩展。
- OpenAI Agents SDK 的 final output 判定很直接：有文本、且没有 tool calls；handoff 是另一种“继续循环”的分支。
- LangGraph 把“是否继续”显式建模成条件边，适合需要可视化/可恢复状态图的场景。
- ReAct 是最小原型，不含持久化、并发、审批、压缩；适合理解循环本质，不适合直接做生产 harness。

## 5. 对 Chant ai_service 的映射

### 5.1 现状

```
Kafka MessageSent
  → main.dispatch
  → apps/chat.handle_message
      → chat_service 拉历史
      → memory/context.assemble_context（DSH 式折叠）
      → llm_relay.stream（单次，无工具）
      → Kafka AiReplyDelta（progress/thinking/content/done）
      → Kafka AiReplyGenerated（最终回复）
      → 成本表
```

已有：

- `llm_relay.complete()` 支持 tools，provider 已能解析 `tool_calls`；
- `llm_tools` 已有注册表、目录、`Toolbox`、`load_tools`；
- `memory/context` 已有跨轮压缩；
- Kafka 已有 delta 协议和成本记录。

缺口：

- 没有循环：模型不能“调工具 → 看结果 → 再调”；
- `llm_relay.stream()` 不接收 tools，且流式 `tool_calls` 被丢弃；
- 没有步骤/工具/成本预算；
- 没有工具执行的事件映射、错误配对、超时/取消；
- 没有把 `Toolbox` 接进 `handle_message`。

### 5.2 建议的循环

```
handle_message
  ├─ 拉历史 + assemble_context
  ├─ toolbox = Toolbox(ctx)
  ├─ system_prompt += toolbox.catalog()
  ├─ loop:
  │    result = llm.complete(messages, tools=toolbox.specs())
  │    if not result.tool_calls:
  │         final = result.content; break
  │    messages.append(assistant(content, tool_calls))
  │    for call in order(result.tool_calls):
  │         output = await toolbox.call(call.name, call.arguments)
  │         messages.append(tool(output, call.id))
  │    if budget_exceeded: force_final()
  └─ 发送最终回复 + 成本
```

工具顺序：

1. `load_tools` 先执行（它会让后续工具的 schema 进入下一轮 `specs()`）；
2. 其余工具按模型顺序串行（v1）；只读工具可后续并行，但结果必须保持模型顺序；
3. 每个 call 必须有 result，包括未知工具、未加载、参数错误、执行异常。

### 5.3 建议接口

```python
@dataclass
class LoopConfig:
    max_steps: int = 8
    max_tool_calls: int = 16
    max_seconds: float = 120
    max_tool_result_chars: int = 8000

@dataclass
class ToolTrace:
    name: str
    arguments: dict
    duration_ms: int
    result_chars: int
    error: str = ""

@dataclass
class LoopResult:
    text: str
    reasoning: str
    usage: Usage
    steps: int
    tool_traces: list[ToolTrace]
    stop_reason: str

class AgentLoop:
    def __init__(self, llm, toolbox, config, on_event=None): ...
    async def run(self, messages, system_prompt, task="chat") -> LoopResult: ...
```

`on_event` 或 async generator 把事件映射到 Kafka delta。建议用 async generator，`apps/chat` 直接 `async for`。

### 5.4 流式方案的取舍

先澄清一点：**工具调用本身不需要 stream**。`complete()` 返回的 `LLMResult` 里已经有完整的 `tool_calls`，解析、执行、追加结果都不依赖流式。这里的 stream 只影响“模型生成的内容能否边生成边展示给前端”，不影响工具能不能调。

**方案 A：全程 complete（最快落地）**

- 工具轮和最终回答都用 `complete()`；
- 最终 `content` 一次性发给前端，或按小段切分模拟打字机；
- 优点：不改 `llm_relay`；缺点：失去真实流式，reasoning 也只能最后一次性给。

**方案 B：扩展 stream 支持 tools（要保留打字机时推荐）**

- 在 `chat_completions.py` 解析 `delta.tool_calls`，按 index 累积；
- 在 `llm_relay.types` 增加 tool call 事件；
- 循环每一步都 `stream()`，边流边决定是否执行工具；
- 优点：保留打字机、思考链、工具进度；缺点：要改 provider 和事件协议。

**方案 C：混合**

- 工具轮用 `complete()`，最终回答再 `stream()` 一次；
- 简单，但最终答案会重新生成一次，成本和延迟翻倍，且可能和 complete 的内容不一致。不建议。

建议：第一版用 A 验证循环/工具/预算；紧接着做 B，作为正式体验。

### 5.5 上下文与预算

- 系统提示 + 工具目录在循环开始时组装一次；`load_tools` 后 `specs()` 变化，但目录可以保持（重复 load 返回 already）。
- 工具结果按 `max_tool_result_chars` 截断，保留头尾；超长结果可让工具自己分页。
- 用 `memory/context.estimate_tokens` 估算每轮 messages；接近窗口时，优先裁剪最老的 tool 结果，再考虑压缩。
- 预算：
  - `max_steps`：防止模型无限调工具；
  - `max_tool_calls`：防止一次并行爆炸；
  - `max_seconds`：防止慢工具拖死 Kafka consumer；
  - token/cost：按累计 usage 计算，超限强制 final。
- 达到预算时，追加一条 developer/system 提示“不再调用工具，基于现有信息回答”，然后做一次无 tools 的 final 调用。

### 5.6 事件映射

| 循环事件 | Kafka delta kind | 内容示例 |
|---|---|---|
| 开始 | progress | 正在思考… |
| reasoning token | thinking | 模型思考增量 |
| tool call start | progress | 正在调用 web_search… |
| tool result | progress | web_search 返回 5 条结果 |
| final token | content | 最终回答 |
| 结束 | done | 空 |

如果前端要展示工具卡片，新增 `tool_call` / `tool_result` kind，需要 proto/前端同步；v1 先用 progress。

### 5.7 错误、取消、幂等

- 模型错误：`llm_relay` 已做 HTTP 重试；循环层只对“尚未产生任何输出”的错误重试一次，已输出后不重试。
- 工具错误：`Toolbox.call` 已捕获异常并返回 JSON 错误；循环把它作为 tool message，让模型决定是否重试。
- 取消：`asyncio.CancelledError` 时停止循环；已发出的 tool_call 要补 aborted 结果，避免历史不配对（如果消息不持久化，可简化）。
- 幂等：Kafka 可能重投；建议用 `message_id` 做去重（Redis/Mongo），否则同一条消息可能回复两次。
- 空回复：force final 后仍为空，走 `send_error_reply`。

### 5.8 观测与成本

- `LoopResult` 记录 steps、tool_traces、usage；日志输出每一步的模型/工具耗时。
- 成本：把多次 `complete` 的 usage 累加，最后统一 `insert_cost`（当前只记一次 stream 的 usage）。
- 可选：把 tool_traces 的摘要放进 `metadata`，便于排查；不要把完整工具输出写进最终消息。

## 6. 分阶段落地

**Phase 1（最小闭环）**

- `runtime/agent_loop/loop.py`：`complete` 循环、工具执行、call/result 配对；
- 接入 `apps/chat.handle_message`，用 `Toolbox`，system prompt 拼 catalog；
- `max_steps` / `max_tool_calls` / 超时；
- 测试：无工具、单工具、load_tools→工具、未知工具、工具异常、max_steps。

**Phase 2（流式）**

- 扩展 `llm_relay.stream` 支持 tools 和 tool_call 增量；
- 循环改为流式，事件映射到 Kafka；
- 保留 reasoning 增量。

**Phase 3（上下文与并发）**

- 工具结果截断/裁剪；token 预算；
- 只读工具并行、独占工具屏障；
- 取消与幂等。

**Phase 4（增强）**

- 子 agent、审批、长期记忆写入、工具卡片、可观测性。

## 7. 需要拍板的问题

1. 第一版是否接受“最终答案非流式”（方案 A），还是先扩 `stream`（方案 B）？
2. `max_steps` / `max_tool_calls` / 单条消息成本上限设多少？
3. 工具结果默认截断到多少字符？超长结果是否让工具分页？
4. `load_tools` 之外，是否允许预加载（按意图/关键词）以减少一轮往返？
5. 工具调用是否要持久化（影响下一轮对话能否引用上一次工具结果）？
6. 是否现在就要并行工具执行，还是 v1 全串行？
7. 取消路径：Kafka 目前没有取消事件，是否先用超时兜底？
8. 是否需要把工具调用展示成独立 delta kind（要改 proto/前端）？

## 8. 参考

- Anthropic, Building effective agents: https://www.anthropic.com/engineering/building-effective-agents
- Claude Code, How Claude Code works: https://code.claude.com/docs/en/how-claude-code-works
- OpenAI Agents SDK, Running agents: https://openai.github.io/openai-agents-python/running_agents/
- LangGraph, toolsCondition: https://reference.langchain.com/javascript/langchain-langgraph/prebuilt/toolsCondition
- ReAct, arXiv:2210.03629: https://arxiv.org/abs/2210.03629
- DSH 本机安装包 README：
  - `@deepseek-ai/dsh-agent-loop`
  - `@deepseek-ai/dsh-agent`
  - `@deepseek-ai/dsh-session`
  - `@deepseek-ai/dsh-compaction`
  - `@deepseek-ai/dsh-tools`
  - `@deepseek-ai/dsh-llm`
  - `@deepseek-ai/dsh-subagent`
