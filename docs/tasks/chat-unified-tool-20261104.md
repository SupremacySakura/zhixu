---
task_id: chat-unified-tool-20261104
status: completed
created: 2026-11-04
last_reviewed: 2026-11-04
applicable_to: "对话、Mastra、BFF、工具调用、流式、架构迁移"
---

# 统一对话入口：检索工具化与 Go 对话网关下线（P1）

## 背景

P0 确定了目标架构（[ai-architecture.md](../ai-architecture.md)）：Go 只做 BFF，Mastra 拥有全部 AI 能力并直连 provider；对外只有 `/chat` 与 `/vectorize` 两个 AI 路由，检索是常驻工具而非流水线步骤。P1 负责落地对话侧：合并 `/plain-chat` 与 `/rag-chat`，删除 Go 的模型网关与「是否 RAG」分支，前端去掉「知识库检索」开关。

## 结果

- **Mastra**：新增 `src/lib/chat.ts`（`chatTurnSchema` strict：`version`、`model{protocol,model_id,base_url,api_key}`、`messages`、可选 `conversation`、`tools{search_url,capability}`；`allowedProviderURL` 只允许 http/https 且不含内嵌凭据）与 `src/mastra/chat.ts`（唯一对话路由）。`src/lib/rag.ts` 改为工具检索客户端：`searchToolsSchema`、`allowedSearchURL` 校验 `/internal/tools/v1/search`、`searchKnowledge(tools, query, signal)`。删除 `plain-chat.ts`、`rag-chat.ts`，`index.ts` 路由面变为 `[chatRoute, vectorizeRoute, healthRoute, ...conversationRoutes]`。
- `chatRoute` 逻辑：鉴权 → schema 校验 → provider/search URL 校验 → 会话并发保护（410 已删除会话）→ 单个 Agent（`id: chat`，模型来自 turn context）→ `maxSteps: 8` → NDJSON 事件 `delta`/`sources`/`done`/`error`。DSML 泄漏防护、引用去重保序、`persistedAssistantId` 取最后一条 assistant 消息等行为沿用。
- **Go**：`ChatInput` 删除 `RAG` 字段；`ChatCapability` 删除 `MultiStep`/`Tools`；`RagPrepared` 与 `Prepare` 整体删除；`Send` 改为每轮签发只读工具能力（`Register(&ChatCapability{Context, RagQuery})`）并 POST Mastra `/chat`，payload 携带 `version/model/messages/conversation/tools`；`Gateway` 改为 `EmbeddingGateway`（仅 embedding 能力，P2 删除）；`/internal/rag/v1/search` 更名 `/internal/tools/v1/search`。
- **前端**：删除「知识库检索」勾选框与 `ragEnabled`/`hasEmbedding`；`streamChat` 去掉 `rag` 参数（`conversationID` 前移为第 5 个参数）；`ChatMessage` 去掉 `rag` 标记，历史恢复不再推断模式；`retry` 不再携带模式。
- **测试**：Mastra `conversations.test.mjs` 全部改走 `chatRoute`（新的 turn 输入形状、凭证/URL 校验、DSML、临时对话、并发删除等 15 例全绿）；Go `chat_test.go` 重写为 `TestChatSendCarriesToolCapability` 与 `TestChatSendStreamsViaMastraRoute`，删除 3 个旧网关用例，`rag_test.go` 删除 `Prepare` 用例；前端删除 2 个 rag 模式用例、补 2 个统一链路重试用例，220 例全绿。
- **文档**：`ai-architecture.md` 更新为 P1 已落地状态（契约与实际 wire 对齐、`delta` 事件名、路由面、阶段表）；`plain-chat.md` 重写为统一对话链路；`README.md`、`frontend-structure.md` 同步。

## 决策与理由

- **Turn context 不放 `turn_id`**：幂等目前由会话行锁与 Mastra `conversationRuns` 覆盖，多一个字段就要多一处校验与穿透；等真正需要请求级重放再加。
- **`api_key` 直传每轮**：与「Go 保管凭据、Mastra 无状态」的目标架构一致；代价是每轮请求体含明文密钥，靠本机环回 + `INTERNAL_API_SECRET` + 不落日志控制暴露面。
- **`/internal/chat/v1/*` 暂存为 `EmbeddingGateway`**：P1 只删对话转发，embedding 直连属于 P2；这样 P1 可独立回滚，不用同时改向量化链路。
- **守卫顺序：先会话后检索服务**：删除会话必须返回 409 而不是 503，否则前端会把「会话已删除」误判为服务未配置。
- **前端保留 `sources`/`query` 字段**：引用卡片渲染依赖它们，删除的只是 `rag` 模式标记。

## 教训

- **契约文档与实现会漂移**：P0 文档写的是 `text-delta`/`turn_id`/`topK`，实现时按 Mastra 实际事件与 Go 现有能力落地为 `delta`/无 `turn_id`/服务端 TopK。迁移阶段必须以代码为准回写文档，否则下一阶段会照着错误契约实现。
- **测试断言要跟着事件形状走**：统一路由后即使不检索也会补发空 `sources`，原来「最后一行 JSON 就是 done」的断言必须改成取最后一行；这类隐式契约变化最容易漏。
- **`pnpm --dir` 在本仓库不可用**：要格式化其他 workspace 的文件，在仓根执行 `pnpm exec prettier` 或进入子目录执行。
- **删除函数前先搜调用方**：`Prepare` 只有 `Send` 一处调用，但 `rag_test.go` 有两个用例直接测它，先删实现会编译失败；先改测试再删实现。

## 候选特性

- P2：`/vectorize` 直连（Go `MastraEmbedder` 去 gateway/capability，删 embedding 转发路由）。
- P3：`done.usage` 逐轮记账与配额拦截；Mastra 侧工具注册表升级为 MCP server。
- `allowedProviderURL` 与 Go 的 provider 地址校验存在两处实现（Mastra 侧防内嵌凭据、Go 侧防 SSRF），后续可抽成共享规范文档，避免两边规则漂移。

## 验证

- Go：`gofmt`、`go build ./...`、`go vet ./...`、`go test ./internal/handler ./internal/service ./internal/repository` 全绿。
- Mastra：`pnpm test` 15/15、`pnpm typecheck` 通过（`mastra build` 仍被运行中的 dev server 拒绝，未执行）。
- 前端：`pnpm typecheck`、`pnpm lint`、`pnpm vitest run` 220/220 通过。
- 未执行：真机验证（需同时重启 Go 与 Mastra）：寒暄直接回答、知识问答触发工具检索、`sources` 引用编号正确、DSML 不复现。
