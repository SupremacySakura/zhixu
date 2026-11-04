---
task_id: chat-mobile-responsive-20261104
status: completed
created: 2026-11-04
last_reviewed: 2026-11-04
applicable_to: "对话页、移动端适配、响应式、抽屉"
---

# 对话页移动端适配

## 背景

`/home/chat` 在手机上基本不可用：会话历史栏 `ConversationSidebar` 以 `w-64 shrink-0` 常驻 flex 布局，375px 宽屏只剩约 119px 给对话区；会话项的改名/删除按钮依赖 `group-hover`，触屏无法触发。根高度 `h-[calc(100dvh-60px)] lg:h-auto`、消息气泡、工具栏 `flex-wrap` 等此前已自适应，本次只补缺口。

## 结果

- 移动端（`<lg`）会话历史改为左侧抽屉，完全复用 HomePage 导航抽屉模式：`AnimatePresence` + `Motion` 弹簧滑入（`springBouncy`）、遮罩点击关闭、`pan-end` 拖拽左滑关闭（速度 < -400 或位移 < -80）。入口为对话页头部 `lg:hidden` 的历史图标按钮（`aria-label="打开会话历史"`）。
- 桌面端布局不变：`ConversationSidebar` 包进 `hidden h-full lg:block` 容器；侧栏根节点改 `w-full lg:w-64`，在抽屉内填满面板。
- 抽屉内选择会话 → 先关抽屉再走原 `openConversation`；「＋ 新对话」同理（无消息/草稿时不弹确认，直接 reset）。
- 会话项改名/删除按钮：`flex lg:hidden lg:group-hover:flex lg:group-focus-within:flex`——触屏常显，桌面 hover 显示不变，顺带补键盘 focus 可达。
- 头部压缩：面包屑「工作空间 / 对话」`hidden lg:block`（移动端顶栏已有页面名），padding 改 `px-4 py-3 lg:py-5`；输入区 `pb-[max(1rem,env(safe-area-inset-bottom))]` 适配 iPhone Home 指示条，键盘提示 `hidden sm:inline`（右侧按钮组加 `ml-auto` 防止提示隐藏后靠左）。
- 桌面与移动端抽屉各持有一个 `ConversationSidebar` 实例（`sidebarDesktop`/`sidebarMobile` 两个 ref），首轮对话完成后两个列表都 `reload()`；抽屉实例随 `v-if` 挂载即自动拉新列表。

## 决策与理由

- 抽屉模式照搬 HomePage 而非抽公共组件：目前仅两处使用，抽象收益低；第三处出现时再抽 composable（见候选特性）。
- 不改任何交互行为：Enter 发送、流式渲染、RAG 开关、确认弹窗逻辑全部保持原样，本次纯布局适配。

## 教训

- 组件测试里 `AnimatePresence` 退场动画不可控（happy-dom 下弹簧 exit 时长不确定），断言抽屉关闭必须 mock `motion-v` 为同步模板（`{ template: '<slot />' }` / `{ template: '<div><slot /></div>' }`），此模式此前已在 KnowledgeActionMenu.test.ts 建立；注意 mock 必须同时提供 `animate: vi.fn()`，否则 import 报错。
- flex `justify-between` 的行里隐藏左侧元素会导致右侧元素靠左，隐藏时要给右侧加 `ml-auto`。

## 候选特性

- HomePage 导航抽屉与对话页历史抽屉已有两份相同的拖拽关闭处理（约 15 行）：第三个抽屉出现时抽 `useDrawerPanClose` composable。

## 验证

- `pnpm vitest run src/pages/__tests__/ChatPage.test.ts`：8/8（新增开/关抽屉、抽屉内选会话并关闭、抽屉内新对话不弹确认 3 例）。
- 全量：`pnpm typecheck`、`pnpm lint`、`pnpm vitest run`（225 通过）、`git diff --check` 全绿。
- 未做浏览器窄屏真机验证，需人工确认抽屉手势与布局观感。
