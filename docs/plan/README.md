---
last_reviewed: 2026-11-11
---

# 功能实现计划（MVP 差距 → 完整功能）

> 依据三份毕业设计文档与 2026-11-09 代码差距分析整理，列出仓库中**需要完成的功能**。
> 执行约定：本目录只维护"做什么/先后顺序/验收标准"；具体执行计划在 `.sisyphus/plans/` 细化，完成后在 `docs/tasks/` 写学习日志。

## 文档来源

| 文档 | 路径 | 作用 |
|---|---|---|
| 需求规格说明书 | [docs/design/需求规格说明书.md](../design/需求规格说明书.md) | 36 条功能需求（FR-*）、13 条验收（AC-*）、优先级 |
| 产品设计文档 | [docs/design/产品设计文档.md](../design/产品设计文档.md) | 9 个页面、9 张核心表、AI 工作流、开发计划 8 阶段、实验指标 |
| 架构设计方案 v2 | [docs/design/架构设计方案-v2.md](../design/架构设计方案-v2.md) | Go+Mastra 双服务职责划分、内部 API 契约、MVP 切法 |

## 需求完成度矩阵（2026-11-09 基线）

| 模块 | 已实现 | 部分实现 | 未实现 |
|---|---|---|---|
| 知识管理 FR-KM-01~09 | FR-KM-01 创建/编辑、FR-KM-02 文件导入、FR-KM-03 网页导入、FR-KM-04 列表筛选、FR-KM-05 详情、FR-KM-06 标签、FR-KM-07 收藏归档、FR-KM-08 分块预览、FR-KM-09 编辑历史 | — | — |
| AI 整理 FR-ORG-01~06 | — | — | **全部 6 条** |
| 检索问答 FR-QA-01~07 | FR-QA-01 关键词检索、FR-QA-02 语义检索、FR-QA-03 混合检索、FR-QA-04 双模式回答、FR-QA-05 引用溯源、FR-QA-06 未命中声明、FR-QA-07 会话历史 | — | — |
| 知识关联 FR-REL-01~03 | — | — | **全部 3 条** |
| 知识演化 FR-EVO-01~05 | — | — | **全部 5 条** |
| 系统支撑 FR-SYS-01~06 | FR-SYS-01 登录、FR-SYS-02 隔离、FR-SYS-04 任务、FR-SYS-05 重试 | FR-SYS-03（缺测试连接）、FR-SYS-06（缺统计展示） | — |

**验收对照**：AC-01/02/03/05/06/08/09 ✅；AC-04/07/10/11/12/13 未达成（P3/P4）。

## 阶段总览

阶段划分对齐《产品设计文档》第十章开发计划，并按依赖关系排序（标签→整理、版本→演化、关系→图谱）。

| 阶段 | 文件 | 目标 | 需求映射 | 验收映射 | 前置依赖 |
|---|---|---|---|---|---|
| P1 知识管理补全 | [phase-1-knowledge-crud.md](./phase-1-knowledge-crud.md) | 手动创建/编辑、版本快照、标签、网页导入、收藏归档、列表增强 | FR-KM-01/03/04/06/07/09、FR-EVO-04（表） | AC-02、AC-03 | 无 |
| P2 混合检索与问答 | [phase-2-hybrid-search-qa.md](./phase-2-hybrid-search-qa.md) | ngram 关键词路、Go 侧 RRF 混合、grounding 双模式回答、引用高亮 | FR-QA-01/03/04/06、FR-KM-08（收尾） | AC-05、AC-06、AC-09 | 无（可与 P1 并行） |
| P3 AI 整理与关联 | [phase-3-ai-organize-relation.md](./phase-3-ai-organize-relation.md) | 统一 ai_tasks、organize 工作流、摘要/关键词/标签确认、关联推荐与管理 | FR-ORG-01~06、FR-REL-01/02、FR-SYS-04（扩展） | AC-04、AC-07 | P1 标签表（W1.4） |
| P4 图谱与演化 | [phase-4-graph-evolution.md](./phase-4-graph-evolution.md) | ECharts 图谱、重复/过时/冲突检测、演化中心人工审核 | FR-REL-03、FR-EVO-01/02/03/05 | AC-10~13 | P1 版本表（W1.2）、P3 关系表（W3.4） |
| P5 收尾与实验 | [phase-5-polish-experiments.md](./phase-5-polish-experiments.md) | 测试连接、用量统计、首页聚合、三组检索实验、演化实验、性能压测 | FR-SYS-03/06 收尾、SRS 7.3 实验 | 全部 AC 回归、论文数据 | P2/P3/P4 |

**关键路径**：P1（标签/版本表）→ P3（整理/关联）→ P4（演化）→ P5（实验）。P2 独立于数据表依赖，可并行启动。

**进度**：P1 已完成（2026-11-10，7 个工作项）；P2 已完成（2026-11-11，5 个工作项）；P3 已完成（2026-11-11，W3.1~W3.5，AC-04 与 AC-07 闭环）；P4-W4.1 局部知识图谱已完成（2026-11-11，AC-10 待真机验收）；P4-W4.2 已完成（2026-11-12：GitHub #9 建表/content_hash 回填/evolve 任务/列表摘要裁决接口，#10 语义近重复与 `POST /evolve` 判定契约）；P4-W4.3 已完成（2026-11-12，GitHub #11：过时识别放行与弱信号双侧证据门槛、`mark_outdated`/`mark_verified` 副作用与操作日志、详情页过时横幅与「重新检查」）；P4-W4.4 已完成（2026-11-12，GitHub #12：`contradicts` 第六种关系类型与 AI 推荐白名单排除、冲突提醒接入 evolve 管线与修订稿门槛、`accept_revision` 走版本链路写 `change_source=ai` 新版本、`create_relation` 写 `confirmed`+`manual` 矛盾关系、图谱与图例渲染矛盾边）；P4-W4.5 已完成（2026-11-12，GitHub #13：`/home/evolution` 演化中心两页签、类型与状态筛选、并排证据高亮、按类型裁剪的裁决动作与备注、首页「待处理提醒」卡片只取 summary）；**P4 全部工作项代码完成，PR #16 待合并，AC-10~AC-13 待真机验收**；P5 未开始。
## 数据库变更总表

新增表（GORM AutoMigrate，全部带 `user_id` 或经知识条目归属隔离）：

| 表 | 用途 | 引入阶段 |
|---|---|---|
| `tags` | 用户标签（user_id+name 唯一，不区分大小写） | P1-W1.4 |
| `knowledge_tags` | 知识-标签关联（含 source: manual/ai） | P1-W1.4 |
| `knowledge_versions` | 编辑前自动快照（version_number/title/content/change_source） | P1-W1.2 |
| `retrieval_logs` | 检索日志（mode/hits JSON/hit_count/latency_ms，实验数据源） | P2-W2.6 |
| `ai_tasks` | 统一 AI 任务状态机（task_type: organize/relation/evolve；ingest 沿用现有导入任务表） | P3-W3.1 |
| `knowledge_relations` | 知识关系（source/target/type/confidence/evidence/status/source: ai 或 manual；唯一键含 relation_type，对称类型归一侧只存一条） | P3-W3.4 |
| `evolution_alerts` | 演化提醒（alert_type: duplicate/outdated/conflict、payload、status: pending/resolved/dismissed/deferred、severity: review/strong、subject_ids 有序数组、dedupe_key 唯一索引且终态不清空） | P4-W4.2 |

字段变更：

| 表 | 变更 | 引入阶段 |
|---|---|---|
| `knowledge_items` | +`source_type`(manual/markdown/pdf/web)、+`source_url`、+`summary`、+`keywords`(JSON)、+`is_favorite`、+`version`(乐观锁)、+`change_source`(manual/ai，当前正文来源) | P1（source_type/favorite/version）、P3（summary/keywords）、P4-W4.4（change_source） |
| `knowledge_chunks` | `content` 建 MySQL ngram 全文索引（`WITH PARSER ngram`） | P2-W2.1 |
| `vectorize_tasks` | +`stale_chunk_ids`(TEXT)、+`cleanup_collection`（编辑后旧向量按点清理） | P1-W1.2 |

已有基础（无需重建）：users、knowledge_items、knowledge_chunks（含向量状态）、模型配置三表、chat 会话（Mastra Memory）+ ChatTurnSource/ChatTurnUsage、导入/向量化任务表。

## 内部 API 契约待实现（对齐架构文档第 5 章）

已有：`POST /v1/chat`（SSE）、`POST /v1/retrieve`、`GET /internal/ai/config`、`GET /internal/files/:id`（AI 服务拉文件）、任务状态回调。

| 契约 | 方向 | 阶段 |
|---|---|---|
| ~~`GET /internal/files/keyword-search`~~ | 已取消（2026-11-10 变更：RRF 融合放 Go 侧，见 phase-2） | — |
| ✅ `POST {CHAT_AI_URL}/organize`（Go→AI 同步返回草稿） | 整理工作流 | P3-W3.2（2026-11-11 简化：取消 metadata 回调，见 phase-3） |
| `POST {CHAT_AI_URL}/relate`（Go→AI 同步返回关系建议） | 关联推荐 | P3-W3.4（2026-11-11 简化：取消 relations 回调，见 phase-3） |
| ✅ `POST {CHAT_AI_URL}/evolve`（Go→AI 同步返回判定数组） | 演化检测 | P4-W4.2（2026-11-12 简化：取消 AI→Go 回调，见 phase-4 方案调整记录） |
| ~~`POST /internal/ai/tasks/{taskId}/status` 扩展 task_type~~ | 已取消（2026-11-11：organize 改同步返回，任务状态只由 Go 侧 worker 维护） | — |

契约硬约束（架构文档）：**持久化回调必须先于任务状态回调**；AI 服务不直接对外、不鉴权 JWT（Go 侧 `X-Internal-Token` 已就绪）。

## 页面清单（产品设计文档第五章，9 个页面）

| 页面 | 状态 | 缺口 |
|---|---|---|
| 首页 Dashboard | ⚠️ 有基础统计 | 已含「待处理提醒」卡片（P4-W4.5）；仍缺最近知识、待处理任务聚合（P5-W5.2） |
| 知识列表 | ✅ 有 | 已支持来源类型/标签/收藏归档筛选与排序（P1-W1.6） |
| 知识详情 | ✅ 有 | 已支持编辑、版本历史、标签、AI 整理面板（P3-W3.3）与「相关知识」面板（P3-W3.4/W3.5） |
| 分块预览 | ✅ 有 | 已支持引用跳转与高亮定位（P2-W2.5） |
| 导入页面 | ✅ 有 | 已支持“网页链接”入口（P1-W1.5）；仍并入文件管理 |
| AI 问答 | ✅ 有 | grounding 双模式与引用高亮已就绪（P2） |
| 知识图谱 | ✅ 有 | `/home/knowledge/graph`：ECharts 局部图谱、筛选、建议边、点击抽屉（P4-W4.1） |
| 演化中心 | ✅ 有 | `/home/evolution`：两页签（待处理/处理记录）、类型与状态筛选、并排证据高亮、按类型裁剪的裁决与备注（P4-W4.5） |
| 系统设置 | ✅ 有 | 缺"测试连接"按钮（P5-W5.1） |

## 执行规则

1. **每个工作项（W-x.y）是一个可独立验收的 PR 粒度**；跨服务的工作项按"契约先行"：先定 JSON Schema，再并行实现 Go 侧与 Mastra 侧。
2. 每个 Phase 完成后跑该阶段验收（对应 AC），通过后才进入下一阶段的依赖工作项。
3. AI 服务不可用时的降级：语义检索退化为关键词检索（架构文档 7.2），P2 完成后即具备降级条件。
4. 实验数据收集（检索日志、确认/采纳记录）从 P2/P3 起随功能落地埋点，避免 P5 返工。
