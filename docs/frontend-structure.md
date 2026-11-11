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
- 页面交互与副作用（三个请求的取消、250ms 搜索防抖、路由 center 同步、筛选与建议边开关状态）集中在 `composables/useKnowledgeGraph.ts`；页面只保留展示与事件编排。工具栏含标签下拉（`GET /api/tags`）、来源类型下拉（manual/web 与文件格式）、「显示建议边」开关与图例；点击节点打开右侧抽屉（摘要、一跳关系、跳详情、以此为中心），空图按"无知识 / 无关系 / 筛选无结果"分别引导。
- 前端 API 契约集中在 `services/apis/knowledge.ts`（`getKnowledgeRelations`、`createKnowledgeRelation`、`updateRelation`、`deleteRelation`、`confirmRelation`、`rejectRelation`、`regenerateKnowledgeRelations`、`getKnowledgeGraph`）。
- 依赖：新增 `echarts ^5.6.0`，图谱页构建为独立懒加载 chunk（约 460 kB，gzip 约 150 kB），不进首屏；颜色解析在挂载时读取一次 CSS 令牌，主题切换后需重新进入页面。

## 对话页

- `/home/chat` 根高度为 `h-[calc(100dvh-60px)] lg:h-auto`，扣除移动端顶栏后内部滚动；桌面端会话历史 `ConversationSidebar` 常驻，移动端（`<lg`）改为左侧抽屉（复用 HomePage 导航抽屉模式：motion-v 弹簧滑入、遮罩点击与拖拽左滑关闭），入口为头部 `lg:hidden` 的历史按钮，选择会话或新建后自动关闭。
- 会话项的改名/删除按钮在触屏常显、桌面端 hover/focus 显示；输入区带底部安全区 padding，键盘提示仅 `sm:` 以上显示。
- 对话不区分普通/知识库：检索由 Mastra 侧模型自主调用 `search_knowledge` 工具，`sources` 事件在回答过程中到达并渲染为引用卡片，页面无「知识库检索」开关。

## UI 与样式约定

模型设置 `/home/settings` 的页面、表单和请求管理见 [模型设置前端](./model-settings-frontend.md)。已接入真实配置存储与[基础服务检测](./service-status.md)，实际模型连接测试仍待实现。

- 样式体系为 Tailwind v4 CSS-first：设计令牌集中维护在 `src/style.css` 的 `@theme`（颜色/字级/圆角/阴影/动效），唯一权威文档为仓库根目录 `DESIGN.md`。旧 `tailwind.config.js` 已废弃（仅留迁移对照），新代码禁止硬编码色值，并避免使用兼容令牌 `--color-background-light/dark`、`--font-display`。
- 主色为品牌绿 `#10b674`（语义令牌 `--color-primary`）；浅色优先，暗色模式保留 class 方案能力（`@custom-variant dark`），暗色令牌后续单独定义，当前代码不新增 `dark:` 变体。
- 动效由 `motion-v` 驱动（`Motion` / `AnimatePresence`），弹簧预设统一取自 `src/utils/motion.ts`（`springSnappy` / `springBouncy` / `fadeTransition`）；减弱动态降级为 `App.vue` 的 `<MotionConfig reduced-motion="user">` 加 `style.css` 的 prefers-\* 媒体查询。新增浮层/弹窗入场必须走 motion-v，不要再引入 Vue `<Transition>`；弹窗结构参照 `ConfirmDialog`（遮罩 + 面板 Motion 同组件内联）。
- 页面优先使用组合式 API 与 TypeScript 严格模式，减少隐式类型风险。
- 复杂页面应保留为编排器：API 请求、取消与资源释放放入 composable，重复视图和弹窗放入对应业务组件；模板事件优先使用具名处理函数和 typed emits。
