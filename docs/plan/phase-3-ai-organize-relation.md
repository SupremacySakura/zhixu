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

### W3.1 统一 ai_tasks 状态机（FR-SYS-04 扩展）✅ 已完成（2026-11-11）

- **数据库**：`ai_tasks`（id、user_id、task_type: organize/relation/evolve、target_id（knowledge_id）、status: pending/running/succeeded/failed、payload JSON、error_message、retry_count、created_at/updated_at；`(task_type,target_id,status)` 部分唯一防并发重复入队，或应用层幂等键）。
- **Go**：任务创建（幂等：同 target 同类型进行中任务复用）、租约/超时回收（复用现有模式）、失败重试（指数退避，≤3 次）、状态回调接收。
- **回调扩展**：`POST /internal/ai/tasks/{taskId}/status` 支持新 task_type；**硬约束：持久化回调（metadata/relations/alerts）必须先于任务状态回调**（架构契约）。
- **依赖**：无。**验收**：任务列表可见（复用任务面板），重复触发不产生重复任务，失败可重试。

### W3.2 Organize 工作流：摘要/关键词/标签（FR-ORG-01/02/03/04）✅ 已完成（2026-11-11）

- **契约**：Go→AI `POST /v1/organize {taskId, knowledgeId, userId}`；AI→Go `POST /internal/ai/files/{fileId}/metadata {taskId, summary, keywords[], suggestedTags[]}`。
- **AI 服务**：`OrganizeAgent`——读取知识正文（由 Go 侧随请求携带）→ `agent.generate(..., { structuredOutput })` + zod Schema（summary ≤200 字、keywords ≤10 个且每项 ≤40 字符、suggestedTags 从用户现有标签+新词混合）→ Schema 校验失败自动重试 1 次 → 同步返回结构化结果。
  - **长度容错**：Mastra 在返回前就按 schema 校验模型输出，因此模型侧校验用宽松 schema（summary ≤400、keywords 每项 ≤40），再在 `normalizeDraft` 里把超长 summary 按 `。！？；.!?;` 语义边界截到 200 字；仍不合法时返回 422（输出不可用），而不是 502（上游故障），避免 Go 侧无意义重试。
- **Go**：`knowledge_items` 增加 `summary`（text）、`keywords`（JSON）；suggestedTags 经 `knowledge_tags.source=ai` 落库（**待确认状态，见 W3.3**）。
- **触发**：导入/创建完成后自动触发（产品设计 8.1：清洗校验后摘要关键词与分块向量化**并行**）；详情页可手动"重新整理"。
- **依赖**：W3.1、P1-W1.4。**验收**：新导入知识 N 秒内出现摘要/关键词；标签推荐包含用户已有标签匹配。

### W3.3 整理结果预览与确认（FR-ORG-05）✅ 已完成（2026-11-11）

- **Go**：整理结果先落"待确认"区（`ai_tasks.payload` 或 `knowledge_items` 增加 `organize_status`/暂存字段）；`POST /api/v1/knowledge/:id/organize/accept {summary?, keywords?, tagIds[]}`（用户可编辑后采纳，一次落正式字段）、`POST .../regenerate`。
- **前端**：详情页"AI 整理"面板：摘要/关键词/推荐标签并排预览，支持逐项编辑、一键采纳、重新生成；采纳前列表页摘要不展示（或展示但标注 AI 待确认）。
- **依赖**：W3.2。**验收**：AC-04——生成 → 预览 → 编辑 → 采纳闭环；未采纳不污染正式字段。

### W3.4 关联推荐（FR-REL-01 + FR-ORG-06）✅ 已完成（2026-11-11，第四批）

- **数据库**：`knowledge_relations`（id、user_id、source_id、target_id、relation_type、confidence、evidence JSON、status、source: ai/manual、created_at/updated_at；唯一键 `(user_id, source_id, target_id, relation_type)`）。
- **关系类型（5 种，2026-11-11 定）**：related / depends_on / supplements / contrasts / updates（界面显示相关、依赖、补充说明、对比、更新替代）。合并了原计划的 extends/supports/contradicts，只补 P4-W4.3 需要的 updates；理由与取舍见 [ADR-0001](../adr/0001-knowledge-relation-model.md)。
- **方向与判重**：有向存储；related、contrasts 两种对称类型写入前按 min/max id 归一，A→B 已存在则不再插 B→A；其余三种保持严格方向。
- **契约（2026-11-11 简化，取消 AI→Go 回调）**：Go→AI `POST {CHAT_AI_URL}/relate`（Bearer INTERNAL_API_SECRET，同步返回）：请求 `{version:1, model{protocol,model_id,base_url,api_key}, taskId, knowledgeId, knowledge{title, excerpt}, candidates:[{knowledge_id, title, excerpt, score}]}`；响应 `{relations:[{target_id, relation_type, confidence, evidence{reason, source_quote, target_quote}}]}`。
- **候选生成（Go 侧）**：用「标题 + 已采纳摘要（无摘要取正文前 1000 字）」嵌入一次向量 → Qdrant Search top 50 → 按 `knowledge_id` 归并取最高分 → 过滤自身/归档/已有关系对/已拒绝对 → Top 10 送模型；相似度 < 0.55 直接丢弃。
- **AI 服务**：Mastra `POST /relate`（严格 zod 校验 + `agent.generate(..., {structuredOutput})`，单条非法丢弃而非整批失败）；最多 5 条建议，置信度 < 0.6 由 Go 侧再过滤；Schema 失败内部重试 1 次后返回 422。
- **触发**：向量化就绪（`vector_state=ready`）后自动入队 `relation` 任务，`dedupe_key=relation:<knowledgeID>`；不向对端扩散（避免一次导入触发雪崩）；详情页「查找相关知识」可手动重跑，新建议与既有建议合并。
- **降级与失败**：候选 < 2 条、422、知识已归档/删除均按终端失败处理不重试；Qdrant/AI 超时与 5xx 走 30/60/120s 退避共 3 次。
- **前端**：详情页右栏「相关知识」面板（已确认列表 + 待确认建议 + 查找/手动添加 + 图谱入口），任务运行中 3s 轮询。
- **依赖**：W3.2（复用 `ai_tasks` 与 organize 的同步契约路线）。**验收**：AC-07——导入相似主题文档后出现相关知识推荐，点击可跳转，确认/拒绝闭环。

### W3.5 关系管理（FR-REL-02）✅ 已完成（2026-11-11，第四批）

- **后端**：
  - `GET /api/v1/knowledge/:id/relations`（confirmed / suggested 两区 + relation 任务状态）。
  - `POST /api/v1/knowledge/:id/relations {target_id, relation_type}`（手动添加，直接 confirmed、source=manual）。
  - `PUT /api/v1/relations/:id {relation_type}`（改类型）、`DELETE /api/v1/relations/:id`（物理删除，前端二次确认）。
  - `POST /api/v1/relations/:id/confirm | /reject`（reject 保留 rejected 行做抑制，同一对不再推荐；用户手动添加同一对时复用 rejected 行改为 confirmed + source=manual，不插新行）。
  - 删除知识条目时在同一事务内删除「source_id 或 target_id 命中」的关系行；归档只影响可见性不删关系。
- **前端**：详情页「相关知识」面板分「已确认 / 待确认」两区，支持改类型、删除、搜索手动添加。
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
| W3.4 关联推荐 | `knowledge_relations` 表（5 种类型、唯一键含类型、对称类型按 min/max 归一、source: ai/manual）；relation 任务（向量化 ready 后入队、代表文本嵌入召回 top50 → 按知识归并 → Top10 送 AI、`POST /relate` 结构化判定、置信度 <0.6 与证据过滤）；详情页「相关知识」面板与「查找相关知识」 | AC-07 导入相似主题出现建议、确认/拒绝闭环；同对可多类型、rejected 永久抑制 |
| W3.5 关系管理 | GET/POST/PUT/DELETE 与 confirm/reject/regenerate；rejected 行保留抑制、手动添加复用为 confirmed+manual；有向存储与对端标题归一；删除知识同事务级联删关系 | FR-REL-02 增删改查、改类型、确认/拒绝、重复建议抑制 |

**验证**：`go vet ./... && go test ./...` 全绿（repository/service/handler/router）；`pnpm --filter zhixu-ai test` 39 项通过（含 organize 契约与清洗 6 项）；前端 `pnpm lint`、`pnpm typecheck`、`pnpm test`（260 项，含新增面板 5 项）与 `pnpm build` 通过。端到端（真实模型生成→采纳）需重启后端与 Mastra 后按 W3.3 验收路径手测。

**P3 状态**：W3.4/W3.5 已于 2026-11-11 随第四批完成（GitHub #2–#4），P3 全部工作项完成；AC-07 代码、单测与图谱联动闭环，真机端到端待用户验收。AC-10 属 P4-W4.1，见 [phase-4](./phase-4-graph-evolution.md)。

