---
last_reviewed: 2026-11-09
phase: P4
status: pending
---

# P4 知识图谱与知识演化

> 对应《产品设计文档》开发计划阶段 6，毕设"重点特色"主战场（论文创新点 9.2/9.3 的实验对象）。
> 验收映射：**AC-10**（图谱）、**AC-11**（重复检测）、**AC-12**（版本管理）、**AC-13**（冲突/过时提示与人工审核）。
> 前置依赖：**P1-W1.2**（knowledge_versions）、**P3-W3.4**（knowledge_relations）、**P3-W3.1**（ai_tasks）。

## 现状基线

- 版本表与关系表在 P1/P3 落地；演化相关表、工作流、页面全部空白。
- 云盘秒传的 hash 精确去重已存在（架构文档附录映射：FR-EVO-01 的"精确重复"部分复用它，本阶段补**语义近重复**）。
- 进度紧张时的裁剪预案（产品设计第十章）：演化模块优先 FR-EVO-01 重复检测 + FR-EVO-04 版本管理，冲突/过时识别可降为规则简化版。

## 工作项

### W4.1 知识图谱可视化（FR-REL-03）⏳ 未开始（第四批，设计已定 2026-11-11）

- **后端**：`GET /api/v1/knowledge/graph?center_id=&depth=1&include_suggested=0&tag_id=&source_type=`——节点=知识条目（标题/来源类型/摘要、度数），边=关系（类型/置信度/来源）；默认只画已确认关系，`include_suggested=1` 时叠加建议关系；默认排除归档知识；`depth` 固定 1（局部视图）；节点上限 100，超过按连接度截断并在响应标 `truncated`；无 `center_id` 时取最近更新的 50 条知识及其间的已确认关系。
- **筛选（FR-REL-03 硬要求）**：`tag_id` 与 `source_type` 同时作用于节点与边（只保留两端都被筛中的边）；前端工具栏两个下拉，标签列表复用 `GET /api/v1/tags`。
- **前端**：新页面 `/home/knowledge/graph`（ECharts 按需引入 core + GraphChart + CanvasRenderer），侧边栏「知识图谱」菜单（`HomePage.vue` 的 `NavItems`）：
  - 局部分力导向布局；节点大小按连接度、颜色按来源类型；边样式区分 ai/manual，标签显示关系类型。
  - 交互：点击节点打开右侧抽屉（标题/摘要/一跳关系，可跳详情或「以此为中心」重绘）、顶部中心知识搜索框、显示建议边开关、图例。
  - 空状态引导「先去知识详情页建立关系」。
- **依赖**：P3-W3.4/W3.5。**验收**：AC-10——图谱展示节点与关系，点击进入详情，局部渲染流畅（百节点级）。

### W4.2 重复检测（FR-EVO-01）

- **数据库**：`evolution_alerts`（id、user_id、alert_type: duplicate/outdated/conflict、subject_ids JSON（涉及的知识 id 列表）、payload JSON（LLM 结构化判定：type/reason/evidence[{knowledgeId,quote}]/suggestedAction）、status: pending/resolved/dismissed、created_at/resolved_at）。
- **检测逻辑**（Go 侧编排，导入完成钩子触发）：
  1. **精确重复**：复用 `content_hash`/秒传 hash——hash 相同直接产生 duplicate 提醒（无需 LLM）。
  2. **语义近重复**：新知识向量在 Qdrant 找 Top5 近邻，相似度 >0.9（阈值入配置）→ 入 candidate → 走 `POST /v1/evolve/check` 由 LLM 复核（防误报）→ 回调 `POST /internal/ai/files/{fileId}/evolution-alerts`。
- **依赖**：W3.1（ai_tasks，task_type=evolve）。**验收**：AC-11——导入与已有知识语义近重复的文档，演化中心出现提醒。

### W4.3 过时识别（FR-EVO-02）

- **触发**：新版本知识入库（P1 编辑产生新 version）或导入与已有知识高相关的新知识时，对"被 updates 关系指向的旧知识"做判定。
- **AI 服务**：`evolve-workflow`——候选对（新旧知识）→ LLM 对比具体内容 → 结构化 `{type: outdated, reason, evidence[{knowledgeId, quote}], suggestedAction: mark_outdated}` → 回调落 `evolution_alerts`。
- **Go**：接受建议后旧知识标记 `outdated`（详情页徽标，仍可检索但标注）。
- **依赖**：W4.2（alerts 表与工作流骨架）。**验收**：更新某知识后，旧版本/旧知识出现过时提醒。

### W4.4 冲突识别（FR-EVO-03）

- **AI 服务**：同一 `evolve-workflow` 增加冲突判定——对高相关候选对（相似度阈值可配，低于重复检测阈值），LLM 判断观点/数据是否矛盾 → `{type: conflict, reason, evidence[两条知识的冲突引文], suggestedAction: create_relation(contradicts)}`。
- **Go**：接受后自动建 `knowledge_relations(type=contradicts)`。
- **依赖**：W4.3（复用同一工作流与回调）。**验收**：AC-13 冲突部分——导入观点矛盾的两篇文档，出现冲突提醒并展示双方引文。

### W4.5 演化中心与人工审核（FR-EVO-05）

- **后端**：
  - `GET /api/v1/evolution/alerts?type=&status=&page=`（筛选/分页）。
  - `POST /api/v1/evolution/alerts/:id/resolve {action: accept_revision|keep_both|create_relation|mark_outdated|dismiss, note?}`——按 action 分别执行：写新版本（走 P1 版本链路）、建关系、标过时、忽略；全部记 `resolved_at` 与处理动作（实验采纳率数据源）。
- **前端**：新页面 `/home/evolution` 演化中心：
  - 提醒列表（类型/涉及知识/时间/状态），pending 置顶。
  - **证据对比视图**：左右并排展示涉及知识的 LLM 引文证据，操作按钮（接受修订/保留两者/建立关联/标记过时/忽略）。
  - 处理记录区（已解决/已忽略，可查看处理动作）。
- **首页聚合**：Dashboard 增加"待处理任务 + 演化提醒"卡片（点击进入演化中心）。
- **依赖**：W4.2/W4.3/W4.4。**验收**：AC-13 全部——提醒可筛选、证据可见、每种操作正确落库、处理记录可查。

## 里程碑验收（P4 Done）

1. AC-10：图谱页可用，与详情页互跳。
2. AC-11：重复检测（精确 + 语义）提醒正确到达。
3. AC-12：版本管理与演化联动（接受修订产生新版本，可回溯）。
4. AC-13：过时/冲突提醒带证据，人工审核五种操作全部生效。
5. 采纳/忽略行为落库，为 P5 演化实验（采纳率、误报率）提供数据。
