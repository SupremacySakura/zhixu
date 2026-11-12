<!-- impeccable:product-schema 1 -->

# 知序（KnowledgeFlow）

## Platform

web

桌面优先的个人知识工作台；移动端在本次范围内只做响应式适配，不做原生 App，因此平台记为 `web` 而非 `adaptive`。

## Stack

- 前端：Vue 3 + TypeScript + Vite + Pinia + Vue Router + Tailwind CSS v4（CSS-first，令牌集中在 `src/style.css` 的 `@theme`）+ motion-v + Axios + Vitest
- 后端：Go 1.25 + Gin + GORM + MySQL 8.4
- AI 服务：Mastra 1.x + Qdrant（仅内部可达，不直接暴露给浏览器）
- 验证命令（仓库根）：`pnpm dev` / `pnpm build` / `pnpm typecheck` / `pnpm test` / `pnpm lint` / `pnpm verify`

## Users

**主要用户是一个人：我自己**，在写毕业论文与课程资料的过程中使用——边读长文，边把材料拆解、归类、沉淀成可检索的知识。

这个判断与本仓文档的记载一致但更窄：`docs/design/产品设计文档.md` §1.4 列的是「开发者 / 学生与研究人员 / 知识工作者」三类，`docs/design/需求规格说明书.md` §2.2 记载系统只有「注册用户」一种角色、数据严格按用户隔离、毕设场景下同一人可用多个账号验证多用户行为。**实际使用场景是单人深度工作，不是团队协作，也不是给第三人演示的产品。**

主要使用情形：一个人在桌面浏览器前，长时间阅读与整理，手边有大量待归类的资料。

## Product Purpose

把「个人知识的积累与维护」从一件需要持续手工组织的事，变成一件只需积累、由系统承担理解的事。

`docs/design/产品设计文档.md` §2.1 的五个产品目标仍在生效：统一管理 / 降低整理成本 / 提高检索效率 / 建立知识体系 / 辅助知识维护。

## Positioning

**用户只负责「存」，系统负责「懂」。**

这是与最近邻的根本分界。Notion、Obsidian 一类工具把组织工作交还给用户：你得先建目录、打标签、拉双链，知识的秩序来自你的手工劳动。知序反过来——内容进来之后自动分块、生成摘要与标签、建立知识间的关系、持续提醒哪条过时了或与哪条冲突。用户的动作止于「收进来」，之后的秩序由系统维护，用户只需要在系统给出的候选上确认或推翻。

`docs/design/产品设计文档.md` §1.3 的原话与之一致：「用户负责积累知识，系统负责理解知识、建立关联、辅助检索，并帮助用户持续维护知识。」

明确**不是**：不是 AI 聊天工具，不是 Markdown 笔记软件，不是团队知识库或企业协作平台。

## Operating Context

- 单人、单账号、数据完全隔离；无共享、无协作、无权限分级。
- 长文阅读是最高频动作：知识详情、分块列表、演化对比、AI 问答的回答，都是长文本。
- AI 介入的是「整理」而非「创作」：系统产出的是分块、摘要、标签、关系、演化提醒这类结构化中间物，由用户确认后才落地。
- 用户会在同一台机器上反复回到同一批知识上，因此「再次进入时的可辨识性」比「首次进入时的惊艳」更重要。
- 离线或内网环境下也需要可用：西文品牌字形（Courier Prime）已于 2026-11 改为自托管（38.1 KB 两个 woff2，含 OFL 授权），但**图标族仍依赖 Google Fonts 的 Material Symbols 外链**，是已知缺陷，按既定结论必须先接本地图标数据再删外链（顺序不可颠倒：缺图时 Iconify 只静默输出空节点，测试不会报错）。

## Capabilities and Constraints

### 不可动的产品事实

- **只换视觉世界，不动信息架构**：路由结构、接口契约、功能语义保持不变。本次工作是视觉与呈现层的替换，不是产品重构。
- **AI 生成内容绝不自动覆盖用户的原始知识**；模型 API 密钥只存服务端，永不下发浏览器（`docs/design/需求规格说明书.md` §2.4）。
- 双服务架构：Go 业务服务 + Mastra AI 服务，AI 服务仅内部可达。
- AI 调用一律结构化输出并做 Schema 校验；能用确定性代码完成的流程不调 LLM。
- **离线/内网必须可用**：字体与图标不得只依赖第三方 CDN 外链。

### 本次工作必须保留的实现契约

- **用户可见的中文文案一个字都不能改**：测试中有 193 处 `wrapper.text()` 文案断言。
- **语义标签结构不能变**：`find('button')` 35 处、`aside` 9 处、`select`/`input` 各 6 处，以及 `a`/`nav`/`header`/`dialog`/`mark` 若干。把 `<button>` 换成 `<a>`、把侧栏 `<aside>` 换成 `<div>` 都会打断测试。
- **16 个 `data-test` 钩子必须逐字保留**：`type-filter`、`empty-filtered`、`empty-none`、`alert-card`、`evidence-${index}`、`tag-filter`、`source-filter`、`relation-filter`、`suggested-toggle`、`clear-filters`、`clear-filters-empty`、`relation-filter-empty`、`graph-legend`、`legend-contradicts`、`evolution-card`、`evolution-pending-total`。
- 类名断言只有 1 处：`src/pages/__tests__/KnowledgeActionMenu.test.ts:63` 的 `classList.contains('text-danger')`。
- 当前测试基线：364 个用例、52 个测试文件全绿。任何改造必须保持这个基线。

### 范围边界

- **不纳入**：深色模式（只预留令牌位）、移动端原生式重做、后端与 AI 与数据库变更、`docs/design/` 三份源头文档的直接编辑（AI 只读）。

## Brand Commitments

- **名称**：知序（KnowledgeFlow）。「知」为知识，「序」为秩序——名字本身讲的是「系统替你把无序变有序」，与 Positioning 同源。
- **历史品牌色**：品牌绿 `#10b674` 是 2026-09 那一轮视觉世界的既有资产。2026-11 的视觉世界替换已裁定它**不再作主色**，退役为 `--color-ai`（只标 AI 产出，且只可当底色、不可作文字色 —— 该绿作文字时对比度 1.94–2.28，不达标）。新世界的品牌与强调同色：图书馆蓝 `#1f4e79`。**这是视觉世界的决策，权威定义在 `DESIGN.md`，本文件只记录结论、不裁定取值。**
- 用户在本次改造中给出的绑定约束（原样记录，不展开）：视觉世界必须按 impeccable 的流程产生，不接受未经 `concept-seed` 或其他合规 provenance 确认的方向。

## Evidence on Hand

- 三份源头文档（对本 AI 只读）：`docs/design/需求规格说明书.md`（V2.1）、`docs/design/产品设计文档.md`（V1.1）、`docs/design/架构设计方案.md`（v2）。
- 领域术语表：仓库根 `GLOSSARY.md`。
- 现行设计规范：仓库根 `DESIGN.md`（v2，索引卡盒世界，2026-11 产出；Layout / States & Accessibility / Responsive 三章是骨架 + 待第一批构建后回填的坑位，其余章节已定案）。**2026-09 那版 279 行浅色 Apple 规范已被整份替换**，只作为「被替换的世界」留档于该文件附录，不再是当前实现的描述。
- 一手视觉证据：`/tmp/zhixu-shots/` 下 9 张登录后页面截图（1600×857）+ 2 张移动端截图（390×844），登录页截图同目录。
- 两份已完成的研究笔记（在一次性 `research/*` 分支上，未合并入 main）：
  - `research/chinese-typography-baseline` → `docs/tasks/frontend-chinese-typography-baseline-20261112.md`（中文排版事实基线）
  - `research/chinese-font-selfhost` → `docs/tasks/frontend-chinese-font-selfhost-20261112.md`（中文字体自托管可行性）
- 代码侧证据：52 个 `.vue`、17 个页面、`components/ui` 仅 5 个组件且无 Button/Input/Card/Badge/Dialog 原子层；Tailwind 任意值 262 处、17 种字号并存、裸 `<button>` 196 个。

## Product Principles

1. **系统承担整理，用户只做确认。** 任何让用户先做组织的设计都是在背叛定位。
2. **长文可读性优先于界面表现力。** 产品的主场是读，不是看。
3. **AI 产出必须可追溯、可推翻、不覆盖原件。** 每个系统判断都要能跳回依据，每个自动动作都要能反悔。
4. **再次进入比首次进入更重要。** 熟悉感、可预测的位置、稳定的节奏，胜过一次性惊喜。
5. **不可用性即失败。** 离线、弱网、内网环境下核心阅读路径必须仍然成立。

## Accessibility & Inclusion

- 中文用户为主，界面语言为简体中文（`<html lang="zh-CN">`）。
- 正文最小 16px、辅助文字不低于 12px；这是中文排版的可读性地板，也是既有研究笔记的结论。
- 必须尊重 `prefers-reduced-motion`；动效是可选项而非信息载体。
- 键盘可达：所有交互元素需有可见焦点环，且侧栏、抽屉、弹窗的焦点管理需完整。
- 对比度：正文与背景、次级文字与背景均需达到 WCAG AA。令牌层已达标（正文墨 `#181818` 最低 13.09、次级 `#605949` 最低 5.12、强调蓝 `#1f4e79` 最低 6.39）。**三级文字 `#847c6b` 最低 3.05，只够非关键文本** —— 现有 58 处用法已定案「改 34 / 删 1 / 保留 23」，属实施范围内待收敛项。
