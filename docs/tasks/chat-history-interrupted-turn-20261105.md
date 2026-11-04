---
task_id: chat-history-interrupted-turn-20261105
status: completed
created: 2026-11-05
last_reviewed: 2026-11-05
applicable_to: "对话、会话历史、Mastra Memory、多步工具调用、消息校验"
---

# 修复：恢复历史会话后继续对话报「消息角色或内容无效」

## 背景

用户反馈「对话两轮后：模型配置参数无效: 消息角色或内容无效，每条最多 8000 字符」。复现路径不是长度：打开一个含中断轮次的历史会话，再发送新消息，Go 在 `ValidateChat` 阶段拒绝请求（Mastra 没有收到任何请求，线程也没有新消息落库）。

根因在 Mastra 的落库形态：多步工具调用会把每个 step 都写成 message。若某轮在工具调用后中断（用户停止、模型/网关报错、DSML 防护 abort），最后一条 assistant 消息可能只包含 `reasoning`、`tool-invocation`、`step-start` 等 part，**没有任何 `text` part**。历史接口把这条消息渲染成空字符串返回，前端把它当作 completed 消息回传，Go 的校验要求每条消息内容非空 → 整轮请求被拒。

实际线程数据确认了这一点：同一线程前 5 条消息正常，最后一条 assistant 的 parts 为 `['reasoning','tool-invocation','step-start']`、渲染长度为 0。

## 结果

- `service.ChatMessage` 新增归一化：`NormalizeChatHistory` 丢弃内容为空的 assistant 轮次，并把因此相邻的用户消息用空行合并，保证严格 `user/assistant` 交替。由 `ChatHandler.Send` 在 `Resolve` 之前调用。
- `ConversationService.Detail` 不再返回渲染后为空的 assistant 消息，避免历史面板出现空白回复，也避免前端把空轮次回传。
- 回归测试：`TestNormalizeChatHistoryDropsInterruptedTurns`（正常历史不变、尾部空回复丢弃、中间空回复合并相邻用户消息）、`TestChatSendDropsInterruptedTurnsBeforeValidation`（handler 端到端：空 assistant 被丢弃、相邻用户消息合并后到达 Mastra）、`TestConversationDetailSkipsInterruptedAssistantTurns`（历史接口跳过空回答，来源关联不受影响）。
- 文档：`plain-chat.md` 记录历史归一化规则，`ai-architecture.md` 契约 1 的 `messages` 注释同步。

## 决策与理由

- **在 Go 归一化而不是在前端**：历史来源不止当前页面（旧线程、刷新后恢复、未来客户端），发送前的契约校验点只有一个，放在 Go 最可靠；前端只负责展示。
- **合并相邻用户消息而不是插入占位替身**：中断轮次没有真实回复，伪造一条「未回复」的 assistant 文本会污染模型的上下文语义；合并后的用户消息如实表达「这两次提问之间没有得到回答」。
- **历史接口直接过滤空 assistant**：这类消息没有可展示内容，返回空气泡只会让用户困惑；过滤后来源配对仍按 message id 进行，不受影响。

## 教训

- **多步 Agent 的 message 不是「一轮一条」**：任何按 `messages[len-1]` 或「逐条必非空」写的校验，都要考虑中间 step 与中断 step；校验应该在归一化之后执行。
- **「两轮后报错」这类现象先查落库数据**：直接读 Mastra 线程消息比猜测前端状态快得多，一次 SQL/接口查询就定位到空 `parts`。
- **中止路径也要有可回放的历史**：DSML 防护、用户停止、超时都可能留下无文本轮次，恢复历史的链路要有容错，而不是假设历史永远完整。

## 候选特性

- 在 Mastra 落库阶段就不持久化「无可见文本」的最终 assistant 消息（需确认 Memory 的写路径与删除策略）。
- 会话历史面板对中断轮次显示「未生成回复」标记，而不是静默跳过。

## 验证

- `go test -count=1 ./internal/handler ./internal/service ./internal/repository` 全绿。
- 未执行（需真机）：恢复含中断轮次的会话后继续对话；确认新消息正常到达 Mastra、历史面板无空白气泡。
