---
task_id: docs/plan/phase-3-ai-organize-relation.md（P3-W3.1/W3.2/W3.3）
status: completed
created: 2026-11-11
last_reviewed: 2026-11-11
applicable_to: 在 Go+Gin+GORM 后端与 Mastra AI 服务之间落地「LLM 结构化产出 + 人工确认」类异步任务
---

# AI 整理工作流（ai_tasks + organize + 预览确认）

## 结果

- 后端新增 `ai_tasks` 表与 `AiTaskService`：2s 轮询、`FOR UPDATE SKIP LOCKED` 领取、3 分钟租约回收、失败退避 30/60/120s 后转 failed；`dedupe_key` 唯一索引只覆盖 pending/running，重复触发复用进行中任务。
- AI 服务新增 `POST /organize`：Bearer `INTERNAL_API_SECRET`、90s 超时、`agent.generate(..., {structuredOutput})` 产出 `{summary, keywords, suggestedTags, concepts}`，zod 校验失败重试 1 次后返回 422，上游异常返回 502。
- Go 侧 `OrganizeProcessor` 解析用户默认对话模型、正文截断 12000 字符、附带用户已有标签 50 个，把草稿写入 `ai_tasks.payload`（含 `base_version`）。
- 新增 `GET/POST /api/v1/knowledge/:id/organize[/accept|/regenerate]`：草稿视图带 `suggested_tags[{name,tag_id,existing}]`、`accepted`、`stale`；采纳事务内只写 `summary/keywords` 并追加标签（`source=ai`），不动正文、版本与向量状态。
- 前端新增 `KnowledgeOrganizePanel`（逐项编辑、推荐标签切换、采纳/重新生成、任务进行中 3s 轮询），详情页右栏接入，知识列表卡片展示已采纳摘要。
- 验证：Go `vet`+全量测试绿；AI 39 项测试绿；前端 260 项测试、lint、typecheck、build 绿。

## 教训

- Mastra 1.78 **没有** `generateObject`：结构化输出必须走 `agent.generate(messages, { structuredOutput: { schema } })` 并读 `result.object`。
- 给已有表追加 NOT NULL 的 JSON 列，MySQL 会用空串回填历史行并报 `ERROR 3140 Invalid JSON text`；用 varchar 存 JSON 字符串、服务层编解码更安全。
- 标签关联不能复用「先删后插」的 `ReplaceForKnowledge`，否则 AI 采纳会抹掉用户手工标签；需要 `ON CONFLICT DO NOTHING` 的只增接口。
- 判断草稿是否过期不能用时间戳（采纳会刷新 `updated_at`），要用正文 `version` 与草稿记录 `base_version` 比对。
- Go 单测用 sqlmock 时，SQL 里的反引号在 Go 双引号字符串中无法转义，正则改用 `.` 匹配；GORM 默认事务要显式 `ExpectBegin/ExpectCommit`。
- 本机 `go` 编译缓存目录被沙箱拒绝，所有命令需 `GOCACHE=/tmp/zhixu-go-cache GOTMPDIR=/tmp`。
- run_code 程序里拼接含反引号的 TS 源码会被模板字面量截断，用 `String.fromCharCode(96)` 或按行数组 join。
- 根目录 `pnpm lint` 会卡住数分钟：ESLint 9 不读 `.gitignore`，把 `apps/zhixu-ai/.mastra/output`（349 个打包文件 / 26MB）也当作源码格式化。在 `eslint.config.js` 的 ignores 里补 `**/.mastra/**` 后 lint 从「>10 分钟」降到 5 秒。

## 可沉淀模式

- **单向同步契约替代回调**：AI 侧只做无副作用生成并同步返回，Go 侧落草稿，用户确认后再写正式字段。省掉 AI→Go 回调的鉴权、重放与乱序补偿。
- **活跃态唯一幂等键**：`dedupe_key` 唯一索引 + 终态置 NULL，实现「同用户同类型同目标只允许一个在跑任务」，又保留历史记录。
- **草稿视图层**：后端把 payload 解成带 `accepted/stale/existing` 标记的视图，前端无需理解内部存储格式。
- **采纳即事务**：校验（长度/数量/归属）→ 写正式字段 → 追加关联 → 记录采纳时间，失败整体回滚，保证正式字段不被半途污染。
