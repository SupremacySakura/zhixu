---
last_reviewed: 2026-11-11
phase: P3
status: in-progress
---

# P3 AI 智能整理与关联推荐

> 对应《产品设计文档》开发计划阶段 5。建立统一 AI 任务状态机，落地 organize 工作流（摘要/关键词/标签）与 relation 工作流（关联推荐）。
> 验收映射：**AC-04**（AI 摘要与标签）、**AC-07**（相关知识推荐）。
> 前置依赖：**P1-W1.4**（标签表）。

## 方案调整记录（2026-11-11，用户确认）

实施 W3.1~W3.3 时对原规格做了以下调整，用户已确认「规格原文不等于真理，按推荐方案走」：

- **契约简化为 Go→AI 单向同步调用**：Go 的 `ai_tasks` worker 直接 `POST {CHAT_AI_URL}/organize` 并同步取回草稿，不实现 `POST /internal/ai/tasks/{taskId}/status` 与 `POST /internal/ai/files/{fileId}/metadata` 两条回调。理由：organize 是无副作用的「读正文→生成」纯计算，同步返回可省掉 AI→Go 的鉴权、重放与乱序补偿；持久化仍在 Go 侧一次性写入，不存在「回调先于状态」的硬约束风险。代价是 AI 调用期间占用 worker 一个循环（租约 3 分钟、AI 超时 100 秒，已在阈值内）。
- **草稿落 `ai_tasks.payload`**：payload 存 `{summary, keywords, suggested_tags, concepts, base_version}`，`knowledge_items.summary/keywords` 只在用户采纳（accept）时写入。
- **关键词用 varchar(1000) 存 JSON 数组**，不用 `type:json` 列：给已有 20 万行级别的 `knowledge_items` 追加 NOT NULL JSON 列会被 MySQL 用空串回填历史行并报 `ERROR 3140 Invalid JSON text`。读写经 `EncodeKeywords/ParseKeywords`，对上层暴露 `keywords: string[]`。
- **AI 侧用 `agent.generate(messages, { structuredOutput })`**：Mastra 1.78 没有 `generateObject`；zod 校验失败自动重试 1 次，仍失败返回 422 `schema_invalid`（Go 侧判定为不可重试）。供应商不支持原生 json_schema 时由 `jsonPromptInjection: 'auto'` 兜底。
- **标签采纳只增不删**：新增 `TagRepository.AttachForKnowledge`（`ON CONFLICT DO NOTHING`，source=ai），不复用会先删后插的 `ReplaceForKnowledge`——否则用户手工标签会被 AI 推荐抹掉。
- **过期判断用版本号而非时间戳**：草稿记录 `base_version`，`knowledge_items.version` 变化即标记 stale（采纳本身会刷新 updated_at，时间戳不可靠）。编辑正文不自动重跑整理，只在面板提示「建议重新生成后再采纳」。
## 现状基线

- AI 服务仅有 1 个 chat agent 与 retrieve/ingest 能力；无 generateObject 结构化输出工作流。
- `knowledge_items` 无 summary/keywords 字段；无 `knowledge_relations` 表；无统一 `ai_tasks` 表（现有 KnowledgeImportTask/VectorizeTask 覆盖导入与向量化，可保留专用，新任务类型入 ai_tasks）。
- 模型配置下发链路已就绪（`GET /internal/ai/config` + 用户默认模型）。

## 工作项

### W3.1 统一 ai_tasks 状态机（FR-SYS-04 扩展）

- **数据库**：`ai_tasks`（id、user_id、task_type: organize/relation/evolve、target_id（knowledge_id）、status: pending/running/succeeded/failed、payload JSON、error_message、retry_count、created_at/updated_at；`(task_type,target_id,status)` 部分唯一防并发重复入队，或应用层幂等键）。
- **Go**：任务创建（幂等：同 target 同类型进行中任务复用）、租约/超时回收（复用现有模式）、失败重试（指数退避，≤3 次）、状态回调接收。
- **回调扩展**：`POST /internal/ai/tasks/{taskId}/status` 支持新 task_type；**硬约束：持久化回调（metadata/relations/alerts）必须先于任务状态回调**（架构契约）。
- **依赖**：无。**验收**：任务列表可见（复用任务面板），重复触发不产生重复任务，失败可重试。

### W3.2 Organize 工作流：摘要/关键词/标签（FR-ORG-01/02/03/04）

- **契约**：Go→AI `POST /v1/organize {taskId, knowledgeId, userId}`；AI→Go `POST /internal/ai/files/{fileId}/metadata {taskId, summary, keywords[], suggestedTags[]}`。
- **AI 服务**：`organize-workflow`——读取知识正文（Go 内部接口或回调 payload 携带）→ `generateObject` + zod Schema（summary ≤200 字、keywords ≤10 个、suggestedTags 从用户现有标签+新词混合）→ Schema 校验失败自动重试 1 次 → 结构化结果回调。
- **Go**：`knowledge_items` 增加 `summary`（text）、`keywords`（JSON）；suggestedTags 经 `knowledge_tags.source=ai` 落库（**待确认状态，见 W3.3**）。
- **触发**：导入/创建完成后自动触发（产品设计 8.1：清洗校验后摘要关键词与分块向量化**并行**）；详情页可手动"重新整理"。
- **依赖**：W3.1、P1-W1.4。**验收**：新导入知识 N 秒内出现摘要/关键词；标签推荐包含用户已有标签匹配。

### W3.3 整理结果预览与确认（FR-ORG-05）

- **Go**：整理结果先落"待确认"区（`ai_tasks.payload` 或 `knowledge_items` 增加 `organize_status`/暂存字段）；`POST /api/v1/knowledge/:id/organize/accept {summary?, keywords?, tagIds[]}`（用户可编辑后采纳，一次落正式字段）、`POST .../regenerate`。
- **前端**：详情页"AI 整理"面板：摘要/关键词/推荐标签并排预览，支持逐项编辑、一键采纳、重新生成；采纳前列表页摘要不展示（或展示但标注 AI 待确认）。
- **依赖**：W3.2。**验收**：AC-04——生成 → 预览 → 编辑 → 采纳闭环；未采纳不污染正式字段。

### W3.4 关联推荐（FR-REL-01 + FR-ORG-06）

- **数据库**：`knowledge_relations`（id、user_id、source_id、target_id、relation_type: related/extends/supports/contradicts/updates、confidence 0~1、evidence JSON、status: suggested/confirmed/rejected、created_at；`(source_id,target_id)` 唯一防重复，反向关系视为同一对）。
- **契约**：AI→Go `POST /internal/ai/files/{fileId}/relations {taskId, relations:[{targetKnowledgeId, relationType, confidence, evidence}]}`。
- **AI 服务**：`relation-workflow`——以新知识向量在 Qdrant 找近邻候选 Top10（同 user、排除自身与 archived）→ LLM 逐对判定关系类型/置信度/证据（generateObject）→ 置信度 <0.6 丢弃 → 回调。
- **触发**：organize 完成后自动触发（产品设计 8.1 的"检索相关知识→关联建议"段）；手动"查找相关知识"按钮同样可用。
- **前端**：详情页"相关知识"区：列表展示（标题/关系类型/置信度），建议态关系带"确认/忽略"操作。
- **依赖**：W3.2（挂在 organize 后）。**验收**：AC-07——导入相似主题文档后出现相关知识推荐，点击可跳转。

### W3.5 关系管理（FR-REL-02）

- **后端**：
  - `GET /api/v1/knowledge/:id/relations`（含 suggested/confirmed，标注方向与类型）。
  - `POST /api/v1/knowledge/:id/relations {targetId, relationType}`（手动添加，直接 confirmed）。
  - `PUT /api/v1/relations/:id {relationType}`（改类型）、`DELETE /api/v1/relations/:id`。
  - `POST /api/v1/relations/:id/confirm | /reject`（确认/拒绝 AI 建议；reject 后不再对同一对重复建议——记录 rejected 状态去重）。
- **前端**：详情页关系管理面板（分"已确认/待确认"两区）。
- **依赖**：W3.4。**验收**：FR-REL-02 全规则——增删改查、类型修改、确认/拒绝、重复建议抑制。

## 里程碑验收（P3 Done）

1. AC-04：新知识自动产生摘要/关键词/标签推荐，确认机制可用。
2. AC-07：相关知识自动推荐且可确认，手动关系管理闭环。
3. `ai_tasks` 统一状态机承载 organize/relation 两类任务，P4 的 evolve 直接复用。
4. `knowledge_relations` 数据积累，为 P4 图谱（已确认关系渲染）备料。
## 完成情况（2026-11-11）

| 工作项 | 主要改动 | 验收 |
|---|---|---|
| W3.1 ai_tasks 状态机 | `ai_tasks` 表（dedupe_key 唯一索引做 `(type,target)` 幂等、仅 pending/running 非空）；`AiTaskService` 2s 轮询、`FOR UPDATE SKIP LOCKED` 领取、3 分钟租约回收、失败退避 30/60/120s、≤3 次转 failed；创建/导入完成后自动入队 organize；`GET /api/v1/knowledge/:id/organize` 返回任务视图 | 重复触发不产生重复任务；进程重启后过期租约被回收 |
| W3.2 organize 工作流 | AI 侧 `POST /organize`（Bearer INTERNAL_API_SECRET、90s 超时、422 契约错误 / 502 上游错误分离）；`OrganizeAgent` 结构化输出摘要/关键词/推荐标签/概念；Go 侧 `OrganizeProcessor` 解析用户默认对话模型、正文截断 12000 字符、带用户已有标签 50 个作提示；草稿写 `ai_tasks.payload` | 新知识十几秒内出现草稿；`knowledge_items.summary` 在采纳前保持为空；AI 未配置时任务 failed 且有中文原因 |
| W3.3 预览与确认 | `GET /organize` 返回 `draft`（含 `suggested_tags[{name,tag_id,existing}]`、`accepted`、`stale`）；`POST /organize/accept` 校验摘要 ≤200 字、关键词 0~10 个、事务内写正式字段 + 追加标签 + 记录采纳时间；`POST /organize/regenerate` 重新入队；前端 `KnowledgeOrganizePanel`（逐项编辑、推荐标签切换、采纳/重新生成、3s 轮询）与列表卡片摘要 | AC-04 生成→预览→编辑→采纳闭环；采纳不新增版本、不改正文与向量状态；手工标签不被覆盖 |

**验证**：`go vet ./... && go test ./...` 全绿（repository/service/handler/router）；`pnpm --filter zhixu-ai test` 39 项通过（含 organize 契约与清洗 6 项）；前端 `pnpm lint`、`pnpm typecheck`、`pnpm test`（260 项，含新增面板 5 项）与 `pnpm build` 通过。端到端（真实模型生成→采纳）需重启后端与 Mastra 后按 W3.3 验收路径手测。

**尚未完成（第四批）**：W3.4 关联推荐、W3.5 关系管理（`knowledge_relations` 表与相关接口），P3 因此仍为 in-progress；AC-07 待第四批验收。

