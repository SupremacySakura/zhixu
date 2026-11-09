---
last_reviewed: 2026-11-09
phase: P2
status: pending
---

# P2 混合检索与问答补全

> 对应《产品设计文档》开发计划阶段 4 的收尾。检索从"纯向量单路"升级为"关键词+向量双路 RRF 融合"，问答补齐 grounding 约束与拒答。
> 验收映射：**AC-05**（混合检索）、**AC-06**（带引用 AI 问答）、**AC-09**（分块预览高亮）。

## 现状基线

- 语义检索已就绪：`POST /v1/retrieve`（Mastra retrieveKnowledge，Qdrant，payload 强制 `user_id`，TopK 与 minScore=0.5 按用户设置）。
- 引用已防伪造（Go 侧按 chunkId 回填）并持久化 `ChatTurnSource`；SSE 流式、会话历史可用。
- 缺口：无 ngram 全文索引（grep 无 MATCH/AGAINST）；无 RRF；[agent.ts](../../apps/zhixu-ai/src/mastra/agents/agent.ts) instructions 仍是 starter 模板；无拒答话术；引用跳转不能定位到具体分块。

## 工作项

### W2.1 关键词检索（FR-QA-01）

- **数据库**：`knowledge_chunks.content` 建 ngram 全文索引（`ALTER TABLE ... ADD FULLTEXT INDEX ... WITH PARSER ngram`，GORM 迁移用原生 SQL；ngram_token_size 保持默认 2，Docker MySQL 需确认 `ngram_token_size` 配置一致）。
- **后端（Go）**：
  - 服务层 `SearchChunksKeyword(userID, query, limit)`：`MATCH(content) AGAINST(? IN NATURAL LANGUAGE MODE)`，按相关度排序，过滤 archived 知识。
  - 内部接口 `GET /internal/files/keyword-search?userId=&query=&limit=`（架构契约，供 AI 服务调用）。
- **依赖**：无。**验收**：中文二字词查询能命中分块，结果按相关度排序；`user_id` 隔离生效（架构附录已有验收用例）。

### W2.2 混合检索 RRF 融合（FR-QA-03）

- **方案**（对齐架构文档"Go 总管"）：**RRF 融合放 AI 服务侧**编排——`POST /v1/retrieve` 内部并行调 Qdrant（语义路）与 `GET /internal/files/keyword-search`（关键词路），RRF（k=60）融合去重后按融合分排序返回。
  - 备选：Go 侧编排两路。决策点：谁调用谁。架构契约中 retrieve 已是 AI 服务入口，故首选 AI 侧融合；Go 保留关键词路实现与降级能力。
- **返回结构**：RetrievalHit 增加 `source: keyword|semantic|both`，持久化到 `ChatTurnSource`（实验分组与溯源用）。
- **降级**：Qdrant 不可用 → 仅关键词路；关键词路异常 → 仅语义路（记日志）。
- **依赖**：W2.1。**验收**：AC-05——同一查询分别仅关键词/仅向量/混合三模式可切换（实验需要），混合模式结果不差于任一单路（抽 5 条标准问题人工核对）。

### W2.3 检索充分性判断与拒答（FR-QA-06）

- **位置**：Go 编排层在转发 `/v1/chat` 前先调 retrieve（或复用现有 prep-assistant 流程的检索结果）：
  - `len(hits)==0` 或 `top1.score < minScore`（沿用用户可配的 0.5）→ **直接返回固定拒答话术，不调用 LLM**，SSE 正常关闭，usage 记录为拒答（token=0）。
  - 话术："根据当前知识库内容，无法回答该问题。建议补充相关文档后重试。"（前端以普通助手消息展示，不标引用）
- **指标埋点**：拒答事件计数（P5 拒答正确率实验用）。
- **依赖**：无（可与 W2.1 并行）。**验收**：知识库为空或提问无关内容时收到固定话术，无编造引用。

### W2.4 Agent grounding 与查询改写（FR-QA-04 补全）

- **[agent.ts](../../apps/zhixu-ai/src/mastra/agents/agent.ts) instructions 重写**：
  - 仅依据 `search_knowledge` 工具返回内容作答；答案中使用的每个事实必须对应至少一个引用 ID；证据不足时必须明确说明"根据当前知识库内容无法回答"，禁止编造与使用通用知识补全。
  - 保留现有 `searchKnowledgeByText` 工具契约（Go 编排层检索，引用防伪造机制不变）。
- **查询改写**（增强，优先级低）：agent 内对多轮指代消解/口语化查询先做 rewrite 再检索；不影响契约。
- **问题长度**：前端输入框 ≤2000 字符（计数器 + 截断禁止），Go 侧同样校验（>2000 → 400）。
- **依赖**：无。**验收**：AC-06——答案只包含检索命中的内容；无命中时拒答；引用 ID 全部能回填到真实分块（现有校验继续生效）。

### W2.5 引用高亮定位（FR-KM-08 收尾、AC-09）

- **前端**：引用卡片链接携带 `?chunk=<chunkId>`；[KnowledgeChunks.vue](../../apps/zhixu-frontend/src/pages/KnowledgeChunks.vue) 读取参数后滚动到对应分块并高亮（复用现有目录定位机制，补 chunkId 锚点）。
- **后端**：确认 `ChatTurnSource` 持久化包含 chunkId（现有结构已含 chunk 信息，若无则补列）。
- **依赖**：无。**验收**：从问答引用点击 → 分块预览页定位到具体分块并高亮。

### W2.6 检索日志（P5 实验前置埋点）

- **数据库**：`retrieval_logs`（id、user_id、conversation_id、query、mode: keyword/semantic/hybrid、hits JSON（chunkId+score+source）、latency_ms、refused bool、created_at）。问答与独立检索都记录。
- **说明**：此表是实验数据源（Recall@K/MRR/拒答正确率），随 W2.2/W2.3 一起落地，避免 P5 补埋点返工。
- **依赖**：W2.2、W2.3。

## 里程碑验收（P2 Done）

1. AC-05：关键词/向量/混合三模式可切换，混合模式 RRF 生效。
2. AC-06：grounding 生效（无证据不编造）、拒答话术正确、引用可跳转。
3. AC-09：引用跳转定位高亮分块。
4. 检索日志落库，三模式对比实验数据可直接导出。
