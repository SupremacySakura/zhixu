---
task_id: chat-history-20261024
status: completed
created: 2026-10-24
last_reviewed: 2026-10-24
applicable_to: "会话历史、Mastra Memory、@mastra/mysql、内网会话管理路由、RAG 来源落库"
---

# 会话历史：Mastra Memory 直存 MySQL + Go 鉴权转发

## 结果

- Mastra 侧 `@mastra/mysql`（MySQLStore）+ `Memory`（lastMessages: 40，关闭 semanticRecall 与 generateTitle），threads/messages 直写业务库 `zhixu`，`resourceId = "u{userID}"` 做用户隔离。
- Go 不再保存聊天消息正文：会话模式下只把**最新一条** user 消息发给 Mastra（附 `conversation: {id, resource, title, metadata}`），历史由 Memory 自动注入，避免“Go 历史 + Memory 历史”重复。
- Mastra 新增四个内网管理路由 `POST /internal/conversations[/messages|/rename|/delete]`，仅接受 `Authorization: Bearer ${INTERNAL_API_SECRET}`；所有路由先按 threadId+resourceId 校验归属，不存在即 404。
- Go 新增 `GET /chat/conversations`、`GET/PATCH/DELETE /chat/conversations/:id`（JWT 鉴权），由 `ConversationService` 携带共享密钥与 user 派生的 resource 转发 Mastra；V2 message 的 parts 扁平化为纯文本返回前端。
- RAG 检索来源无法塞进 Mastra 自动落库的 message metadata，故 Go 侧建轻表 `chat_turn_sources`（conversation_id+user_id+query+sources JSON），流结束时按 `done && rag` 落库；读详情时与 assistant 消息按创建顺序 lockstep 配对。
- 会话标题由 Go 取首条用户消息截断 50 字符，不用 Mastra generateTitle（多一次模型调用且 token 时机不可控）。
- 前端新增会话侧栏 `ConversationSidebar.vue`（列表、相对时间、行内改名、删除），`useChat` 捕获 meta 事件的 `conversation_id` 并透传续聊，`loadConversation` 还原历史；历史对话的 RAG 来源一并还原。

## 教训

- 开 Memory 后调用方必须只发增量消息：`stream(messages.slice(-1), {memory})`。若把全量历史也发过去，Memory 注入的历史与请求体重叠，上下文重复计费且可能混乱。
- Mastra 自动落库的 message 不接受自定义 metadata，凡是需要随消息还原的业务数据（如 RAG sources）必须自建关联表，按序配对是成本最低的方案。
- 内网管理路由不要信任客户端 user id：由 Go 鉴权后派生 resourceId，Mastra 只认共享密钥；secret 为空时直接拒绝服务，避免“未配置即裸奔”。
- `chatStore.stores.memory` 的 base 类型 `MemoryStorage` 缺少 MySQL 具体方法，需要断言为 `MemoryMySQL` 后再调 `getThreadsByResourceIdPaginated` 等。
- Go 统一返回 `{code,msg,data}` 包裹，前端 axios 必须取 `res.data.data`；本次错取 `res.data.conversations` 得到 undefined，侧栏渲染 `conversations.length` 抛 TypeError 卡死整个 ChatPage 渲染，表现为“列表加载中 + 聊天无回复”两个假象——子组件渲染异常会向上扩散，下游功能全部失灵。
- 前端 axios 实例无 baseURL，所有 API 路径必须带 `/api` 前缀走 Vite 代理；漏掉前缀时请求命中 SPA fallback 返回 HTML 且状态 200，数据静默为空，不会出现网络错误，排查时要先核对实际请求 URL。

## 可沉淀模式

- **AI 服务自持记忆**：业务后端不镜像聊天内容，只做鉴权与转发；用户隔离靠 resourceId 约定（`u{userID}`），读写两侧共用同一函数生成，防止不一致。
- **共享密钥内网路由**：管理面接口独立于用户面，Bearer 密钥 + 归属校验双层防护，密钥走环境变量两端共用。
- **流式落库旁路**：流式响应边透传边解析 NDJSON（Scanner 行缓冲放大到 1MB），在 `done` 事件处一次性落库，不影响透传延迟。

---

*本文件由 docs/tasks/.template.md 模板生成*
