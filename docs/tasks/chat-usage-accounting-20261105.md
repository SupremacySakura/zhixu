---
task_id: chat-usage-accounting-20261105
status: completed
created: 2026-11-05
last_reviewed: 2026-11-05
applicable_to: "对话、token 用量、Mastra 事件流、Go 记账、配额与账单"
---

# P3：对话 token 用量记账（`done.usage`）

## 背景

架构改造的最后一阶段（`docs/ai-architecture.md` 的 P3）要求把每轮对话的模型 token 用量落库，作为后续配额拦截与账单的「落库基础」。此前 Go 与 Mastra 都没有任何 usage 结构：Mastra 只把 `message_id` 放进 `done` 事件，供应商的 usage 在流结束后被丢弃。

## 结果

- **Mastra**（`apps/zhixu-ai/src/mastra/chat.ts`）：从 `step-finish` 的 `totalUsage`（跨 step 累计）与终止 `finish` 的 `output.usage` 收集用量，归一化为 `{input_tokens, output_tokens, total_tokens}`，追加到 `done` 事件；供应商未上报时整字段省略，事件形状保持稳定。
- **Go**：`ChatTurnUsage` 模型 + `chat_turn_usage` 表（AutoMigrate）；`ChatSourceRepository.SaveTurnUsage` 复用会话行锁与墓碑校验，删除会话时在 `MarkDeleted`/`CompleteCleanup` 同一事务中删除用量行；`ChatHandler.Send` 在 `done` 时记账（5s 超时、失败只记日志）。
- **前端**：`ChatEvent` 的 `done` 增加可选 `usage`，仅类型层，界面暂不展示（属扩展点）。

## 决策与理由

- **用量留在 Go 而不是 Mastra 的 LibSQL/DuckDB**：一张表一个写入方；Go 是用户、模型配置与未来配额的所有者，记账必须与用户维度同库，才能在删除会话时与溯源一起事务清理。
- **记账失败不影响回复**：此时供应商调用已产生费用、文本已生成，把流的完成状态改写成错误只会让用户重试并再付一次费用；记日志 + 保留回复是最小损失方案。
- **归一化字段而不是透传供应商原始 usage**：不同供应商的 usage 字段（reasoning/cached tokens 等）不一致，统一成三个整数能保证表结构与前端契约稳定；需要细分时再扩列。
- **以 `finish` 的累计值为准**：多步工具调用每步都可能上报 usage，终止事件代表整轮；`step-finish` 仅作为终止事件缺失时的兜底。

## 教训

- **流式 usage 来自终止事件**：`text-delta` 循环里顺手统计是行不通的，必须显式处理 `finish`/`step-finish`；同时要容错「供应商完全不上报」。
- **记账路径与非关键路径要分开设计**：溯源保存失败会发送错误事件（用户需要知道引用缺失），用量失败静默记录即可，两者的严重程度不同。
- **删除会话的清理必须在同一事务覆盖所有派生的用户数据**，否则墓碑之后仍有晚到的用量行写入。

## 候选特性

- 用量配额拦截（按用户/模型/周期）与超限提示。
- 对话界面展示本轮/累计 token 与估算费用。
- 按模型维度聚合的用量统计 API 与运营视图。

## 验证

- Go：`go build ./...`、`go vet ./...`、`go test -count=1 ./internal/handler ./internal/service ./internal/repository` 全绿（新增 handler 记账成功/失败两例、repository 落库/墓碑拒绝/缺 message id 三例、service 空值与墓碑两例；会话删除用例补用量删除断言）。
- Mastra：`pnpm test` 19/19（`done` 携带 usage、供应商未上报时省略），`pnpm typecheck` 通过。
- 前端：typecheck/lint/vitest 全绿。
- 未执行（需真机）：重启 Go 后端迁移 `chat_turn_usage`，对话一轮后确认表中有对应行；删除会话后确认行被清除。
