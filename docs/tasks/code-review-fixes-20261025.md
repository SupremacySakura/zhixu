---
task_id: code-review-fixes-20261025
status: completed
created: 2026-10-25
last_reviewed: 2026-10-25
applicable_to: "暂存区 CR-001 至 CR-011 修复、会话历史、向量索引与删除一致性"
---

# 暂存区审查修复

## 结果与约束

- CR-001：plain/rag 与管理路由共用 INTERNAL_API_SECRET，验证先于解析、存储和模型调用；Go 发送内部 Bearer，未配置拒绝服务。
- CR-002：新集合使用 `zhixu_u{user}_v{embeddingVersion}`。历史任务保留原集合，检索兼容数据库记录的同 embeddingVersion legacy 集合，并按 score 合并去重。仅修改聊天配置不再切换向量集合，不自动全量付费重建。
- CR-003：历史加载 AbortController + 当前请求身份 guard；重置、发送、卸载使旧请求失效；页面加载中禁止提交。
- CR-004：Mastra done 回传本轮 response.dbMessages 的唯一 assistant ID，并验证其确实已落库且 thread/resource 匹配。Go 在透传 done 前保存该 MessageID；详情只按精确 ID 关联，旧记录或未知 ID 不按序猜测。Query 列扩为 512。
- CR-005：缺模型/凭据的任务保持 pending，30 秒后可再次调度；历史缺配置 partial 仅在 model_id=0、collection 为空且旧错误文本匹配时安全恢复。其他付费失败不自动重试。付费前持久标记不确定批次，崩溃恢复不重复模型调用。
- CR-006：删除知识事务同时保存 VectorCleanupTask；upsert 与删除按 task → knowledge 行锁串行。后台按 user/knowledge filter 清理 Qdrant，失败重试，成功后每 5 分钟再检查，以防远端超时请求迟到。检索扩大候选补足仍有效来源，每集合最多 320 个。
- CR-007：ChatConversationState 复合主键作为持久锁点，删除先提交 tombstone 并清来源，再删除远端；404 幂等，失败保留 pending，启动/后台重试。来源保存同锁拒绝 tombstone；列表/详情/改名/续聊屏蔽删除过的 ID。AI 进程内删除 abort 并等待活动流结束后删除存储。
- CR-008：列表/详情按每页 100 累积历史，不再依赖默认分页；最多 10000 条，超限明确报错而非静默截断。Memory 模型上下文仍为最近 40 条，区别于管理页显示历史。
- CR-009：管理路由通过 `await chatStore.getStore("memory")` 初始化存储，而不是直接访问未初始化的 domain。
- CR-010：本地 assistant 记录原轮 rag 模式，重试使用原轮而不是当前开关；历史有 query/sources 时推断 RAG。
- CR-011：修正 API 文件格式，前端 lint 通过。
- 新表和新增列由现有 AutoMigrate 接线；清理 worker 共用应用生命周期 context。保留用户原暂存区，修复只在工作树，不执行 git add/commit。

## 验证范围

- 前端单 worker 全量 171/171 通过，typecheck 与 lint 通过。首轮多任务并行有 2 个既有测试 5 秒超时，单 worker 重跑通过。
- Go 最终 `go build ./...`、`go test ./...` 和 service/repository/handler 三包 `go test -race` 均通过。标准缓存访问曾被 sandbox 拒绝，按同一命令审批重试后退出 0。
- AI 最终 typecheck 与 12/12 fake 存储/SDK 流回归通过，覆盖认证、完整分页、初始化、同轮 ID 和在途删除，不调用付费模型。
- 本次未使用真实 MySQL/Qdrant/模型做端到端验证，部署前应验证迁移及三服务协作。

## 边界及文档差异

- Go tombstone 持久保护来源和用户视图；AI 活动流取消屏障仅进程内，不能取消另一 AI 实例/重启前的远端流。多实例旧流可能在删除后物理重建 Mastra 消息，但 Go 不再展示或允许续聊该 ID。要保证跨实例物理删除需分布式持久 fence，不能把进程内 barrier 宣称成此保证。
- Qdrant cleanup tombstone 暂永久保留、成功周期性删除，后续需制定安全 GC；候选超过 320 上限时可能不足 topK。
- 旧来源没有 MessageID 不可可靠恢复，故不附引用；缺 query/sources 的历史不能恢复不可见的原始 rag 模式。
- `docs/tasks/chat-history-20261024.md` 中按序配对及直接 `stores.memory` 的描述已被此修复替代；旧向量任务文档中的 ConfigVersion 集合命名仅适用于历史集合。代码为准，保留旧任务记录作为历史而不改写事实。
- Brain 的旧 cloud-drive 路径及无自动化测试描述与当前 zhixu/Vitest 不一致；Brain/skills 对 AI 只读，未修改，需人工审核更新。
