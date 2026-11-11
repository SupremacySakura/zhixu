---
task_id: relations-graph-20261111
status: completed
created: 2026-11-11
last_reviewed: 2026-11-11
applicable_to: "知识关系、关联推荐、关系管理、知识图谱、ai_tasks relation 任务"
---

# 知识关联推荐、关系管理与局部知识图谱（第四批）

## 背景

P3 的 W3.4/W3.5 与 P4 的 W4.1 依赖同一张 `knowledge_relations` 表：先有"AI 推荐—用户确认"的闭环积累已确认关系，图谱才有内容可画。设计前 SRS 与 phase-3 计划在关系类型、唯一键、候选数量、置信度阈值上互相矛盾，因此先用 ADR 固定模型，再按 5 个可独立验收的任务单（GitHub #2–#6）顺序实现。

## 结果

- 数据模型：`knowledge_relations`（user_id、source_id、target_id、relation_type、confidence、evidence JSON、status、source: ai/manual、confirmed_at、时间戳）；唯一键 `(user_id, source_id, target_id, relation_type)`，对称类型 related/contrasts 写入前按 min/max id 归一，其余严格有向。
- 五种类型：related / depends_on / supplements / contrasts / updates（相关、依赖、补充说明、对比、更新替代），中文映射在前端 `src/utils/relation.ts`。
- 手动闭环：`GET /api/v1/knowledge/:id/relations`、`POST /knowledge/:id/relations`、`PUT /relations/:id`（改类型）、`DELETE /relations/:id`（物理删除，前端二次确认）、`POST /relations/:id/confirm | /reject`（rejected 行保留并永久抑制同对建议；手动添加命中 rejected 行时复用为 confirmed + source=manual）。
- 自动推荐：向量化 ready 后入队 relation 任务（`dedupe_key=relation:<knowledgeID>`，不向对端扩散）；Go 侧用"标题 + 已采纳摘要（无摘要取正文前 1000 字）"嵌入一次 → Qdrant top 50 → 按 knowledge_id 归并取最高分 → 过滤自身/归档/已有关系对/已拒绝对 → 相似度 < 0.55 丢弃 → Top 10 送 AI。AI 侧 `POST /relate` 用 `agent.generate(..., {structuredOutput})` 返回至多 5 条、置信度 < 0.6 由 Go 再过滤，evidence 存 reason/source_quote/target_quote。
- 失败语义：候选 < 2、422、知识归档/删除、未配置模型 → 终端失败不重试；Qdrant/AI 超时与 5xx → 30/60/120 秒退避、首次 + 3 次重试，第 4 次失败转 failed；中文 error_message 区分"未配置对话模型 / 未配置 Embedding / AI 未启动 / 超时"。
- 图谱：`GET /api/v1/knowledge/graph?center_id=&depth=1&include_suggested=0&tag_id=&source_type=`；默认只画已确认关系，节点上限 100（按度数截断、中心不裁、响应标 truncated），无 center 取最近更新 50 条；筛选先筛节点再只保留两端都在集合内的边；非法 tag 格式 400，不存在标签/非法来源类型返回空集 200。
- 前端：详情页右栏「相关知识」面板（已确认 / 待确认两区、置信度、AI 理由展开、改类型、删除二次确认、查找进度、3 秒轮询与卸载清理）；`/home/knowledge/graph` 图谱页（ECharts 按需引入、力导向、按度数定节点大小、按来源类型着色、AI/手动边区分、关系类型边标签、建议边虚线、点击抽屉、中心搜索、标签/来源筛选、建议边开关、截断提示、空状态）；侧边栏新增「知识图谱」。
- 跨服务契约：Go→AI 同步调用 `POST {CHAT_AI_URL}/organize` 与 `/relate`（Bearer INTERNAL_API_SECRET），不实现 AI→Go 回调；AI 服务不对外、不校验 JWT。
- 依赖：前端新增 `echarts ^5.6.0`（按需引入，图谱页独立懒加载 chunk，约 459 kB / gzip 约 149 kB），lockfile 顺带补齐此前缺失的 `undici 7.30.1` 记录。

## 决策与理由

- 模型与术语见 [ADR-0001](../adr/0001-knowledge-relation-model.md) 与根 [GLOSSARY](../../GLOSSARY.md)：有向行存储 + 唯一键含类型，允许同一对多类型；对称类型归一避免同一语义存两条反向记录。
- 候选由 Go 生成而非让 AI 全库对比：先向量召回再让模型判定，控制 token 与延迟；只送 Top 10，避免一次判定几十条候选。
- 同步 Go→AI 而非 AI 回调：与既有 organize 一致，失败与超时在 Go 侧可观测、可重试；代价是长任务受 HTTP 超时约束（客户端 100 秒 / Mastra 90 秒）。
- 编辑器正文更新不自动重跑推荐：规格明确范围外；实现上向量化完成后仅当该知识从未有过 relation 任务才自动入队，手动「查找相关知识」始终可用。
- 建议抑制用 rejected 行而不是删除：保留"用户曾拒绝"的事实，避免同一对反复打扰；删除是物理删除，删后可重新被推荐。

## 教训

- **能力类型不能混用**：relation 任务最初把 embedding 能力当对话模型传给 `/relate`，正常配置下必然 502 直至失败；任务处理器必须分别解析并校验每类能力，测试要断言请求体里的 `model_id`。
- **来源类型白名单要单一事实源**：图谱来源筛选最初硬编码 manual/web，把 PDF/Markdown/Word 等文件导入知识静默过滤为空；改为复用 `knowledgeSourceTypes`，前端下拉与知识库一致。
- **ECharts 选中不要重建 series**：`setOption(option, true)` 会丢掉 force 布局的 preservedPoints 与 roam 状态，点节点即整图重排；选中态应走 `dispatchAction select/unselect`，只在数据变化时重建。
- **重试次数与退避档位要对齐**：注释/文档写 30/60/120 秒三次重试时，判定必须是"第 4 次失败才终态"，否则最后一档永远不会发生。
- **去重键必须含关系类型**：规格与 ADR 的唯一键含 relation_type，实现里若只按知识对去重，同一对的第二种类型会被静默丢弃。
- **复用被拒绝行时只改该改的字段**：确认 AI 建议应保留 source=ai，只有手动添加复用 rejected 行才改成 manual；否则"来源"事实被覆盖。
- **测试通过不等于验收通过**：图谱页、边标签、缩放与抽屉交互最终仍需真机手测（本轮服务未启动，未做）。

## 2026-11-11 修复：嵌入文本超长导致「查找相关知识」必失败

- **现象**：点「让 AI 找相关知识」约 3.5 分钟后失败，面板只显示「Embedding 调用失败，请检查 Mastra、模型地址、凭据及额度」，Mastra 与 Go 日志都没有错误行。
- **根因**：relation 任务发给 `/vectorize` 的文本是「标题 + 换行 + 代表文本（1000 字）」，而 AI 侧 schema 把单条文本卡在 1024 字符。中文标题 ≥24 字时总长即超限 → `safeParse` 失败 → AI 返回 400（catch 分支不打日志）→ Go 把任何非 200 都压成同一句可重试错误 → 30/60/120 秒退避重试 3 次后 failed。实测：知识 #33/#34（标题 26/28 字、无已采纳摘要）每次必失败；#35 在采纳 169 字摘要后文本变短，同一任务随即成功。
- **修复**：
  - `apps/zhixu-ai/src/lib/vectorize.ts`：单条文本上限 1024 → 4096（覆盖「标题 200 字 + 代表文本 1000 字」的最坏 UTF-16 长度，也覆盖长提问的查询向量化）。
  - `apps/zhixu-backend/internal/service/relation_task.go`：标题计入嵌入文本预算，先拼接再按 `RelationEmbeddingTextMaxRunes`（代表文本 1000 + 标题 200 + 换行）截断，长度不再依赖远端上限。
  - `apps/zhixu-backend/internal/service/embedding.go`：400/403 返回 `EmbeddingRejectedError`（带状态码），5xx/超时文案也带状态码；relation 任务对前者直接判终态，不再空转重试。
  - `apps/zhixu-ai/src/mastra/vectorize.ts`：catch 里加 `console.error`（只记错误摘要，不记正文、凭据与供应商响应）。
- **测试**：AI 侧 1029 字符文本通过、4097 拒绝；Go 侧 400 归类为拒绝、502 仍可重试、200 字标题下嵌入文本不超预算、被拒绝时终态失败且不调模型。

## 已知限制（候选改进）

- 图谱关系查询未加 LIMIT，超级枢纽节点会把全部关联行读入内存后再截断，超大关系量有放大风险。
- 候选阶段仍按"无序知识对"过滤已有任意类型关系，因此同一对的第二种类型只能手动添加，AI 不会再推荐；与"同对允许多类型"存在张力，属既有取舍。
- 确认接口的 Get→UpdateStatus 非原子，存在极窄并发竞态。
- relation 任务 3 分钟租约小于最坏耗时（候选嵌入 110 秒 + 判定 100 秒），极慢供应商下可能被回收重跑。
- 图谱页主题切换后颜色不自动重取（挂载时解析一次 CSS 令牌）；标签下拉一次性拉取全部标签，无分页。
- 级联删除的仓储测试只断言 DELETE 语句被调用，未连真实 MySQL 验证删除后不可查。

## 验证

- `pnpm verify` 全绿：根级 lint、typecheck、前端 Vitest（46 文件 / 310 项）、前端 build、Go build 与 go test（config/database/handler/middleware/repository/router/service/utils）。
- `pnpm typecheck:ai` 通过（AI 侧 51 项测试）；AI 侧无新增依赖。
- 2026-11-11 修复嵌入超长后：用知识 #34 的真实标题 + 1000 字正文（1029 字符）复跑，schema 通过、Ollama 返回 768 维向量；`pnpm verify` 全绿。
- 未 push、未开 PR；真机端到端（导入相似文档 → 建议出现 → 确认 → 图谱可见）需用户重启 `pnpm dev` 与 Mastra 后手测。
