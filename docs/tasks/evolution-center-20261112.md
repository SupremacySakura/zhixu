---
task_id: evolution-center-20261112
status: completed
created: 2026-11-12
last_reviewed: 2026-11-12
applicable_to: "演化中心页面、演化提醒裁决交互、首页聚合卡片、组合式函数与页面测试"
---

# 演化中心页面与首页待处理卡片（第五批 W4.5）

## 背景

前面四单把演化提醒的检测与裁决副作用都做完了（#9 骨架与精确重复、#10 判定契约、#11 过时、#12 冲突与图谱），但用户没有地方看这些提醒：提醒躺在 `evolution_alerts` 表里，只能靠接口自查。本单（GitHub #13）补上最后一环——一个集中处理三类提醒的演化中心页面，以及首页上「有多少条待处理」的入口卡片。

设计口径来自规格 issue #8 与产品设计文档 5.2（演化中心：重复项、冲突项、过时风险及处理状态）与 SRS FR-EVO-05（逐项处理、四种操作留痕、动作只影响目标知识）。约束很明确：单页两页签、不做子路由、不做批量处理与导出、不引图表库，交互沿用图谱页已有的抽屉与筛选组件。

## 结果

- 接口层 `src/services/apis/evolution.ts`：新增 `listEvolutionAlerts`（`page`/`page_size`/`type`/`status`/`open`，空筛选不进查询串）、`getEvolutionSummary`、`resolveEvolutionAlert`（`action` + 可选 `note`、`base_status`，备注 trim 后为空则不带），以及 `EvolutionAlert`/`EvolutionAlertPage`/`EvolutionSummary`/`EvolutionAction` 等类型；沿用 `unwrap` 的 `code !== 0` 抛错约定。
- 组合式函数 `src/composables/useEvolutionAlerts.ts`：页签、类型、状态、页码与列表/总数/摘要/错误状态集中在此。待办页签走 `open=true` 一次取回 pending+deferred；处理记录页签并发查 `resolved` 与 `dismissed` 两路后按 `resolved_at` 倒序合并、总数取两路之和。`resolveAlert` 成功即重拉列表与摘要，失败把服务端文案留在 `resolveError`。所有请求走 AbortController，`onScopeDispose` 中断在途请求。
- 页面 `src/pages/EvolutionCenter.vue`：面包屑 + 标题 + 待处理计数 + 两页签 + 两个 `FilterMenu`（类型三选、状态随页签变化）+ 卡片列表 + `FilePagination` 翻页；右侧详情抽屉复用图谱节点抽屉的 `Motion` + `springSnappy` + `bg-scrim` 结构（`role="dialog"`、`aria-modal`、`knowledge-icon-button` 关闭按钮）。抽屉内并排两栏证据引文（左 `subjects[0]` 本次检查的知识、右 `subjects[1]` 被比对的知识），标题可跳知识详情；备注 textarea 随裁决提交；动作按钮按提醒类型裁剪并带 `data-action`。
- 高亮工具 `src/utils/evolution.ts`：`longestCommonPhrase` 求两侧引文最长公共片段（少于 2 字视为偶发字符），`highlightQuote` 把引文切成普通段与命中段供模板渲染 `<mark>`。
- 入口：`src/router/route.ts` 新增 `/home/evolution`（`meta.requiresAuth`），`HomePage.vue` 的 `NavItems` 在「知识图谱」之后加「演化中心」（`material-symbols:sync-alt-rounded`）。
- 首页 `DashboardPage.vue`：标题块下方新增「待处理提醒」`RouterLink` 卡片，只调 `getEvolutionSummary` 展示 pending+deferred 总数与三类计数，无提醒时显示「暂无待处理提醒」；摘要失败静默退回空态，不影响存储与日志区块。
- 复用：`src/utils/operationLog.ts` 的 `evolutionActionLabel` 加 `export`，页面与操作日志共用同一套动作文案，不再另抄一份。
- 测试：前端新增 30 个用例——接口层 5（查询串拼装、空筛选省略、备注 trim、envelope 报错）、组合式函数 8（待办视野、记录页签合并排序、筛选复位、状态与 open 互斥、裁决成功重拉、裁决失败不重拉、加载失败、scope 销毁中断）、高亮工具 6、演化中心页面 8（按类型裁剪动作、归档禁用态、无修订稿置灰、两种空状态、证据高亮、备注随裁决提交、处理记录展示动作与时间、裁决失败提示）、首页卡片 3（计数与跳转、只调 summary、空态文案）。全量 349 个用例通过，`vue-tsc -b` 无错。

## 决策与理由

- **待办与记录用两个查询口径而不是统一 `status` 枚举**：后端 `open` 已经表达了「待处理 + 暂不处理」这个业务概念，前端再自己拆成两次查询既多打一次请求，也把「暂不处理算不算待办」这条规则复制了一份；记录页签必须拆两路是因为 `status` 只能取单值，而「已处理 + 已忽略」是两个终态。
- **状态筛选有值时绝不传 `open`**：后端 `repository.List` 里 `filter.Open` 优先于 `Status`，两个一起传会静默丢掉状态筛选，用户点了「已处理」却看到待办列表——这是最难排查的一类 bug，所以用互斥拼装并在测试里钉住。
- **换页签时由组合式函数复位状态筛选**：待办页签的选项（待处理/暂不处理）在记录页签里根本不存在，若保留旧值就会用一个后端不认识的状态去查。复位逻辑放在 `watch([tab, type, status])` 里先清值再让下一轮重拉，页面只管切 `tab`。
- **动作按钮直接由前端类型映射表裁剪，不调后端取 AllowedFor**：动作集合是随提醒类型固定的静态规则（后端 `model.EvolutionAction.AllowedFor` 同一份口径），多一次往返换不来灵活性，反而让面板多一个加载态。风险是两边漂移，因此按钮上同时保留服务端 409 文案展示——真漂移了用户能看到原因。
- **证据高亮取「两侧引文最长公共片段」而不是让后端给高亮区间**：后端证据只有 `{knowledge_id, quote}`，没有偏移量；重复与冲突提醒的两侧引文本来就指向同一段撞车文本，前端算公共片段既不需要改契约，也能覆盖「引文两侧表述不同」的情况。
- **归档提醒只留「忽略」与「标记已核实」**：票面明确要求这两个动作之外全部置灰。归档知识已经不在检索与图谱视野里，保留两者/建立矛盾关系/采纳修订都会作用到一条用户已经收起来的知识上；「暂不处理」也一并禁用，否则提醒会一直挂在待办视野里。
- **首页卡片只调 summary，不拉列表**：首页是登录后的第一屏，多一个分页列表请求就多一份首屏延迟；卡片只需要一个总数与三类计数，summary 一次就够，也顺带避免了首页引入提醒卡片渲染逻辑。
- **抽 `src/utils/evolution.ts` 而不是把高亮逻辑写在页面里**：最长公共子串有明确的边界（空串、无重合、多次出现），放在纯函数里能直接单测，页面模板只负责把切好的段落渲染成 `<mark>`。

## 教训

- **`open` 与 `status` 的优先级是后端实现细节，必须在前端钉住**：接口文档只写了「两个参数都支持」，没写谁优先。读 `repository/evolution.go` 才发现 `Open` 为真时直接忽略 `Status`；这类隐式优先级只能靠读实现发现，写接口层时要把「互斥」写成测试断言而不是注释。
- **Vue 模板里不要写多语句内联事件**：`@click="type = ''; status = ''"` 在 prettier 格式化后会被折成多行，编译器直接报 `Unexpected token, expected ","`，整页测试全部无法收集（报错位置还指向格式化后的行号）。多语句一律抽成方法。
- **组合式函数的 watch 里改自己的被监听值要防重入**：`watch([tab, type, status])` 里若在 `tab` 变化时改 `status`，会立刻触发第二轮 watch 再发一次请求；先清值 `return`，让第二轮承担加载，才只有一次有效请求。
- **`EffectScope` 才能测组合式函数的取消语义**：`useKnowledgeGraph` 的测试先例用组件 host 挂载，但取消钩子写在 `onScopeDispose` 里时，`effectScope().run()` + `scope.stop()` 更直接，也不用为了一个断言多挂一个组件。
- **`vi.mock` 的 `mockImplementation` 参数默认不是必填**：`async query => query.status` 在 `strict` 下会报 `'query' is possibly 'undefined'`（接口签名里参数可选），写成 `async (query = {}) =>` 才能过 `vue-tsc`。
- **既有工具函数没 export 就要先补 export 再复用**：`evolutionActionLabel` 一直是模块内私有，页面若抄一份文案，八种动作的中文就会有两处真相；补一个 `export` 比复制便宜得多，也顺带让操作日志与页面口径强制一致。
