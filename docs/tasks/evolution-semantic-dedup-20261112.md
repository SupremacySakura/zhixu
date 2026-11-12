---
task_id: evolution-semantic-dedup-20261112
status: completed
created: 2026-11-12
last_reviewed: 2026-11-12
applicable_to: "知识演化、语义近重复、Go→AI 同步判定契约、模型结论分派与证据校验"
---

# 语义近重复检测与演化判定契约（第五批 W4.2 第二部分）

## 背景

上一单（GitHub #9）只铺了骨架：提醒表、正文指纹、evolve 任务与读取/裁决接口，重复检测只覆盖「正文完全相同」这一种确定性情形。本单（GitHub #10）补模型这一路：措辞不同但语义相同的两条知识也要能判为重复，同时把 AI 服务的演化判定接口立起来，为后续过时（#11）与冲突（#12）共用同一次调用。

设计口径已固定在规格 issue #8：一次模型调用同时判三类，Go 按向量相似度分派允许的结论类型，证据引文必须能在正文里定位。

## 结果

- AI 服务新增 `POST /evolve`（`apps/zhixu-ai/src/mastra/evolve.ts` + `src/lib/evolve.ts`）：与 `/relate` 同构——Bearer 鉴权、`.strict()` 结构校验失败 400、模型地址校验失败 403、120 秒超时 504、模型输出结构非法 422、其余 502。请求体含模型配置、任务与知识标识、触发知识标题与 ≤2000 字摘录、最多 8 个候选（各含 ID、标题、≤1200 字摘录、相似度、来源标签 `update_relation`/`vector_neighbor`）；响应为 `judgements[]`，每条含目标知识 ID、类型、理由、1~4 条证据（知识 ID + ≤200 字引文）、严重度，冲突时另带 `revised_title`/`revised_content`。
- AI 侧清洗 `sanitizeJudgements`：引文截断到 200 字、理由截断到 200 字、修订稿必须标题与正文都非空且正文 ≤20000 字否则整份丢弃、按 `target:type` 去重、最多 5 条；输入非空但全部被清洗掉时返回 null（走 422 而不是伪装成「没有重复」）。
- Go 侧新增 `internal/service/evolve_client.go`：`EvolveClient.Evolve` 同步调用（125s 总超时、120s 响应头超时、禁代理与跳转、1MB 响应上限），状态码映射与 `RelateClient` 一致；`sanitizeEvolveJudgements` 丢弃未知类型、目标不在候选批次、空理由、证据知识 ID 既不是触发知识也不是该目标的结论；`evolutionAllowedTypes(origin, score)` 做分派：来源是更新关系的强候选只允许 `outdated`，否则相似度 ≥0.90 只允许 `duplicate`、≥0.75 只允许 `conflict`、更低则不允许任何结论。
- `EvolveProcessor` 扩展：指纹未命中后走「候选生成 → 一次模型调用 → 分派校验 → 落库」。候选合并两类来源——`updates` 关系指向的目标（强信号，`score` 记 0）与向量相似度 ≥ `EvolveOutdatedMinScore`(0.80) 的近邻（弱信号），强候选排前、其余按相似度降序，合计取前 `EvolveCandidateMax`(8)；向量召回 `EvolveVectorTopK`(30)。
- 只认「本条更新了对方」的方向：`ListByKnowledge` 返回双向关系，反向的 `updates`（本条是被更新的旧文）不算强候选，否则会产出方向相反的提醒。
- 本阶段只消费 `duplicate`：`outdated`/`conflict` 结论通过分派校验后直接跳过，留给 #11/#12 在同一处理器里接上；这样模型一次调用就能覆盖三类，不必为同一份正文付两遍钱。
- 证据校验沿用 #9 的 `validEvidence`：按 `knowledge_id` 拆成触发侧与候选侧，各自子串定位；定位不到的剔除，全部剔除则整条不落库，部分缺失则严重度从 `strong` 降级为 `review`。修订稿只在「证据全部可定位且类型为冲突」时保留。
- 同一对知识只留一条提醒（按 `evolveTypePriority` duplicate→conflict→outdated 稳定排序后去重），单任务最多落 5 条（`EvolveAlertMaxPerTask`）。
- `cmd/server/main.go` 补全装配：`EvolveProcessor` 注入 `chat.Service`（对话与 Embedding 能力）、`relationRepo`、`MastraEmbedder`、`qdrant` 与 `EvolveClient`。
- 测试：AI 侧 `src/tests/evolve.test.mjs` 10 个用例（schema 19 种非法输入、三类 item、清洗纯函数、修订稿门槛、prompt 内容、路由 403/400/403）；Go 侧 `evolve_task_test.go` 重写为 20 个用例（精确重复不调模型、语义重复落库、分数分派、强候选只收过时、反方向忽略、证据校验三种结果、同目标去重、候选上限、归档跳过、版本栅栏、错误映射、前置守卫、能力缺失）。后端 `go build`/`go vet`/`go test ./...` 全绿，AI 服务 `npm test` 61 pass 与 `tsc --noEmit` 无错。

## 决策与理由

- **一次调用判三类而不是每类一次调用**：三类判定读的是同一份触发正文与同一批候选，拆成三次会把候选摘录重复送给模型三遍；分派放在 Go 侧按相似度做，模型只负责「说它看到了什么」，成本与可控性都更好。
- **分派规则挂在候选来源上而不是只挂分数**：`updates` 关系是用户确认过的强信号，此时「两条相似」的正确答案是「新的取代了旧的」而不是「两条重复」，所以强候选只接受 `outdated`；若只按分数分派，用户手工确认的更新关系会被判成重复并建议归档。
- **低分候选不进批次而不是进批次后被丢弃**：低于冲突阈值（0.75）的候选几乎不可能给出有意义的结论，送进去只会让模型在无关文本上编造理由；直接不送，同时省下 token。
- **AI 侧清洗返回 null 而非空数组**：空数组会让 Go 以为「模型确认没有重复」，把「模型胡言乱语」伪装成「检测通过」；返回 null 触发 422，任务以终态失败收场，用户能看到「输出格式不符合要求」而不是静默零提醒。
- **`strong` 只在证据齐全时保留**：严重度是给用户判断「要不要立刻处理」的信号，证据定位不到一半却标 `strong`，等于让用户相信一个自己无法验证的结论。
- **修订稿与证据校验绑定**：修订稿是模型基于引文改写出来的，引文本身定位不到时，改写内容没有可核对的事实基础，保留它比丢弃更危险。

## 教训

- **测试里的引文要各自落在自己那一侧的正文里**：`validEvidence` 是双侧独立校验的，用同一句引文同时代表触发知识与候选知识，只有一侧能定位，严重度会被按设计降级，测试就会在「期望 strong 实得 review」上失败——这是测试写错而不是代码错，第一反应应当是回看校验规则。
- **在 `run_code` 的模板字符串里写 Go/TS 代码要转义反斜杠**：`strings.Join(lines, "\n")` 里的 `\n` 会被当成真实换行写进文件，TS 文件因此报 `ERR_INVALID_TYPESCRIPT_SYNTAX`；写代码片段时一律用双反斜杠。
- **反向关系容易漏**：`ListByKnowledge` 按 `source_id OR target_id` 返回，拿到 `updates` 关系后必须再判方向；不判方向时，任何「被更新的旧文」都会把更新它的新文当成强候选，产出的提醒方向完全相反。
- **同一份能力注入两个接口要显式**：`chat.Service` 同时实现了对话与 Embedding 解析，装配时两个字段都要给，只给一个会在运行到 Embedding 时才报「未配置可用的 Embedding 模型」，编译期发现不了。
