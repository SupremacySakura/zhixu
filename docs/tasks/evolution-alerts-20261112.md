---
task_id: evolution-alerts-20261112
status: completed
created: 2026-11-12
last_reviewed: 2026-11-12
applicable_to: "知识演化、重复检测、content_hash 指纹、ai_tasks evolve 任务、演化提醒裁决"
---

# 演化提醒骨架与精确重复检测（第五批 W4.2 第一部分）

## 背景

第五批（P4 剩余 W4.2~W4.5）拆成五张实施单，本单是其中唯一无阻塞的第一张（GitHub #9）：先把「提醒表 + evolve 任务 + 正文指纹」这条骨架铺好，后续语义近重复、过时、冲突与演化中心页都挂在同一张表与同一个处理器上。

设计阶段已把口径固定在规格 issue #8：提醒独立成表、指纹终态不清空、正文指纹统一走 Go 规范化、Go→AI 单向同步调用（不做回调）。

## 结果

- 数据模型：`evolution_alerts`（user_id、alert_type: duplicate/outdated/conflict、status: pending/resolved/dismissed/deferred、severity: review/strong、subject_ids 有序 JSON、payload JSON、dedupe_key char(64) 唯一索引、resolved_at/resolved_action/resolution_note、时间戳）；`knowledge_items` 新增 `content_hash`（char(64)，带索引）、`outdated_at`、`outdated_reason`。
- 正文指纹：`utils.NormalizeKnowledgeContent`（去 BOM、CRLF/CR→LF、逐行去行尾空白、整体 trim）+ `KnowledgeContentHash`（sha256 hex，空正文返回空串）。四条写入路径统一计算：手动创建/网页导入（`buildInlineKnowledge`）、文件导入（仓储 `Finish` 写正文时同批写入）、编辑（仓储 `Update`）、历史数据回填（`backfillKnowledgeContentHashes`，游标分页每批 500 条、只处理空值行）。
- 任务骨架：`EvolveProcessor` 注册在既有 ai_tasks worker 上，幂等键 `evolve:<knowledgeID>`；向量化成功即入队（与 relation 的「只跑一次」策略不同，编辑正文后要重新判定）；载荷带 `base_version`，处理时版本不一致直接终态失败「知识已更新，本次检查作废」。
- 精确重复：`FindByContentHash` 限定同用户、未归档、非自身，取 ID 最小一条；命中即落一条 `duplicate` 提醒，理由、证据引文（双方正文开头 200 字）、建议动作 `archive_one` 一并写进 payload；证据经 `validEvidence` 双侧子串校验，全部定位不到则不落库。
- 提醒指纹：`EvolutionDedupeKey` = sha256(类型 + 排序后 subject_ids + 证据引文指纹)；仓储 `Upsert` 按指纹幂等：pending 只刷新时间、deferred 复活回 pending、resolved/dismissed 抑制不再产生，并发写撞唯一索引时回读按刷新处理。
- 接口：`GET /evolution/alerts`（类型/状态/open 筛选、page 1..1000000、page_size 1..100、created_at DESC，返回涉及知识标题与归档/过时标记）、`GET /evolution/alerts/summary`（一条分组查询）、`POST /evolution/alerts/:id/resolve`（六动作 + defer，按类型裁剪，非法组合 400，已处理再提交 200 不写日志，期望状态不符 409）、`POST /knowledge/:id/evolve/regenerate`（202 返回任务视图）。
- 裁决副作用：`archive_one` 归档 ID 较大的一条（保留先入库的原件），其余五个动作的副作用留给后续单；裁决写提醒行 + 操作日志新动作 `evolution_resolve`，目标类型新增 `evolution`。
- 级联：知识删除时同事务按 `JSON_OVERLAPS(subject_ids, ?)` 删提醒；归档只保留提醒不删。
- 测试：新增 6 个测试文件（模型/仓储/服务/处理器/handler/router/utils），后端 `go build`、`go vet`、`go test ./...` 全绿。

## 决策与理由

- **content_hash 必须由 Go 算**：MySQL 的 `SHA2(content,256)` 与 Go 规范化结果对不上，历史回填若走 SQL 会让全部旧知识漏判重复。回填只填空值行且按 id 游标分页，避免大库启动时一次性读全部正文。
- **提醒指纹终态不清空**：与 `ai_tasks.dedupe_key`（终态置 NULL）相反。任务键的目的是「允许重跑」，提醒键的目的是「处理过的不再弹」，清空就失去抑制能力。
- **精确重复不拦截创建**：拦截会让「从网页导入一篇已在库的文章」变成硬错误；改为只产提醒，由用户裁决归档其一。
- **「归档其一」按 ID 大小而非触发方向选受害者**：重复检查可能从任一侧发起，按方向选会让同一对知识在不同触发顺序下归档不同条目。
- **版本栅栏而非比对正文**：入队时写 `base_version`，处理时版本不一致即作废。比对正文需要额外读一次历史版本，且编辑回滚到相同正文时语义更含糊。

## 教训

- **仓储测试用 sqlmock 时，GORM 的参数展开要按实际占位符数写**：`status IN ?` 会被展开成两个占位符；`LIMIT` 的 offset 为 0 时不占参数位，多给一个就报 `expected 4, but got 3`。正则里 `count(*)` 的 `*` 会修饰 `(`，要么转义要么用反引号拼字符串。
- **改级联删除会打破既有 sqlmock 用例**：`DeleteKnowledgeRows` 新增一步删除后，`knowledge_repo_test.go`、`file_test.go`、`operation_log_test.go` 里逐条断言 DELETE 顺序的用例全部要补期望，否则是「测试红」而不是「代码错」。
- **handler 层 fake 返回 `gorm.ErrRecordNotFound` 会走 500**：service 层把 not found 包成领域错误，handler 按领域错误映射状态码；fake 要返回 `service.ErrKnowledgeNotFound` 才能覆盖 404 分支。
- **操作日志的目标类型要跟着实体走**：把提醒 ID 挂在 `knowledge` 目标类型下，前端按 ID 打开会跳到错误的知识；新增 `evolution` 目标类型比复用更安全。
- **沙箱里默认 GOCACHE 不可写**：必须显式 `GOCACHE=$PWD/.gocache GOTMPDIR=$PWD/.gotmp`（先 mkdir -p），否则 go 命令报 operation not permitted。
