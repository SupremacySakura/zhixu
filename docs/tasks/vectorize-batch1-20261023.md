---
task_id: vectorize-batch1-20261023
status: completed
created: 2026-10-23
last_reviewed: 2026-10-23
applicable_to: "知识库向量化、Mastra Embedding 网关、Qdrant 写入、后台任务与状态展示"
---

# 知识向量化第一批：自动触发与状态展示

## 结果

- 知识入库成功后在同一事务内写入 `vectorize_tasks`，后台 worker 每 3 秒补建缺失任务并排空队列；失败任务不会被 backfill 重复触发，避免重启后重放付费调用。
- 任务状态为 `pending/running/ready/partial`，分块状态为 `pending/ready/failed`；知识列表与详情页展示状态、成功/失败分块数、Embedding 模型与版本、失败原因。
- 新增分块页 `/home/knowledge/:id/chunks`：按章节聚合大纲、展示分块序号与字符区间、高亮相邻块重叠内容、支持只看失败分块；任务活跃时每 3 秒轮询。
- Markdown 分块为 512 字符窗口、64 字符重叠，按 Unicode 字符切分，按标题维护 `" / "` 章节路径，跳过代码围栏中的伪标题，优先在空白边界截断。
- Embedding 只经由 Mastra：Go 生成一次性 capability 并调用 `POST /vectorize`，Mastra 回调 Go 内部网关 `/internal/chat/v1/embeddings`，Go 校验固定批次、模型与端点后代理真实供应商；后端不直连模型供应商。
- 写入 Qdrant 前校验向量数量/维度/有限值/非零；`Upsert` 使用 `wait=true` 并要求 `result.status == "completed"`；单批失败立即停止后续批次，不继续产生付费调用。
- collection 名为 `zhixu_u{userID}_v{embeddingVersion}_c{configVersion}`；配置版本变化时中止旧任务并给出明确错误，避免新旧向量混写。
- 任务领取使用 `FOR UPDATE SKIP LOCKED` + 3 分钟租约 + fencing 事务（先锁任务再锁知识行），多 worker 下不会重复写入。
- 删除知识时连带删除向量任务与分块记录；列表新增 `vector` 筛选项，接口新增 `GET /knowledge/{id}/chunks`，Swagger 已重新生成。

## 教训

- 批量外部调用必须“先校验、后写入、失败即停”：只要批次返回数量或维度不符，就应放弃整批而不是部分写入，否则 Qdrant 中会残留不可解释的向量。
- 测试替身必须与真实契约同形。Embedding fake 最初返回固定数量向量，导致 `ValidateEmbeddings` 判定数量不符，测试表现为“没有写入 Qdrant”；fake 应按输入 `texts` 动态生成向量。
- 一次性 capability 的回调边界要精确到 origin + pathname，并校验无凭据参数；仅校验域名不足以防开放代理。
- 后台任务的重试策略要区分“未建任务”和“已失败任务”：backfill 只补建缺失任务，避免服务重启把所有失败任务重新变成付费调用。
- 配置版本化不能只体现在数据字段上，必须参与 collection 命名并在执行中再次比对；否则用户在任务运行中修改模型会写坏索引。
- 前端展示分块时，重叠内容需要按字符数组切分而非 `slice`，否则含 emoji/多字节字符时高亮位置会错位。

## 可沉淀模式

- **服务端 AI 调用网关**：后端只保存模型配置与凭据，AI 服务持有实际调用能力；通过短时效、单次使用、固定请求体摘要的 capability 把权限收敛到单次调用。
- **可恢复的后台任务**：`pending → running（租约）→ ready/partial`，配合 `SKIP LOCKED`、lease token 与 fencing 写库；进程崩溃后任务由租约过期自动回收。
- **向量写入校验链**：数量 → 维度 → 数值有效性 → 非零向量 → 存储确认，任一环节失败都不进入下一批。
- **知识分块展示**：标题分节 + 固定窗口重叠 + 章节大纲 + 重叠高亮，既能让用户理解切分方式，也便于定位失败分块。
- **本机最小依赖启动**：Qdrant 单独以 Docker 运行（`127.0.0.1:6333`），Mastra 通过 `pnpm dev:ai` 启动（`127.0.0.1:4111`），不要求完整 Compose 编排。

---

*本文件由 docs/tasks/.template.md 模板生成*
