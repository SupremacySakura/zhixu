---
last_reviewed: 2026-11-12
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

## 方案调整记录（2026-11-12，第五批设计定稿）

第五批 = 本文件剩余工作项 W4.2~W4.5，规格见 GitHub Issue #8（`第五批：知识演化检测与演化中心`）。讨论中确认以下口径，与下文原文不一致处以本记录为准：

- **契约简化为 Go→AI 单向同步调用**：新增 `POST {CHAT_AI_URL}/evolve`，同步返回结构化判定；不实现架构文档 5.1/5.2 的 `POST /v1/evolve/check` 异步与 `POST /internal/ai/files/{fileId}/evolution-alerts` 回调。理由同 P3（organize/relate 均已简化）。
- **提醒独立成表**：`evolution_alerts` 不复用 `knowledge_relations`。关系表状态表达「关系是否存在」，提醒状态表达「审计结论是否被人看过」，两套生命周期混表会让 `uniq_relation_pair` 失效（同一对知识可同时有重复与冲突两条提醒）。
- **冲突关系类型为 `contradicts`**：新增第六种关系类型（对称），推翻 ADR-0001 的「合并进 contrasts」决定，见 [ADR-0002](../adr/0002-conflict-relation-type.md)。AI 关联推荐不产出该类型，只能由用户或冲突提醒的 `create_relation` 动作建立。
- **「过时」不是知识状态**：`knowledge_items.status` 是处理状态（pending/running/ready/failed），不能承载 `outdated`。改为新增 `outdated_at`、`outdated_reason` 两列，不改变可检索性。
- **精确重复需要 `content_hash`**：现有 `SourceHash` 在文件导入时是文件哈希、手动创建时才是正文哈希，不能直接用于精确重复。新增 `content_hash`（正文规范化后 sha256），全来源统一计算。
- **重复提醒的裁决不含合并**：采纳 = 保留一条、归档另一条（复用 `archived_at`）；「保留两者」并列可选。真合并写入范围之外。
- **状态与动作**：`status` 四值 pending/resolved/dismissed/deferred；`action` 六值 accept_revision/keep_both/archive_one/create_relation/mark_outdated/mark_verified；SRS 的「采纳/忽略/暂不处理/标记已核实」分别映射到这六个动作、`dismiss`、`deferred`、`mark_verified`。
- **过时提醒带严重度**：`severity` ∈ review/strong，由证据强度决定；引文无法在原文定位或知识不属于当前用户时降级或丢弃（SRS FR-EVO-03 降级要求）。重复与冲突固定 review。
- **阈值仍用 Go 常量**（`EvolveDuplicateMinScore` 等），不进 `Config`，与 relation 的 `RelationMinScore` 保持一致。
- **触发**：新建/导入/编辑后向量化就绪时入队 `evolve` 任务（`dedupe_key=evolve:<knowledgeID>`）；演化中心与详情页提供手动「重新检查」；不做全库定时扫描。同一知识的版本之间不产生过时提醒（只做跨条目判定）。
- **前端四块**：`/home/evolution` 演化中心（W4.5）、详情页过时徽标（W4.3）、图谱渲染 contradicts 边（W4.4）、Dashboard 待处理提醒卡片（W4.5 收尾）。
- 术语以根 `GLOSSARY.md` 新增的「知识演化」分区为准。

## 工作项

### W4.1 知识图谱可视化（FR-REL-03）✅ 已完成（2026-11-11，第四批先行）

- **后端**：`GET /api/v1/knowledge/graph?center_id=&depth=1&include_suggested=0&tag_id=&source_type=`——节点=知识条目（标题/来源类型/摘要、度数），边=关系（类型/置信度/来源）；默认只画已确认关系，`include_suggested=1` 时叠加建议关系；默认排除归档知识；`depth` 固定 1（局部视图）；节点上限 100，超过按连接度截断并在响应标 `truncated`；无 `center_id` 时取最近更新的 50 条知识及其间的已确认关系。
- **筛选（FR-REL-03 硬要求）**：`tag_id` 与 `source_type` 同时作用于节点与边（只保留两端都被筛中的边）；前端工具栏两个下拉，标签列表复用 `GET /api/v1/tags`。
- **前端**：新页面 `/home/knowledge/graph`（ECharts 按需引入 core + GraphChart + CanvasRenderer），侧边栏「知识图谱」菜单（`HomePage.vue` 的 `NavItems`）：
  - 局部分力导向布局；节点大小按连接度、颜色按来源类型；边样式区分 ai/manual，标签显示关系类型。
  - 交互：点击节点打开右侧抽屉（标题/摘要/一跳关系，可跳详情或「以此为中心」重绘）、顶部中心知识搜索框、显示建议边开关、图例。
  - 空状态引导「先去知识详情页建立关系」。
- **依赖**：P3-W3.4/W3.5。**验收**：AC-10——图谱展示节点与关系，点击进入详情，局部渲染流畅（百节点级）。

**完成情况（2026-11-11）**：`GET /api/v1/knowledge/graph`（center/depth/include_suggested/tag_id/source_type、节点上限 100 与 truncated、默认排除归档）与 `/home/knowledge/graph` 页面（ECharts 按需引入、力导向、筛选、建议边虚线、点击抽屉、中心搜索、空状态）已实现并合入；验收细节见 [docs/tasks/relations-graph-20261111.md](../tasks/relations-graph-20261111.md)。AC-10 待真机验收。

### W4.2 重复检测（FR-EVO-01）

- **数据库**：`evolution_alerts`（id、user_id、alert_type: duplicate/outdated/conflict、subject_ids JSON（涉及的知识 id 列表）、payload JSON（LLM 结构化判定：type/reason/evidence[{knowledgeId,quote}]/suggestedAction）、status: pending/resolved/dismissed、created_at/resolved_at）。
- **检测逻辑**（Go 侧编排，导入完成钩子触发）：
  1. **精确重复**：复用 `content_hash`/秒传 hash——hash 相同直接产生 duplicate 提醒（无需 LLM）。
  2. **语义近重复**：新知识向量在 Qdrant 找 Top5 近邻，相似度 >0.9（阈值入配置）→ 入 candidate → 走 `POST /v1/evolve/check` 由 LLM 复核（防误报）→ 回调 `POST /internal/ai/files/{fileId}/evolution-alerts`。
- **依赖**：W3.1（ai_tasks，task_type=evolve）。**验收**：AC-11——导入与已有知识语义近重复的文档，演化中心出现提醒。

**完成情况（2026-11-12，第一部分 / GitHub #9）**：提醒骨架与精确重复已落地——`evolution_alerts` 表（含 `dedupe_key` 唯一索引且终态不清空）、`knowledge_items` 的 `content_hash`（规范化后 sha256，历史数据每批 500 条回填）、`evolve` 任务（幂等键 `evolve:<id>`，向量化就绪入队，载荷带 `base_version` 做版本栅栏）、精确重复判定（正文指纹相同即产提醒，不调模型）、列表/摘要/裁决接口（六动作 + defer，按类型裁剪）。

**完成情况（2026-11-12，第二部分 / GitHub #10）**：语义近重复与演化判定契约已落地——AI 服务新增 `POST /evolve`（与 `/relate` 同构：鉴权、结构校验、模型地址校验、120s 超时、504/422/403/502 错误码）；Go 侧 `EvolveClient` 同步调用，候选合并「updates 关系指向的目标」（强，只接受过时结论）与「向量相似度 ≥ 0.80 近邻」（弱），按相似度降序取前 8 送模型；一次调用同时产出重复/过时/冲突三类结论，Go 按相似度分派（≥0.90 只收 duplicate、0.75~0.90 只收 conflict、强候选只收 outdated），本阶段只消费 duplicate，过时与冲突的落库在 #11/#12 接同一处理器；证据引文按知识正文子串校验，定位不到即剔除、全剔除则整条不落库，部分缺失降级为 `review`。

### W4.3 过时识别（FR-EVO-02）

- **触发**：新版本知识入库（P1 编辑产生新 version）或导入与已有知识高相关的新知识时，对"被 updates 关系指向的旧知识"做判定。
- **AI 服务**：`evolve-workflow`——候选对（新旧知识）→ LLM 对比具体内容 → 结构化 `{type: outdated, reason, evidence[{knowledgeId, quote}], suggestedAction: mark_outdated}` → 回调落 `evolution_alerts`。
- **Go**：接受建议后旧知识标记 `outdated`（详情页徽标，仍可检索但标注）。
- **依赖**：W4.2（alerts 表与工作流骨架）。**验收**：更新某知识后，旧版本/旧知识出现过时提醒。

**完成情况（2026-11-12，第三部分 / GitHub #11）**：过时识别与详情页标记已落地——`EvolveProcessor` 放行 `outdated` 结论（仍挡 `conflict` 留给 #12），候选取「`updates` 关系指向的目标」（强信号，单侧证据即可）与「向量相似度 ≥ 0.80 近邻」（弱信号，必须双侧各至少一条可定位引文，否则整条不落库）；相似度分派放宽为「≥0.90 收 duplicate、≥0.75 收 conflict、≥0.80 收 outdated」，因此 0.80~0.90 段同时接受冲突与过时两类结论。裁决侧 `mark_outdated` 写目标知识（`subject_ids[1]`）的 `outdated_at` 与 `outdated_reason`，`mark_verified` 只对过时提醒清除标记（重复提醒的核实不会误清别的判定），两者都记 `knowledge_outdated`/`knowledge_verified` 操作日志。编辑正文（版本 +1）会清除过时标记——标记针对被判定时的正文，用户改写即视为自行处理。详情页新增过时横幅（含判定理由与判定时间）与「重新检查」按钮（`POST /knowledge/:id/evolve/regenerate`，已有活跃任务时后端复用）。

### W4.4 冲突识别（FR-EVO-03）

- **AI 服务**：同一 `evolve-workflow` 增加冲突判定——对高相关候选对（相似度阈值可配，低于重复检测阈值），LLM 判断观点/数据是否矛盾 → `{type: conflict, reason, evidence[两条知识的冲突引文], suggestedAction: create_relation(contradicts)}`。
- **Go**：接受后自动建 `knowledge_relations(type=contradicts)`。
- **依赖**：W4.3（复用同一工作流与回调）。**验收**：AC-13 冲突部分——导入观点矛盾的两篇文档，出现冲突提醒并展示双方引文。

**完成情况（2026-11-12，第四部分 / GitHub #12）**：冲突识别、修订稿采纳与矛盾关系已落地——关系类型新增第六种 `contradicts`（对称，写入前 min/max 归一，唯一键 `(user_id, source_id, target_id, relation_type)` 不变），并新增 `RelationType.Recommendable()` 作为 AI 关联推荐的白名单（五值 true、`contradicts` false），`relate.go` 与 `relation_task.go` 两处清洗改用它——`Valid()` 是「所有合法类型」，直接当白名单用会让模型建议出矛盾关系，ADR-0002 明确要求排除；冲突提醒接入同一条 evolve 管线（删除 #11 留下的 `if conflict { continue }` 跳过分支），相似度 0.75~0.90 的候选只接受冲突结论，同一对同时给出重复与冲突时按 `evolveTypePriority` 只留重复；修订稿由同一次模型调用产出并存入提醒载荷，门槛在客户端清洗（`sanitizeEvolveJudgements`）与处理器（`buildAlert` 的 `evolveRevisionUsable`）两处各守一道：证据未全部定位或正文超 20000 字即整份丢弃（不截断），提醒严重度降级为 `review`，建议动作随之从 `accept_revision` 改回 `create_relation`；裁决侧 `accept_revision` 经 `EvolutionService.Reviser` 调到 `KnowledgeService.AcceptRevision`，走既有版本链路写为新版本（`change_source=ai`），原文进版本快照、不覆盖历史，无修订稿时 effectFor 直接返回 `ErrEvolutionInvalid` 由 handler 映射 400；`create_relation` 经 `EvolutionService.Relations` 以 `subject_ids[0]` 为源、`[1]` 为目标写一条 `confirmed`+`manual` 的 `contradicts`，同对已存在时由仓储 `findPair`/`confirm` 复用而不重复插入。版本来源语义随之补全：`knowledge_items` 新增 `change_source`(manual/ai) 记录**当前正文**的产生来源，版本快照行记录**被替换掉的旧正文**的来源，版本列表的「当前」条目据此标注（此前硬编码 manual，采纳修订只能错记在被替换的旧版本上）。图谱侧后端无需改动（`relation_type` 原样透出、不按类型过滤），前端补 `contradicts` 的「矛盾」标签与选项（仍是 `utils/relation.ts` 唯一文案源）、矛盾边改用 `--color-danger` 系而非按来源分色（类型比来源更需要一眼可辨），图例项仅在图中真的出现矛盾边时显示。测试覆盖：对称归一与幂等复用、AI 推荐不产出矛盾、修订稿门槛三分支（客户端清洗 + 处理器各一）、采纳修订生成新版本且原文入快照、无修订稿 400、矛盾关系不重复、图谱返回矛盾边。后端 `go build`/`go vet`/`go test ./...` 全绿；前端 `npm test` 与 `npm run typecheck` 全绿（本单范围内 47 文件 324 用例，合并 #13 后 52 文件 354 用例）。

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

**完成情况（2026-11-12，第五部分 / GitHub #13）**：演化中心与首页待处理卡片已落地——新增 `/home/evolution` 页面（侧边栏「演化中心」入口），单页两页签：「待处理」走 `open=true` 后端一次取回 pending+deferred，「处理记录」并发查 resolved 与 dismissed 两路再按 `resolved_at` 倒序合并（总数取两路之和，翻页沿用既有 `FilePagination`）；类型三选（重复/过时/冲突）与状态筛选都用既有 `FilterMenu`，任一筛选变化即复位到第一页重拉，且状态筛选存在时不传 `open`（后端 `open` 优先于 `status`，同传会静默覆盖）。点列表项从右侧滑出详情抽屉（与图谱节点抽屉同一套 `Motion` + `springSnappy` + `bg-scrim` 结构），左右并排两侧证据引文，并把两条引文的最长公共片段用 `<mark>` 高亮，标题可跳对应知识详情。动作按钮按提醒类型裁剪（duplicate 五个、outdated 四个、conflict 六个，含「暂不处理」的状态迁移），无修订稿时「采纳修订」置灰并给出原因，涉及知识已归档时卡片标注「涉及知识已归档」且只留「忽略」与「标记已核实」可点。备注随裁决一并提交，处理记录页签展示动作文案（复用 `evolutionActionLabel`）、备注与裁决时间。首页新增「待处理提醒」卡片，只调 `GET /api/evolution/alerts/summary` 拿总数与三类计数并链到演化中心，不拉列表、不引图表库。列表/筛选/裁决状态收在 `useEvolutionAlerts` 组合式函数里（AbortController 管理取消，scope 销毁时中断在途请求），页面只做渲染编排。前端新增 30 个用例（接口层 5、组合式函数 8、高亮工具 6、页面 8、首页卡片 3），全量 349 个用例通过。AC-13 的「每种操作正确落库」需真机验收——其中 `accept_revision` 与 `create_relation` 后端当前返回 409「该裁决动作尚未启用」，页面按服务端文案提示而不静默失败。

## 里程碑验收（P4 Done）

1. AC-10：图谱页可用，与详情页互跳。
2. AC-11：重复检测（精确 + 语义）提醒正确到达。
3. AC-12：版本管理与演化联动（接受修订产生新版本，可回溯）。
4. AC-13：过时/冲突提醒带证据，人工审核五种操作全部生效。
5. 采纳/忽略行为落库，为 P5 演化实验（采纳率、误报率）提供数据。
