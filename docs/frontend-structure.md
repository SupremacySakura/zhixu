# 前端结构说明

## 来源

- `apps/zhixu-frontend/AGENTS.md`（入口、路由、状态、接口约定）。
- 根目录 `AGENTS.md`（前后端联调关系、通用命令约定）。

## 适用范围

- 适用于 `apps/zhixu-frontend` 目录结构、页面路由、状态管理与请求层协作。

## 更新入口

- 当页面结构、路由配置、鉴权流程、请求拦截器或前端构建命令变更时更新本文。
- 接口契约变化需同时检查 `backend-structure.md` 与 `brain/project.md` 的对应描述。

## 目录结构与职责

- 启动配置与端口约定见根 [README](../README.md#两种启动方式)；本地 Vite 使用 `DEV_FRONTEND_PORT`，代理通过 `DEV_BACKEND_URL` 跟随后端端口，启用 `strictPort` 防止静默换端口。

- `src/main.ts`：应用启动入口，注册 Pinia、路由及持久化插件。
- `src/router/`：路由定义与导航守卫。
- `src/pages/`：页面级组件（登录、首页、文件管理、上传、分享等）。
- `src/components/file-management/`：文件管理页的展示组件，包括双视图、工具栏、分页、操作菜单、弹窗与上传面板；通过 typed props/emits 与页面编排层通信。
- `src/composables/`：页面级业务状态与副作用生命周期。文件管理域分别维护列表、文件操作、预览分享、上传队列和通知，避免页面与多个 Store 重复持有同一状态。
- `src/stores/`：Pinia 状态管理，当前以用户登录态为核心。
- `src/services/apis/`：按业务域拆分 API 封装。
- `src/services/request.ts`：Axios 实例与请求/响应拦截器。

## 路由与页面映射

- 公共路由：`/login`、`/register`、`/require-login`。
- 受保护主路由：`/home` 及其多数子路由。
- 特例：`/home/pickup-codes` 为免登录访问路由。
- 鉴权守卫在进入受保护页面前调用登录态检查接口，失败后重定向到登录提示页。

## 状态与请求层分工

- Pinia（`src/stores/user.ts`）负责保存用户状态与 token，并持久化到本地存储；登录用户资料（`profile`）随登录态一并持久化，退出登录时清空。
- 主框架 `src/pages/HomePage.vue` 挂载时静默拉取 `GET /api/auth/profile` 刷新用户资料；侧边栏 `src/components/bussiness/SideBar.vue` 底部展示当前用户（头像缺省时用用户名首字符占位）并提供退出登录入口，退出前经 `ConfirmDialog` 二次确认，确认后清空登录态并跳转 `/login`。
- 请求层（`src/services/request.ts`）负责统一注入 `Authorization` 请求头与处理通用响应逻辑。
- 业务 API 文件只关心参数与返回结构，不重复处理 token 拼接逻辑。

## Markdown 知识库

- `/home/knowledge` 为知识列表，`/home/knowledge/:id` 为阅读详情，沿用主路由鉴权。
- 文件列表、网格及更多菜单使用 `components/knowledge/FileKnowledgeAction.vue` 查询来源文件的知识状态，支持单文件加入。
- `useKnowledgeImport` 管理提交、重试、状态面板与任务轮询，`useKnowledgeLibrary` 管理分页搜索、状态筛选及请求取消。
- 知识详情的向量面板 `components/knowledge/KnowledgeVectorPanel.vue` 展示向量化状态与分块进度；向量状态为部分失败时提供「重试向量化」按钮（`POST /knowledge/:id/vectorize/retry`），仅重置失败分块，成功后面板数据随任务轮询刷新。
- 知识列表每条的 `KnowledgeActionMenu.vue` 同样在知识已加入且向量状态为 `partial` 时提供「重试向量化」；成功后更新条目并重新加载当前列表（保持筛选条件），失败显示提示；同条目请求去重，离开页面取消未完成请求。
- Markdown 使用 `markdown-it` 渲染，禁用原始 HTML、危险协议、相对资源链接和所有图片请求。正文仅阅读，不编辑、不请求 AI。
- 已加入文件直接跳转详情。详情来源目录链接使用加入时快照，因此文件后续移动或删除时不能保证定位原件；不声称精确定位。
- 接入行为与验收边界见 [Markdown 知识前端](./markdown-knowledge-frontend.md)。

## 知识关系与图谱

- 知识详情页右栏最上方为「相关知识」面板 `components/knowledge/KnowledgeRelationsPanel.vue`：已确认关系（中文类型、来源 ai/manual、可改类型/删除）与待确认建议（置信度、可展开 AI 理由、确认/忽略）；「查找相关知识」在任务运行中禁用并 3 秒轮询，卸载时清理定时器与 AbortController；底部「在图谱中查看」跳转 `/home/knowledge/graph?center=<id>`。
- 图谱页 `pages/KnowledgeGraph.vue`（路由 `/home/knowledge/graph`，懒加载）+ `components/knowledge/KnowledgeGraphChart.vue`：ECharts 按需引入（core + GraphChart + Tooltip + Canvas + LabelLayout），力导向布局；节点大小按度数、颜色按来源类型从 `src/style.css` 的 `@theme` 令牌解析；边区分 AI/手动来源，建议关系虚线并显示中文关系类型标签。
- 页面交互与副作用（三个请求的取消、250ms 搜索防抖、路由 center 同步、筛选与建议边开关状态）集中在 `composables/useKnowledgeGraph.ts`；页面只保留展示与事件编排。工具栏含标签下拉（`GET /api/tags`）、来源类型下拉（manual/web 与文件格式）、关系类型下拉（全部 + 六种类型）、「显示建议边」开关与图例；点击节点打开右侧抽屉（摘要、一跳关系、跳详情、以此为中心），空图按"无知识 / 无关系 / 筛选无结果"分别引导。
- 关系类型筛选只在客户端按 `edge.relation_type` 裁剪边（`visibleEdges`），不发新请求、不裁节点——图例说明的是线型，筛选若交给后端会连节点一起裁掉。选项文案复用 `utils/relation.ts` 的 `RELATION_TYPE_OPTIONS`，与图例、详情页关系面板同源；它计入 `filtersActive`，因此「清空筛选」能一并复位，选中类型没有边时给 `data-test="relation-filter-empty"` 提示而不是退化成空图引导。
- 前端 API 契约集中在 `services/apis/knowledge.ts`（`getKnowledgeRelations`、`createKnowledgeRelation`、`updateRelation`、`deleteRelation`、`confirmRelation`、`rejectRelation`、`regenerateKnowledgeRelations`、`getKnowledgeGraph`）。
- 依赖：新增 `echarts ^5.6.0`，图谱页构建为独立懒加载 chunk（约 460 kB，gzip 约 150 kB），不进首屏；颜色解析在挂载时读取一次 CSS 令牌，主题切换后需重新进入页面。

## 演化中心

- `/home/evolution` 为演化中心（懒加载 `pages/EvolutionCenter.vue`），侧边栏「演化中心」入口；首页 `pages/DashboardPage.vue` 的「待处理提醒」卡片只调 `GET /api/evolution/alerts/summary` 取计数并链到该页，不拉列表、不引图表库。
- 单页两页签：「待处理」走 `open=true` 由后端一次取回 pending + deferred；「处理记录」用 `status=resolved,dismissed` 一次请求取回两个状态，排序（`resolved_at DESC, id DESC`）与 `total` 都由服务端给，前端只按服务端 total 分页——不要改回「两个状态各查一次再本地合并」，那样第 2 页起会拿到错位集合。
- 列表、筛选、分页与裁决状态收在 `composables/useEvolutionAlerts.ts`（AbortController 管理取消，scope 销毁时中断在途请求；换页签时复位不再适用的状态筛选），页面只做渲染编排。点列表项从右侧滑出详情抽屉，左右并排两侧证据引文并用 `utils/evolution.ts` 的 `longestCommonPhrase`/`highlightQuote` 高亮共有片段。
- API 契约集中在 `services/apis/evolution.ts`（`listEvolutionAlerts`、`getEvolutionSummary`、`resolveEvolutionAlert`）；该模块同时导出两个纯计算：`pendingTotal(summary)` 是待办视野总数（pending + deferred，演化中心页头用），`evolutionStatusesParam` 拼多状态查询参数。**首页卡片的「待处理」按规格只取 `summary.pending`，暂不处理不计入**，两者语义不同不可混用。

## 对话页

- `/home/chat` 根高度为 `h-[calc(100dvh-60px)] lg:h-auto`，扣除移动端顶栏后内部滚动；桌面端会话历史 `ConversationSidebar` 常驻，移动端（`<lg`）改为左侧抽屉（复用 HomePage 导航抽屉模式：motion-v 面板滑入、遮罩点击与拖拽左滑关闭；动效取值见上节，弹簧随旧世界作废），入口为头部 `lg:hidden` 的历史按钮，选择会话或新建后自动关闭。
- 会话项的改名/删除按钮在触屏常显、桌面端 hover/focus 显示；输入区带底部安全区 padding，键盘提示仅 `sm:` 以上显示。
- 对话不区分普通/知识库：检索由 Mastra 侧模型自主调用 `search_knowledge` 工具，`sources` 事件在回答过程中到达并渲染为引用卡片，页面无「知识库检索」开关。

## UI 与样式约定

模型设置 `/home/settings` 的页面、表单和请求管理见 [模型设置前端](./model-settings-frontend.md)。已接入真实配置存储与[基础服务检测](./service-status.md)，实际模型连接测试仍待实现。

- **视觉规范唯一权威是仓库根目录 [DESIGN.md](../DESIGN.md)（v2「索引卡盒 Card Cabinet」）。** 它在 2026-11 整份替换了 2026-09 那份「浅色 Apple 风格设计规范」（279 行 → 614 行），旧世界的令牌、圆角、阴影、字号、弹簧与动效口径**随替换一并作废，不是并存、也不是兼容关系**。v2 的版本状态是「**骨架 + 三章待回填**」：`Layout`、`States & Accessibility`、`Responsive` 三章的规则已冻结，但章首明确标注「待第一批 after-build 回填」，回填问题清单写在各章末；其余章节为已定稿。引用规范时请引章节名（v2 用 canonical order，**没有全局节号**）。
- 样式体系为 Tailwind v4 CSS-first：设计令牌集中维护在 `src/style.css` 的 `@theme`（颜色/字级/圆角/阴影/动效），**令牌表以 `DESIGN.md` v2 为准**，取值冻结于[世界参数与令牌表](https://github.com/SupremacySakura/zhixu/issues/24)。旧 `tailwind.config.js` 已废弃（仅留迁移对照）。新代码禁止硬编码色值；旧的兼容令牌 `--color-background-light/dark`、`--font-display` **已从 `style.css` 删除**，不要再引用，也不要在新代码里重建。
- 主色为**图书馆蓝 `#1f4e79`**（语义令牌 `--color-primary`）。旧品牌绿 `#10b674` **已退役为 `--color-ai`**，只作 AI 内容标记（`bg-ai-tint` + `border-t-2 border-ai`），**永不作文字色**。世界材料：暖米白画布 `#e9e5db` / 卡面 `#f2eee4` / 墨 `#181818` / 印章红 `#a8322a`；零圆角、1px 规则线、2px 硬偏移，**没有柔和模糊阴影**。v2 的 scope **不含暗色模式**：`@custom-variant dark` 与 `dark:` 变体均已移除（全仓 0 处），不再保留 class 方案能力。
- 动效由 `motion-v` 驱动（`Motion` / `AnimatePresence`），这是唯一实现，**禁止 Vue 原生 `<Transition>`**。`src/utils/motion.ts` 的三个导出名（`springSnappy` / `springBouncy` / `fadeTransition`）**保留不变**（改名要动 24 个文件、收益为零），但**值已换成 tween，`bounce` / `damping` / `response` / 动量投影 / 橡皮筋随旧世界一并作废**（该替换由[批 1：组件原子层与消息提示收敛](https://github.com/SupremacySakura/zhixu/issues/37) 落地，尚未合入前 `motion.ts` 内仍是旧弹簧值——**以规范为准，不要照抄现存代码**）；同时新增 `overlayIn` / `overlayExit` / `fadeIn` / `snapBack`。三档时长 `micro` 90ms / `standard` 160ms / `sheet` 220ms，两条曲线 `--ease-settle` / `--ease-exit`，**不新增第四档第三条**。减弱动态由 `App.vue` 的 `<MotionConfig reduced-motion="user">` 与 `style.css` 的 prefers-\* 媒体查询双轨互补；弹窗结构参照 `ConfirmDialog`（遮罩 + 面板 Motion 同组件内联）。
- **外壳**：桌面 ≥1024px = 左侧栏 **200px**（1px hairline 右边线）+ **每个页面自带的 64px 页头带**；**桌面端没有全局顶栏**（加一条会形成双层栏杆并吃掉 60px 内容高度）。移动 <1024px = 60px 全局条 + 左滑抽屉 `min(15rem, 84vw)`。`<aside>` 语义必须保留（`ChatPage.test.ts` 用 `findAll('aside')` 计数：桌面 1 / 抽屉打开 2 / 关闭后 1）。
- **页面容器只有四级**：`measure`（`--container-measure`，40em/640px，长文正文唯一合法宽度）/ `panel`（56rem/896px，表单与双列对照）/ `wide`（72rem/1152px，索引列表、表格、对话）/ `full`（无 max-width，仅画布）。`wide` 只允许出现在表格行与卡片行上，页内一旦出现连续正文段落，那段必须收进 `measure`。**禁止 `max-w-6xl` 及其它任意宽度**——非这四级的一律视为违规。
- **新增页面先选六个布局原型之一**（见[Operate 模式下的信息层级与密度](https://github.com/SupremacySakura/zhixu/issues/21)）：①索引列表 / ②长文阅读 / ③画布 / ④表单 / ⑤对话 / ⑥认证独立页。仪表盘是 ① 的**总览变体**（不是第七个原型）；分块结构页是 ② 的**双列变体**（唯一被授权突破 `measure` 的长文场景）。选型决定「容器档 × 页头带 × 密度档」三元组，页面的长相不靠换内容区分。
- 页面优先使用组合式 API 与 TypeScript 严格模式，减少隐式类型风险。
- 复杂页面应保留为编排器：API 请求、取消与资源释放放入 composable，重复视图和弹窗放入对应业务组件；模板事件优先使用具名处理函数和 typed emits。
- **三份权威的边界**（详表与冲突序见 [ADR-0003](./adr/0003-visual-world-replacement.md)）：`apps/zhixu-frontend/PRODUCT.md` 管**产品真相**（用户是谁、产品让什么成为可能、不可破的产品与实现契约）；`DESIGN.md` v2 管**视觉规范**（令牌、字级、版面语法、组件、动效）；`docs/design/` 三份毕设源头文档对 AI **全程只读**，只在人工审阅后由人修改。**冲突时：产品真相 > 视觉规范 > 源头文档的历史表述。**

## 迁移状态与验收口径

本节记录「规范已定」与「代码已实现」之间的落差，避免把未落地的规范当成现状引用。**逐项进度以[前端视觉世界迁移规格](https://github.com/SupremacySakura/zhixu/issues/35)与各批议题为准，本节只记口径。**

- 迁移按**扩展—迁移—收缩**三段式推进，不得一次性切换：批 0 地基与批 1 原子层只新增、不动旧代码（视觉零变化）；批 2 机械收敛、批 3 外壳、批 4A–4F 六个页面群分批迁移；**批 5 才删兼容层，且必须等六个页面群全完**——否则 4 个页面的 `<h1>` 会从 `text-title-1`（26px）塌成默认字号。依据见[分批实施与验收口径](https://github.com/SupremacySakura/zhixu/issues/28)。
- 兼容层现存于 `src/style.css` 的 `@theme` 段（`--text-title-1/2/3`、`--text-headline`、`--text-subhead`、`--text-caption`、`--color-material*` 等旧名映射），**标记为 deprecated**：新代码只写 v2 语义名，不引用兼容令牌，也不在兼容层上加新条目。
- 触控目标口径为 **≥44×44 CSS px**（原规范中 40 的三处已统一改为 44，见[状态语法与可访问性地板](https://github.com/SupremacySakura/zhixu/issues/26)）。
- 已知未落地项一律**以规范为准、不以代码为准**：`src/utils/motion.ts` 仍是旧弹簧值；`.vue` 模板仍用旧字号与旧 class；`<meta name="viewport">` 全仓 0 处（批 0 一行补齐）。
