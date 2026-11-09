---
last_reviewed: 2026-11-09
phase: P3
status: pending
---

# P3 AI 智能整理与关联推荐

> 对应《产品设计文档》开发计划阶段 5。建立统一 AI 任务状态机，落地 organize 工作流（摘要/关键词/标签）与 relation 工作流（关联推荐）。
> 验收映射：**AC-04**（AI 摘要与标签）、**AC-07**（相关知识推荐）。
> 前置依赖：**P1-W1.4**（标签表）。

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
