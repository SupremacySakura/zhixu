---
last_reviewed: 2026-11-09
phase: P1
status: pending
---

# P1 知识管理补全

> 对应《产品设计文档》开发计划阶段 2/3 的收尾。目标是让知识条目脱离"只能由文件导入"的限制，补齐手动创建、编辑、版本、标签、网页导入、收藏归档。
> 验收映射：**AC-02**（知识创建编辑管理）、**AC-03**（三种来源导入）。

## 现状基线

- `POST /api/v1/knowledge` 强制要求 `file_id`（[knowledge.go](../../apps/zhixu-backend/internal/router/knowledge.go)），无编辑接口，前端正文只读（[frontend-structure.md](../frontend-structure.md)："正文仅阅读，不编辑"）。
- 分块/向量化管线已就绪（Markdown split + Qdrant + 部分重试），编辑后的"重分块"可直接复用。
- 无标签、无版本、无收藏归档字段；列表仅支持状态筛选 + 标题 LIKE 搜索。

## 工作项

### W1.1 知识手动创建（FR-KM-01）

- **后端**：`POST /api/v1/knowledge` 放开 `file_id` 必填，支持 `{title, markdown}` 手工创建：
  - `knowledge_items` 增加 `source_type`（manual/markdown/pdf/web）与 `source_url`（本工作项先落 manual；文件导入按文件 mime/扩展名回填 markdown/pdf）。
  - 创建后走与文件导入一致的"分块 → 向量化"管线（复用现有任务，不新建流程）。
  - 标题 ≤200 字符、正文非空校验；`content_hash` 对手工内容计算 SHA-256。
- **前端**：知识列表页"新建知识"按钮 → 编辑页（Markdown 编辑器 + 预览），保存后跳详情页。
- **依赖**：无。**验收**：手动创建的知识可分块、可向量化、可被检索命中。

### W1.2 知识编辑 + 版本快照（FR-KM-01、FR-EVO-04 表基础）

- **数据库**：新表 `knowledge_versions`（id、user_id、knowledge_id、version_number、title、content 快照、change_source: manual/ai、created_at）；`knowledge_items` 增加 `version` 乐观锁字段。
- **后端**：
  - `PUT /api/v1/knowledge/:id`（body 带 `base_version` 乐观锁，409 冲突）。
  - 每次保存**先把旧版本写入** `knowledge_versions`，再更新正文，随后异步重分块 + 内向量清理重跑（复用 `VectorCleanupTask`）。
  - 版本号单调递增；快照与正文同事务。
- **前端**：详情页进入"编辑模式"（Markdown 编辑器），保存时处理 409（提示刷新）。
- **依赖**：W1.1。**验收**：AC-02 编辑部分；编辑后旧版本可查询，分块/向量与新版正文一致。

### W1.3 编辑历史（FR-KM-09）

- **后端**：`GET /api/v1/knowledge/:id/versions`（版本列表）、`GET /api/v1/knowledge/:id/versions/:n`（快照详情）、`POST /api/v1/knowledge/:id/versions/:n/restore`（恢复 = 以快照内容创建一个**新版本**，change_source=manual，不篡改历史）。
- **前端**：详情页"历史"抽屉：版本列表（时间/来源）、快照只读预览、恢复按钮；相邻版本 diff 为可选项（低优先级）。
- **依赖**：W1.2。**验收**：FR-KM-09 五条规则全过（列表/查看/恢复/恢复不删历史/时间倒序）。

### W1.4 标签（FR-KM-06）

- **数据库**：`tags`（id、user_id、name；同用户不区分大小写唯一）；`knowledge_tags`（knowledge_id、tag_id、source: manual/ai，联合唯一）。
- **后端**：`GET/POST /api/v1/tags`、`PUT/DELETE /api/v1/tags/:id`；`PUT /api/v1/knowledge/:id/tags`（批量绑定，source=manual）；删除标签级联解绑；标签被删不影响知识本体。
- **前端**：详情页标签编辑器（输入联想现有标签）；列表页标签筛选（与 W1.6 合并落地）。
- **依赖**：无；**是 P3 FR-ORG-04（AI 标签推荐）的前置**。**验收**：标签 CRUD、绑定解绑、同库标签唯一性、筛选可用。

### W1.5 网页导入（FR-KM-03）

- **后端**：`POST /api/v1/knowledge/import-web {url}`：
  - Go 侧抓取（超时 30s、仅 http/https、SSRF 防护：禁内网地址段）→ 正文提取（goquery/readability 类库）→ 转 Markdown → `source_type=web`、`source_url` 落库 → 复用分块向量化管线。
  - 失败入 `failure_reason` 可重试（复用现有重试接口）。
- **前端**：导入入口增加"网页链接"Tab（文件管理导入对话框或独立导入页），输入 URL → 创建后跳详情页看进度。
- **依赖**：W1.1（source_type 字段）。**验收**：AC-03 网页部分——公开文章 URL 可导入、标题/正文正确、可检索。

### W1.6 列表增强（FR-KM-04）

- **后端**：`GET /api/v1/knowledge` 增加参数：`source_type`、`tag_id`、`sort`（updated_at/created_at/title）、`favorite`、`archived`；默认排除已归档。
- **前端**：列表页筛选栏（来源类型/标签/收藏/归档）+ 排序下拉。
- **依赖**：W1.4（tag 筛选）、W1.5（source_type 值集）、W1.7（favorite/archived）。**验收**：FR-KM-04 各筛选与排序组合生效。

### W1.7 收藏与归档（FR-KM-07）

- **数据库**：`knowledge_items` 增加 `is_favorite`（bool）。
- **后端**：`POST /api/v1/knowledge/:id/favorite`（幂等切换）、`POST /api/v1/knowledge/:id/archive`（active↔archived，可恢复）。
  - **归档语义**：归档知识**不参与检索**（RAG payload 过滤排除 archived，问答与关联推荐均不可见），恢复后重新生效——需在向量化重建或检索过滤二选一，推荐检索侧按状态过滤（轻量）。
- **前端**：列表行操作（收藏/归档/恢复）、归档默认隐藏。
- **依赖**：无。**验收**：收藏/取消、归档/恢复、归档后检索不可见。

## 里程碑验收（P1 Done）

1. AC-02 全过：手动创建 → 编辑 → 版本恢复 → 删除，全程状态可见。
2. AC-03 全过：手动 + 文件 + 网页三种来源均可导入并检索。
3. 标签可手工管理，为 P3 的 AI 标签推荐提供落库结构。
4. `knowledge_versions` 表就绪，为 P4 演化"接受修订"提供版本写入路径。
