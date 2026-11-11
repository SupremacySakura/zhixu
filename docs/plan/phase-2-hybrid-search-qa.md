---
last_reviewed: 2026-11-10
phase: P2
status: pending
---

# P2 混合检索与问答补全

> 对应《产品设计文档》开发计划阶段 4 的收尾。检索从"纯向量单路"升级为"关键词+向量双路 RRF 融合"，问答硬化为"命中基于知识库、未命中声明后通用回答"的双模式。
> 验收映射：**AC-05**（混合检索）、**AC-06**（带引用 AI 问答）、**AC-09**（分块预览高亮）。

## 需求变更记录（2026-11-10，用户确认）

- **FR-QA-06 拒答改为"未命中声明"**：不再做 Go 侧预检索门禁与固定拒答话术。保留"模型自主决定是否调用知识库"的工具驱动设计：未调用检索或检索未命中时，模型在回复中明确说明知识库没有相关内容，然后基于自己的训练知识回答；命中时基于知识库回答并展示来源。引用防伪造机制（引用卡片只由工具结果重建）保证通用知识回答不会出现伪造引用。
- **架构基线**：问答 Agent 在 [chat.ts](../../apps/zhixu-ai/src/mastra/chat.ts) 内联构建（SYSTEM_PROMPT），检索由 AI 服务以工具形式回调 Go 的 `POST /internal/tools/v1/search`（RagSearcher）。Mastra starter 遗留的占位 Agent 已在 W2.4 删除，不参与问答。Go/Mastra 分工（Go 管检索/持久化/能力令牌，AI 管 Agent/提示词/工具编排）保持不变。
- **RRF 融合放 Go 侧**：架构文档原计划 AI 服务侧经 `/v1/retrieve` 融合，但该入口不存在；现状检索入口就是 Go，Go 侧融合改动最小且保留降级能力。原计划中的 `GET /internal/files/keyword-search` 契约不再需要。

## 现状基线

- 语义检索已就绪：RagSearcher（Qdrant，payload 强制 `user_id`，TopK 与 minScore=0.5 按用户设置）。
- 引用已防伪造（Go 侧按 chunkId 回填）并持久化 `ChatTurnSource`；SSE 流式、会话历史可用。
- [chat.ts](../../apps/zhixu-ai/src/mastra/chat.ts) SYSTEM_PROMPT 已有检索时机与"检索不到如实告知"的基础约束，但未覆盖"未命中后允许通用知识完整回答"的双模式语义。
- 缺口：无 ngram 全文索引；无 RRF；无关键词/向量/混合三模式切换；引用跳转不能定位到具体分块；无检索日志。

## 工作项

### W2.1 关键词检索（FR-QA-01）

- **数据库**：`knowledge_chunks.content` 建 ngram 全文索引（`ALTER TABLE ... ADD FULLTEXT INDEX ... WITH PARSER ngram`，GORM 迁移用原生 SQL；ngram_token_size 保持默认 2，Docker MySQL 需确认 `ngram_token_size` 配置一致）。
- **后端（Go）**：`RagChunkStore` 增加 `SearchChunksKeyword(userID, query, limit)`：`MATCH(content) AGAINST(? IN NATURAL LANGUAGE MODE)`，JOIN knowledge_items 过滤 `archived_at IS NULL` 与 `state='ready'`，按相关度排序。
- **依赖**：无。**验收**：中文二字词查询能命中分块，结果按相关度排序；`user_id` 与归档隔离生效。

### W2.2 混合检索 RRF 融合（FR-QA-03）

- **方案**：融合放 **Go 侧 RagSearcher**——向量路（现有逻辑原样）与关键词路（W2.1）各自取候选，RRF（k=60）融合去重后按融合分排序取 topK。
- **返回结构**：RagSource/KnowledgeSource 增加 `source: keyword|semantic|both`，随 `ChatTurnSource.Sources` JSON 持久化（实验分组与溯源用）。
- **模式切换**：RagQuery 增加 `mode: hybrid（默认）|semantic|keyword`，经内部搜索网关参数透传，供三模式实验对比。
- **降级**：Embedding/Qdrant 不可用 → 仅关键词路；关键词路异常 → 仅语义路（记日志）。任一路可用即返回。
- **依赖**：W2.1。**验收**：AC-05——三模式可切换，混合模式 RRF 生效；抽 5 条标准问题人工核对混合模式结果不差于任一单路。

### W2.4 Agent grounding 双模式与输入限制（FR-QA-04 补全、FR-QA-06 变更后）

- **[chat.ts](../../apps/zhixu-ai/src/mastra/chat.ts) SYSTEM_PROMPT 重写**：
  - 命中模式：基于 `search_knowledge` 返回片段回答，事实性内容后标注 [n] 引用，编号与工具返回顺序一致（现有规则保留）。
  - 未命中模式：未调用检索（寒暄/通用问题）或检索无命中时，先明确说明"知识库中没有找到相关内容"，再基于自己的训练知识回答；该模式下**不得出现 [n] 引用**。
  - 保留现有工具契约、每轮最多三次检索、指代展开规则；引用防伪造机制不变（Go 侧重建引用卡片）。
- **问题长度**：前端输入框 ≤2000 字符（计数器 + 超限禁止发送）；Go `ValidateChat` 用户消息限 2000（助手历史消息维持 8000，避免截断历史会话）。
- **starter 清理**：已删除 Mastra starter 模板 `agents/agent.ts` 与 `tools/schedule-tools.ts`（带 web_search/web_fetch 工具且无人使用，属攻击面与混淆源），`index.ts` 注册同步清理。
- **依赖**：无。**验收**：AC-06——命中时答案基于检索内容且引用全部能回填真实分块；未命中时回复先声明知识库无相关内容再用通用知识回答，且无伪造引用。

### W2.5 引用高亮定位（FR-KM-08 收尾、AC-09）

- **数据补全**：RagSource/KnowledgeSource 增加 `chunk_id` 与 `seq`（Go 检索已有，透出即可）；AI 侧 searchResultSchema 与 lib/rag.ts 同步加字段并透传到 sources 事件；`ChatTurnSource` 无需加列（随 Sources JSON 持久化）。
- **前端**：引用卡片链接携带 `?chunk=<seq>`；[KnowledgeChunks.vue](../../apps/zhixu-frontend/src/pages/KnowledgeChunks.vue) 读取参数后复用现有锚点滚动机制定位到对应分块并临时高亮。
- **依赖**：无。**验收**：从问答引用点击 → 分块预览页定位到具体分块并高亮。

### W2.6 检索日志（P5 实验前置埋点）

- **数据库**：`retrieval_logs`（id、user_id、conversation_id、query、mode: keyword/semantic/hybrid、hits JSON（chunk_id+score+source）、hit_count、latency_ms、created_at）。问答工具检索与独立检索都记录。
- **说明**：此表是实验数据源（Recall@K、MRR、未命中声明正确率）；拒答指标随 FR-QA-06 变更移除，改为记录命中数与模式分布。
- **依赖**：W2.2。

## 里程碑验收（P2 Done）

1. AC-05：关键词/向量/混合三模式可切换，混合模式 RRF 生效。
2. AC-06：双模式生效——命中基于知识库且引用可回填；未命中先声明再通用回答且无伪造引用；引用可跳转。
3. AC-09：引用跳转定位高亮分块。
4. 检索日志落库，三模式对比实验数据可直接导出。
