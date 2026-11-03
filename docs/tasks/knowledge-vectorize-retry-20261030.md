---
task_id: knowledge-vectorize-retry-20261030
status: completed
created: 2026-10-30
last_reviewed: 2026-10-30
applicable_to: "知识库、向量化、分块重试、操作日志"
---

# 知识向量化失败分块重试

## 背景

向量化走「单任务驱动分块队列」模型：`vectorize_tasks` 持有整条知识的状态租约，`knowledge_chunks` 按块跟踪 pending/running/ready/failed。此前分块失败只能删除整条知识重新导入，没有失败块级别的重试入口；详情页向量面板只能看状态，不能操作。

## 结果

- 新接口 `POST /api/knowledge/:id/vectorize/retry`（JWT 保护，仅本人物品）：仅允许知识状态 `ready` 且向量状态 `partial` 的条目。同一事务内任务重置为 `pending`（清空租约令牌、租约过期时间与退避时间，按 worker 锁顺序先锁任务行再锁知识行；`attempts` 为单调计数器予以保留，Claim 查询无上限门槛故不影响再次领取）、全部 `failed` 分块重置为 `pending` 并清空错误文本、条目 `vector_error` 清空且 `chunks_failed` 归零；返回 202 与更新后条目。非 `partial` 返回 409，不存在/非本人 404。
- 重试语义为增量重试：成功分块不重复写入 Qdrant、不重复调用 Embedding，不产生重复计费。
- 新增 `VectorizeRepository.Requeue`（`vectorize_repo.go`）；`KnowledgeService.RetryVectorize`（`service/knowledge.go`）经 `Vectors VectorizeStore` 装配，`internal/mastra` 包不变。
- 操作日志新增 action `knowledge_vector_retry`（纯字符串枚举，无 schema 迁移），仅重入队列成功后记录；前端 `describeOperationLog` 副标题「重试向量化」。
- 前端 `KnowledgeVectorPanel.vue` 在 `partial` 状态展示「重试向量化」按钮（`knowledge-button-tint` 样式，busy 禁用 + 内联错误文案，不复用弹窗确认——操作安全、无级联删除），成功后 emit `retried`，`KnowledgeDetail.vue` 整体替换 item，面板数据由既有 3s 任务轮询刷新。
- 文档同步：`docs/markdown-knowledge-backend.md`（接口表 + 操作日志与重试语义）、`docs/frontend-structure.md`（向量面板行为）；Swagger 三文件 `swag init` 重新生成。

## 决策与理由

- 只允许 `partial` 重试而非任意失败态：`ready/running/pending` 要么无需重试要么正在处理；全块失败的条目由 worker 以 `error_code=vectorize_failed` 标记，入口仍走「重试导入」旧接口，两个入口语义互斥。
- 分块级重置而非整条重建：向量库按块 upsert，成功块重跑是纯粹的 Embedding 费用浪费。
- Requeue 放在同一事务、遵循既有 worker 锁顺序（任务行→知识行）：避免与正在领取任务的 worker 死锁或竞态覆盖。
- 前端整体替换 item 而非局部补字段：与 `retryKnowledge` 既有接线一致，避免 vector\_\* 字段漂移。

## 教训

- sqlmock 正则匹配 SQL 文本而不是绑定参数：`UPDATE ... WHERE state = ?` 中的 `failed` 是参数，正则写 `state.*failed` 会失配，只能匹配语句结构。
- 组件测试断言请求参数时注意组件实际调用签名：`retryKnowledgeVectorize(props.item.id)` 不传 signal，断言 `toHaveBeenCalledWith(10)` 而非 `(10, undefined)`。
- 后端仓库存在与本次改动无关的 gofmt 未格式化旧文件（`gofmt -l` 整包列出）：只格式化自己改动的文件，不要顺手全量格式化造成无关 diff。

## 候选特性（未实施）

- **变更模型/Embedding 配置后的重建**：改配置导致 embedding_version 变化时，旧向量作废、需全量重建；当前由既有 worker 错误消息提示，不做自动重试，单独立项。

## 验证

- 后端 `go build/vet/test ./...` 全绿；新增用例：`TestVectorRequeueResetsFailedChunksOnly`（任务/分块/条目三处重置、锁顺序）、`TestVectorRequeueRejectsNonPartialTask`（任务非 partial 不重置）、`TestVectorRequeueRejectsConcurrentStateChange`（条目状态并发漂移返回 409）、`TestKnowledgeRetryVectorize`（partial 成功记日志、各非 partial 态 409、内容未 ready 409、not_found/其他用户 404、向量存储未装配 409、Requeue 错误透传）。
- 前端 vitest 204 全绿、typecheck/lint/prettier 通过；新增用例：按钮仅 partial 展示、成功后 emit retried、busy 禁用 + 失败内联错误、`knowledge_vector_retry` 日志映射。
- 未做浏览器真机验证；需要重启后端加载新接口（无 schema 变更）。
