---
task_id: evolution-outdated-20261112
status: completed
created: 2026-11-12
last_reviewed: 2026-11-12
applicable_to: "知识演化、过时识别、演化提醒裁决副作用、详情页过时标记"
---

# 过时识别与详情页过时标记（第五批 W4.3）

## 背景

上一单（GitHub #10）把 AI 服务的演化判定契约立了起来，但 Go 侧只消费 `duplicate` 一种结论：`outdated` 与 `conflict` 过了分派校验就被丢弃。本单（GitHub #11）接上过时这一路——用户导入一份新版资料、而它明确替代某条旧知识时，旧知识要能被标为「可能过时」并带上判定理由，详情页要能看见这个标记和它为什么成立，同时旧知识仍然可检索、可阅读。

设计口径已固定在规格 issue #8：过时只做跨条目判定（同一 `knowledge_id` 的不同版本之间永不提醒），弱信号必须有明确证据才落库，标记落在 `knowledge_items` 的两列上、不碰处理状态与归档时间。

## 结果

- 检测侧放行过时结论：`EvolveProcessor.buildAlerts` 由「只接受 duplicate」改为「挡掉 conflict、放行 outdated」，冲突落库留给 #12。
- 相似度分派放宽：`evolutionAllowedTypes` 由「≥0.90 只 duplicate、≥0.75 只 conflict」改为累加式——≥0.90 加 duplicate、≥0.75 加 conflict、≥0.80 加 outdated，因此 0.80~0.90 段同时接受冲突与过时两类结论；`updates` 关系来源的强候选仍只接受 outdated。
- 弱信号过时门槛：`buildAlert` 把证据校验拆成触发侧与候选侧分别定位，向量近邻来源的过时结论要求两侧各至少一条可定位引文，否则整条不落库；强信号（`updates` 关系）单侧即可。
- 裁决副作用：`EvolutionService.effectFor` 从「mark_outdated 未启用」改为真实写入——目标取 `subject_ids[1]`（被比对的那条旧知识），时间取当前时刻，理由取提醒 payload 的 `reason`（为空时回退「已被更新的一条知识取代。」）；`mark_verified` 仅在提醒类型为 outdated 时清除标记，`Outdated` 依赖未注入时核实本身仍成功。
- 数据写入：`KnowledgeRepository.SetOutdated` 先按 `id + user_id` 确认归属再更新两列；`KnowledgeService.SetOutdated` 做幂等判断（同一状态不重复写、不重复记日志），写入记 `knowledge_outdated`、清除记 `knowledge_verified`。
- 编辑正文清标记：`KnowledgeRepository.Update` 在更新正文的同一个 `Updates` 里把 `outdated_at` 置空、`outdated_reason` 清空，返回结构体同步置空——标记针对被判定时的正文，用户改写即视为自行处理。
- 详情接口与前端：知识详情本来就整条返回 `KnowledgeItem`，`outdated_at`/`outdated_reason` 已有 json tag，无需改 handler；前端 `KnowledgeItem` 类型补两字段，详情页在归档警告条同位置新增过时横幅（标题 + 理由 + 判定时间 + 一句「仍可检索、改写或核实后消失」），header 按钮组新增「重新检查」（`POST /api/knowledge/:id/evolve/regenerate`，提交后给一句状态提示，活跃任务由后端复用）。
- 操作日志前端映射：`describeOperationLog` 补 `knowledge_web_import`、`knowledge_organize_accept`、`knowledge_outdated`、`knowledge_verified`、`evolution_resolve` 五个分支（`evolution_resolve` 按 detail 里的 action 再映射到八种中文文案），`OperationLogAction` 联合类型与 `OperationLogTargetType` 同步补齐。
- 测试：后端新增 12 个用例（仓储 `SetOutdated` 三条、服务 `SetOutdated` 幂等与日志、裁决侧 mark_outdated 落 `subject_ids[1]`、mark_verified 只清过时提醒、依赖缺失时的降级、检测侧强候选过时落库、弱信号单侧证据不落库、弱信号双侧证据落库、自身版本不成候选）；前端新增 6 个用例（过时横幅展示、无标记时不展示、重新检查提交与提示、三个新日志动作映射、未知动作回退）。后端 `go build`/`go vet`/`go test ./...` 全绿，前端 `npm test` 319 pass 与 `vue-tsc -b` 无错。

## 决策与理由

- **过时落在 `subject_ids[1]` 而不是 [0]**：`subject_ids` 的口径是「[0] 本次触发知识、[1] 被比对知识」，过时的语义是「旧的那条被新的取代」，标记必须落在被比对的那条上；落到 [0] 会把刚导入的新知识标成过时。
- **弱信号要求双侧证据、强信号只要单侧**：`updates` 关系是用户显式声明过的「这条取代了那条」，模型只需要说清取代理由；向量近邻只是「正文像」，模型很容易在无关文本上编出理由，必须两侧都能在原文里指出来才可信。
- **mark_verified 只清过时标记**：「已核实」在重复提醒上的含义是「我看过了，这两条不是重复」，与过时标记无关；不按提醒类型收窄，核实一条重复提醒会顺带清掉另一条判定留下的过时标记。
- **编辑正文清标记而不是保留**：标记带着「这条知识的正文已被后续资料取代」的判断，用户改写正文后该判断不再成立；保留会让横幅长期挂在一条已经更新过的知识上，比不显示更糟。
- **新增两个知识侧操作日志动作而不是复用 `evolution_resolve`**：裁决日志的目标是提醒（`target_type=evolution`），点击跳转到演化中心；标记过时改的是知识本身，日志要落在知识上才能从操作日志直接跳回那条知识。
- **详情页只给提示不轮询任务**：演化检查是异步任务，前端若轮询就要再引一套任务状态机；当前只需让用户知道「已提交」，结果在演化中心（#13）统一看，详情页保持简单。

## 教训

- **放宽分派规则会连带改既有断言**：`evolutionAllowedTypes` 从「分段互斥」改成「累加」后，原本断言「强候选只接受过时结论、不落任何提醒」的测试立刻失败——这是设计变了而不是代码坏了，应当先确认分派口径再改断言，而不是把断言放宽了事。
- **`append` 到已有切片会污染源切片**：证据合并原本写 `append(validEvidence(trigger...), candidate...)`，当触发侧切片还有容量时，候选侧证据会写进触发侧底层数组，后面再读触发侧证据就串了；先各自算出再 `append` 到新切片才安全。
- **`ListByIDs` 这类精简 Select 容易漏新列**：`views` 里用 `item.OutdatedAt != nil` 判断过时，但 `ListByIDs` 的 Select 没有 `outdated_at`，导致列表里的过时标记永远是 false；加了列必须回头检查所有精简查询。
- **Go 测试里的 sqlmock 正则需要转义两层**：`run_code` 的模板字符串会把 `\?` 吃掉一层，写进文件的 `?` 不再是字面量，SQL 匹配失败；正则里的 `\?` 在字符串里要写成四层反斜杠，或者干脆用字符串数组 join 拼文件。
- **前端类型联合漏项是静默的**：`OperationLogAction` 只到 `knowledge_unarchive`，但后端早已有 `knowledge_web_import`、`knowledge_organize_accept`、`evolution_resolve`，前端拿到这些动作会落到 default 问号图标，类型检查也不会报错（因为后端返回的是字符串）。补动作时要连联合类型一起补。
