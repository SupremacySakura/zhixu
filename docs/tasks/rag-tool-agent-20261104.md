---
task_id: rag-tool-agent-20261104
status: completed
created: 2026-11-04
last_reviewed: 2026-11-04
applicable_to: "RAG、检索工具化、Agent、提示词、Mastra"
---

# RAG 检索工具化与助手提示词重设计

## 背景

知识库助手对「你好，请用一句话介绍你能做什么」回复「知识库中没有找到相关内容」。根因是 `apps/zhixu-ai/src/mastra/rag-chat.ts` 的固定三段流水线（转写 → 检索 → 回答）：提示词无身份声明、无闲聊概念，任何输入都被强制检索，空结果触发机械拒答。且检索是写死的独立步骤，模型无法决定何时检索、检索几次。

## 结果

- 检索改为工具：删除转写 agent 与 REWRITE_PROMPT，新增 `search_knowledge` 工具（`createTool`，请求级闭包，注入该次请求的 RAG 凭据与 AbortSignal），由模型自主决定是否调用、调用几次。
- 单份 SYSTEM_PROMPT 作为「何时/如何调用工具」的指引：身份声明（「知序」知识库助手）、何时检索（资料问题必先检索；寒暄/问能力直接回答）、如何检索（query 精炼、指代展开、每轮最多三次）、如何回答（[n] 引用、部分可答、无命中如实告知、推测标注）。
- `maxSteps` 从 1 放开到 8（每次工具调用占 1 step，约允许 3 次检索 + 最终回答）。
- 引用卡片来源从固定检索结果改为监听 fullStream 的 `tool-call`/`tool-result` 事件：按 `knowledge_id:section` 去重、保序合并（顺序即 [n] 编号依据），每次检索落盘后即时推送 `sources` 事件（时序从"回答前"变为"回答中途"）；从未检索（闲聊）时回答末尾补发空 sources，前端契约与 UI 零改动。
- 工具内检索失败返回 `{ sources: [], error: '检索服务暂不可用' }` 而非抛出，由模型如实告知用户；模型调用失败的错误提示追加「若所用模型不支持工具调用，请更换支持的模型后手动重试」。
- `conversation-storage.ts` 的 `persistedAssistantId` 从「要求恰好一条 assistant 消息」改为取最后一条（多步运行会产生多条，可见答案是最后一条；plain-chat 单步场景兼容）。
- DSML 泄漏防护：网关未把模型输出的 DSML 工具调用标记解析为结构化 `tool_calls` 时，标记会作为普通文本流进回答（实测「什么是香蕉手机」直接展示了 `<｜｜DSML｜｜ calls>` 原文）。流循环累积文本，前 600 字符内检测到 `DSML` 即中止 run、不再转发 delta，并以明确错误提示用户检查网关的 function calling 解析能力；顺带修正了「error 分块会被误报为超时」的消息选择顺序。
- 真正根因在 Go 内部聊天网关：`internal/handler/chat.go` 的 `Gateway` 只转发白名单字段，`tools` 被主动剥掉（原设计「no tools or remote resources」）。Mastra 把工具定义放在请求体里，但 DeepSeek 官方 API 从未收到 `tools`，模型只在系统提示词里知道有个 `search_knowledge`，于是用训练格式（DSML）在正文里"假装调用"。修复：`ChatCapability` 增加 `Tools` 字段，RAG 轮次（`prepared.Rag && h.Rag != nil`）置 true；网关在该能力下放行 `tools`/`tool_choice`/`parallel_tool_calls`，工具定义仍由 Mastra 生成、调用由其执行，客户端不可注入。

## 决策与理由

- 参考 khoj / Dify / FastGPT 的提示词实践：身份与能力声明 + 场景穷举（可答/部分可答/无关）+ 闲聊免检索路由。 khoj 用独立路由步骤把寒暄导向 source:["default"]，本次直接由工具描述 + 系统提示词完成路由，少一次模型调用。
- 工具做成请求级闭包而非全局注册：RAG 检索 URL 与凭据随每次请求传入，避免全局状态串用户。
- 不做浏览器端任何改动：`sources` 事件形状 `{type, query, items}` 保持不变，仅时序变化（前端本就异步渲染引用卡片）。
- 不支持 tool calling 的模型：Mastra/模型侧会报错，错误提示引导用户换模型，不做降级回退（用户决策 m01177）。

## 教训

- `persistedAssistantId` 的「恰好一条 assistant 消息」假设与 maxSteps:1 深度耦合，放开 maxSteps 前必须先改这里——`response.dbMessages` 在多步运行下有多条 assistant 消息。
- 工具调用解析是分层的：Mastra/AI SDK 只解析 OpenAI 标准格式的结构化 `tool_calls`，不负责把模型原始生成（DeepSeek 的 DSML 标记）转成结构化调用；这一步是模型服务端/网关的职责（DeepSeek 官方 API 服务端解析；自托管 vLLM/SGLang 需启用对应 tool-call parser）。网关没解析时，从 Mastra 看只是普通文本，不会报错——所以应用层必须自己兜底检测。
- 中间网关的字段白名单是最容易断链的地方：请求体里的 `tools` 被剥掉不会报错，只会表现为模型"自己发明"工具格式。排查工具调用问题时先确认工具定义是否真的到达了模型服务端。
- zhixu-ai 测试用 `node --experimental-strip-types` + `registerHooks` mock `@mastra/core/*`；`@mastra/core/tools` 未 mock，走真实 `createTool` 实现，无需改 mock。
- `mastra build` 在 dev server 运行时会拒绝执行（提示 PID 冲突），本次以 typecheck + 测试验证，build 未跑。

## 候选特性

- 多次检索时把每次 query 也透传给前端展示（当前 sources 事件只带最后一次 query）。
- 抽屉/面板类交互组件抽象 composable（`useDrawerPanClose`）时，可一并补 role=dialog/aria-modal/Esc 与焦点管理。
- 若网关确实无法解析 DSML，可考虑在 zhixu-ai 侧自己做文本级 DSML 解析并代执行工具（成本高、易碎，暂不做）。

## 验证

- `pnpm test`：15/15 通过（含新增 DSML 泄漏防护用例；更新 persistedAssistantId 用例：多 assistant 消息取最后一条）。
- `pnpm typecheck` 通过；`pnpm build` 未执行（dev server 占用）。
- Go 后端：`go build/vet/test ./...` 全绿；新增 `TestChatGatewayForwardsToolsForRagCapability`（Tools 能力放行 `tools`/`tool_choice`/`parallel_tool_calls`、模型 ID 仍被覆盖、SSE `tool_calls` 原样透传）；原安全用例继续断言无 Tools 能力时剥离 `tools`。
- 未做真机验证：需重启 Mastra 后实测三类输入——知识性问题（触发检索+引用）、闲聊（不检索）、无命中问题（友好拒答）；网关不支持工具调用时应看到明确错误提示而非原始标记。
