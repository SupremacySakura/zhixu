---
task_id: evolution-conflict-20261112
status: completed
created: 2026-11-12
last_reviewed: 2026-11-12
applicable_to: "知识演化、冲突识别、修订稿采纳、矛盾关系类型、版本来源语义、图谱边渲染"
---

# 冲突识别、修订稿采纳与矛盾关系（第五批 W4.4）

## 背景

上一单（GitHub #11）把过时结论接进了 `EvolveProcessor`，但冲突结论仍被一行 `if conflict { continue }` 挡在门外；同时 `accept_revision` 与 `create_relation` 两个裁决动作在 `effectFor` 里直接返回 `ErrEvolutionUnsupported`（409）。本单（GitHub #12）把这两半补齐：模型在一次演化判定调用里同时产出冲突结论与修订稿，Go 侧落成提醒，用户采纳后写新版本、或建立一条矛盾关系；前端图谱要能把这些矛盾边画出来。

设计口径已固定在规格 issue #8 与 ADR-0002：`contradicts` 是第六种关系类型（对称），只允许用户手动或冲突提醒建立，AI 关联推荐必须显式排除它。

## 结果

- **关系类型**：`internal/model/relation.go` 新增 `RelationTypeContradicts = "contradicts"`，`Valid()` 六值，`Symmetric()` 增加该类型（`related || contrasts || contradicts`），写入前仍由 `normalizeRelationPair` 按 min/max id 归一，唯一键不变。
- **AI 推荐白名单**：新增 `RelationType.Recommendable()`（五值 true、`contradicts` false），`internal/service/relate.go` 与 `internal/service/relation_task.go` 两处清洗从 `Valid()` 改为 `Recommendable()`。`Valid()` 是「所有合法类型」，直接当推荐白名单用会让 `/relate` 自由产出矛盾关系。
- **冲突提醒接入管线**：`internal/service/evolve_task.go` 删除 `buildAlerts` 里跳过 conflict 的分支；相似度分派沿用 #11 的 `evolutionAllowedTypes`（≥0.90 只收 duplicate、≥0.75 收 conflict、≥0.80 收 outdated），同一对同时给出重复与冲突时按 `evolveTypePriority` 只留重复。
- **修订稿门槛**：同一次模型调用产出 `revised_title`/`revised_content`，存入提醒载荷。门槛守两道——客户端清洗 `sanitizeEvolveJudgements` 挡住超长/缺半份的模型输出（超 20000 字整份丢弃而不是截断），处理器 `buildAlert` 用新增的 `evolveRevisionUsable` 再判一次（防止绕过客户端的调用路径）。任一处判定不可用时修订稿置空、严重度降级为 `review`，`SuggestedAction` 随之从 `accept_revision` 改回 `create_relation`（`evolveSuggestedAction` 签名从 `EvolveJudgement` 改为 `EvolutionPayload`，判的是「载荷里到底有没有修订稿」）。
- **采纳修订**：`KnowledgeService.AcceptRevision(ctx, userID, id, title, content)` 校验标题/正文、取当前版本作 `baseVersion`、以 `change_source=ai` 走既有 `KnowledgeStore.Update` 版本链路，记 `knowledge_update` 操作日志（detail 含 `change_source: "ai"`）。`EvolutionService` 新增可选依赖 `Reviser EvolutionReviser`，`effectFor` 的 `accept_revision` 解析载荷、修订稿为空时返回 `ErrEvolutionInvalid`（handler 映射 400），否则调 `AcceptRevision`；未配置 `Reviser` 时仍返回 `ErrEvolutionUnsupported`。
- **建立矛盾关系**：`EvolutionService` 新增可选依赖 `Relations EvolutionRelationWriter`（签名同 `RelationService.Create`），`effectFor` 的 `create_relation` 以 `subject_ids[0]` 为源、`[1]` 为目标写 `contradicts`，状态 `confirmed`、来源 `manual`；同对已存在时由仓储 `findPair`/`confirm` 复用，不重复插入。
- **版本来源语义**：`knowledge_items` 新增 `change_source`(manual/ai) 记录**当前正文**的产生来源，`KnowledgeStore.Update` 增加 `changeSource` 参数；版本快照行记录**被替换掉的旧正文**的来源（`item.ChangeSource`，空则 manual），条目列写新来源；`Versions()`/`VersionDetail()` 的「当前」条目改读条目列（此前硬编码 manual）。
- **图谱与前端**：后端 `service/graph.go` 与 `repository/graph.go` 原样透出 `relation_type`、不按类型过滤，无需改动，只补「图谱返回矛盾边」测试。前端 `utils/relation.ts` 加 `contradicts: '矛盾'` 标签与选项（仍是唯一中文文案源），`services/apis/knowledge.ts` 的 `RelationType` 联合类型加该值；`utils/knowledgeGraph.ts` 新增 `edgeContradicts`（读 `--color-danger`），`graphEdgeColor` 改为「矛盾类型优先，其余按来源分色」；`KnowledgeGraphChart.vue` 传 `relation_type`，`KnowledgeGraph.vue` 在图中真的出现矛盾边时补一条图例项。
- **测试**：仓库层 `TestNormalizeRelationPair` 加 contradicts 行、`TestCreateContradictsRelationReusesExistingRow`（幂等复用）、`TestKnowledgeUpdateTransactional` 加「条目与快照各记各的来源」子用例；服务层 `TestKnowledgeAcceptRevision`（新版本 + ai 来源 + 原文入快照 + 日志）、`TestKnowledgeVersions` 加「当前条目保留自己的来源」、`TestEvolutionResolveAcceptRevision*` 与 `TestEvolutionResolveCreateRelation*` 共 5 个（含能力缺失与无修订稿 400）、`TestRelateClientDropsContradictsSuggestion`、`TestSanitizeEvolveJudgementsKeepsCompleteRevisionOnly` 三分支、`TestEvolveProcessor` 新增冲突落库/证据不全丢弃修订稿/超长丢弃修订稿/重复优先四个、`TestGraphIncludesContradictsEdges`；handler 层 `TestEvolutionHandlerConflictActions` 三子用例；前端新增 `relation.test.ts` 并扩展图谱图例与边色用例。后端 `go build`/`go vet`/`go test ./...` 全绿；前端 `npm test` 与 `npm run typecheck` 全绿（本单范围内 47 文件 324 用例，合并 #13 后 52 文件 354 用例）。

## 决策与理由

- **新增 `knowledge_items.change_source` 而不是只靠快照行**：规格要求「采纳修订写为新版本，变更来源为 ai」。若只把 ai 记在快照行上，它记录的是**被替换掉的旧正文**由谁产生，采纳动作本身反而无处可查，版本列表的「当前」条目也只能继续硬编码 manual。分成「条目记当前来源、快照记旧来源」后，两个问题一起解决，且不需要额外的历史表。
- **推荐白名单独立于 `Valid()`**：`Valid()` 的语义是「这个值能不能进数据库」，`Recommendable()` 的语义是「模型有没有资格建议它」。ADR-0002 明确要求 AI 白名单排除 `contradicts`，把两个语义塞进一个方法，将来加第七种类型时必然有人只改一边。
- **修订稿门槛守两道**：客户端清洗挡的是模型输出（长度、半份），处理器挡的是任何绕过客户端的调用路径。只守一道时，处理器被单测或别的调用方直接喂判定，超长修订稿会落进载荷并最终写进正文。
- **`SuggestedAction` 在丢弃修订稿之后再算**：先算再丢弃会推荐 `accept_revision`，用户点下去只拿到 400；建议动作必须跟着载荷的真实内容走。
- **`create_relation` 不额外写去重逻辑**：仓储 `Create` 已经在同对同类型命中时复用行（`confirm`），服务层再判一次只会多一次查询、并可能与仓储的 `rejected`/`suggested` 复用语义打架。
- **矛盾边按类型分色而不是按来源分色**：既有边色只区分 AI/手动来源，但矛盾在图上属于「必须一眼看见」的类型；用 `--color-danger` 系让它在同一张图里跳出来。图例项仅在图中真有矛盾边时显示，避免给不存在的样式留说明。

## 教训

- **`gofmt -l` 的既有噪声要先认领**：仓库里有一批历史未格式化文件（`internal/database/db.go`、`internal/middleware/*`、`internal/repository/*_repo.go` 等）。验收口径是「你改的文件无输出」，不是「全仓库无输出」；顺手格式化会制造与任务无关的 diff。
- **接口签名加参数要一次数清 fake**：`KnowledgeStore.Update` 加 `changeSource` 后，除仓储实现外还有 service 层 fake、handler 层两个 fake 共三处要同步；漏一处编译能过（fake 少方法会报错，但 handler 层用的是内嵌接口，只实现部分方法时会在运行到该路径才炸）。
- **fake 的 `applyCalls` 不能用来断言「动作被拒」**：`evolutionStoreFake.Resolve` 在调用 `apply` 前就自增了计数，动作被拒时计数照样加一。要断言「状态没变」必须看 `alert.Status`，或者让 fake 更贴近真实仓储的调用顺序。
- **对称类型的幂等断言要落在「只有一行」上**：`contradicts` 归一后 A→B 与 B→A 是同一对，测试如果只断言「返回了 confirmed 行」，插入两行的实现也能过；必须断言仓储被调用一次且返回的是既有行。
- **前端图例测试用文本包含断言更稳**：`graph-legend` 的用例断言的是「文本包含某几项」，新增一项不会打破它；但如果新图例项是条件渲染，还要补一条「条件不成立时不显示」的用例，否则 `v-if` 写反了没人发现。
