# front_code —— AI Agent 维护规格（AGENT-FACING SPEC）

本文件供 AI agent 读取与执行。修改任何前端文件前先读本文件；
修改后必须执行「验证清单」。

## 1. 技术栈（事实，勿更改）

- React 19 + TypeScript ~5.9 + Vite 7，CSS 用 SCSS Modules（`*.module.scss`）
- 运行时依赖仅 8 个：react / react-dom / react-router-dom / axios / classnames / sass / react-markdown / remark-gfm
- **禁止**引入：状态管理库（redux/zustand）、UI 组件库、CSS-in-JS。组件状态一律 React Hooks
- 构建：`npm run build`（= `tsc -b && vite build`），产物 `dist/`（纯静态，勿提交手改产物）
- 开发：`npm run dev` → http://localhost:5173，代理：`/api` → localhost:8080（chat_service），`/api/v1/auth` → localhost:9030

## 2. 目录职责（只动对应文件）

| 路径 | 唯一职责 |
|---|---|
| `src/styles/tokens.scss` | 全部设计 token（颜色/圆角/字体/动效/布局尺寸），变量名 `--chant-*`。**换肤只改此文件** |
| `src/styles/global.scss` | 全局 reset、body 底色、滚动条、`@use './tokens.scss'` |
| `src/types/chat.ts` | 消息/会话类型的**唯一来源**。后端协议字段先在此登记，禁止在组件内内联自定义消息类型 |
| `src/api/chat.ts` | chat_service REST 封装（axios 实例 `chatRequest`，baseURL `/api/v1`，自动带 Bearer token） |
| `src/api/auth.ts` | auth REST 封装 |
| `src/hooks/useChatSocket.ts` | WS 连接/2 秒重连/统一消息出口。**WS 逻辑只在此文件与 ChatPage 回调** |
| `src/pages/ChatPage.tsx` | 装配层：全部状态与数据流（groups/messages/streams），渲染委托组件。WS 消息分发在 `handleWsMessage` |
| `src/pages/ChatPage.module.scss` | 聊天页布局（wrapper/main/header） |
| `src/pages/LoginPage.tsx` `RegisterPage.tsx` + `LoginPage.module.scss` | 登录/注册（共用样式） |
| `src/components/Sidebar.tsx` | 侧栏：AI 会话列表 + 用户区。创建逻辑回调到 ChatPage |
| `src/components/MessageList.tsx` | 消息分组（同发送者 3 分钟合并）+ 加载更多 + AI step 流渲染 |
| `src/components/StepTimeline.tsx` | 按 step 渲染 THINK / TOOL_CALL / TOOL_RESULT，按 tool id 配对调用与返回 |
| `src/components/MessageBubble.tsx` | 单条消息：头像/名称/气泡/时间/Markdown（AI 消息走 react-markdown+GFM，用户消息纯文本） |
| `src/components/ThinkingBlock.tsx` | 思考块：StepTimeline 把多个 THINK 拼成 reasoning 传入；streaming 时自动展开 |
| `src/components/ChatInput.tsx` | 输入区：Enter 发送 / Shift+Enter 换行 / 自动增高 |
| `src/components/NewChatModal.tsx` | 新建 AI 会话弹窗（只有一个入口） |
| `src/App.tsx` | 路由与 token 自动刷新（每分钟） |

## 3. 强制约定

1. **组件三件套**：`X.tsx` + `X.module.scss` 同名配对；新组件放 `src/components/`，样式全在自身 module，禁止往 global 塞组件样式
2. **颜色/尺寸**：组件内只用 `var(--chant-*)`，禁止硬编码色值（设计 token 已覆盖全部场景）
3. **协议字段**：`WsMessage` / `Group` / `SendPayload` 只增不改名；后端字段以 `docs/message-chain.md` 为准
4. **状态归属**：ChatPage 持状态，组件只收 props + 回调；组件内部状态仅限 UI 局部（弹窗开关等）
5. **文案**：界面文案统一中文；英文保留项：状态 `online`
6. **协议**：AI 只推 `type === 'ai_step'` 的 step 事件，前端按 `message_id + step` 归组渲染

## 4. 关键协议事实（改动涉及消息时必须知道）

- WS 接收：AI step `{type:"ai_step", conversation_id, reply_to_msg_id, message_id, round, step, attempt, kind, payload, time}`
- `kind` 取值：THINK / TOOL_CALL / TOOL_RESULT / RESULT；`payload` 是该步的 JSON
- 前端累积：`streams: Record<message_id, StepStream>`，按 `step` 排序；收到 RESULT 置 `done=true`，有 result 就渲染最终回答，取消/失败渲染 status + reason
- 历史接口尚未实现；接入后若返回**新 → 旧**，前端必须 `reverse()`
- 历史消息思考全文在 `metadata.reasoning`，映射到 `WsMessage.reasoning`
- 发送：`SendPayload` 结构见 `types/chat.ts`，带 parts / message_id / message_type / conversation_id；文本是 `{type:"text",text}`
- 去重：按 `message_id + step` 覆盖同一 step；旧 `attempt` / 旧 `round` 的迟到事件直接忽略

## 5. 常见修改的标准步骤

### 换主题色 / 调深色
只改 `tokens.scss` 中对应 `--chant-*` 变量（文件内均有中文注释说明用途）。

### 改某组件样式
只改该组件的 `.module.scss`；不要动其他文件；不要新增硬编码色值。

### 后端新增消息字段
1. `types/chat.ts` 的 `WsMessage` 加可选字段
2. `ChatPage.tsx` 的 `handleWsMessage` 里透传该字段
3. 对应渲染组件加一个条件分支

### 后端新增 REST 接口
在 `api/chat.ts` 的 `chatApi` 对象里按现有格式加一行方法；组件里 `chatApi.xxx().then(...)` 调用。

### 后端新增 WS 事件类型
在 `ChatPage.tsx` 的 `handleWsMessage` 里按 `metadata.kind` 或新字段加分支；
不进入消息流的轻量事件参考旧 progress 处理方式。

### 加全新 UI 面板
1. 新建 `src/components/X.tsx` + `X.module.scss`（三件套）
2. 样式用 `var(--chant-*)`，可参考 `NewChatModal` 的弹窗/面板写法
3. 在 `ChatPage.tsx` 挂载并接 props/回调

## 6. 验证清单（任何改动后必做）

```bash
cd front_code && npm run build   # 必须 0 错误（tsc 严格模式）
```

交互类改动追加冒烟：`npm run dev` 后访问 http://localhost:5173，
确认登录 → 进入 dashboard → 发送一条 AI 消息（思考块 + 分块正文 + Markdown 正常）。

## 7. 已知约束

- `ChatPage` 的 `handleWsMessage` 是 useCallback，闭包内**禁止直接读流式状态**，必须用函数式 setState
- 登录/注册页共用 `LoginPage.module.scss`，改样式同时影响两页
- `dist/` 与 `node_modules/` 不进 git
- 前端改动不向后端提出协议要求；协议变更先改 `docs/message-chain.md` 与 `proto`，再改这里
