---
version: 1
slug: 'src-pages-knowledgelibrary-vue'
primary_target: 'src/pages/KnowledgeLibrary.vue'
related_targets: []
---

## Scope and mode

Surface：知识库列表页 —— 路由 `/home/knowledge`，主文件 `apps/zhixu-frontend/src/pages/KnowledgeLibrary.vue`（含本页用到的 `components/knowledge/*`、`components/ui/*`）。
Visitor mode：**Operate**。单人（作者本人）每天多次进入这一页，扫读、筛选、重试失败的向量、打开某条知识。世界只借四样：字体、配色、密度、一个招牌动作。

## Audience, job, task

- **受众**：作者本人。写毕设与课程资料时边读长文边把材料存进来，回到这一页是为了「找到那条、看它现在什么状态、决定下一步」。
- **任务**：一屏之内扫过 142 条 → 按状态或标签筛 → 认出哪条向量失败、哪条正在整理 → 打开它或就地重试。
- **内容证据（全部真实，不做占位块）**：真实中文标题（Transformer 位置编码的三种实现、BERT 与 GPT 训练目标的差异、向量检索的 HNSW 参数取舍…）、分块数、向量状态、标签、更新时间、六类关系计数。
- **约束**：中文文案一字不改（测试里 193 处 `wrapper.text()` 断言）；16 个 `data-test` 钩子全保留；`<button>` / `<aside>` / `<select>` / `<input>` 语义标签结构不变；离线与内网可用（不引外链字体、不引外链图标）；364 用例基线不破。

## Chosen direction and memorable moment

方向：**索引卡盒 Card Cabinet**（图书馆卡片目录 / Zettelkasten）。用户在决策页从六张候选卡里选了它，而不是掷骰指派的「织样稿」。
招牌动作：每条知识是一张**硬卡** —— 顶边一行打字机标题、左上分类号、右下日期戳、正文压在蓝格线上；**交叉引用另立在卡片右侧**，是一叠手写小卡（相关 / 依赖 / 矛盾 / 补充），不是行内图标。

## Unresolved decisions

- 一屏放几张卡、卡高上限（草图取 6 张 / 118px）要在实做时按真实视口定。
- 打字机字形在中文里怎么落：中文没有等宽打字机字面，要决定「中文系统黑体 + 西文数字等宽」的混合，还是中文也走更硬的无衬。
- 关系小卡最多显示几条、超出部分如何收纳。
- 卡片 1–2px 硬偏移堆叠在长滚动列表里是否保留（影响密度与可扫读性）。
- 390px 移动端上卡盒形态如何退化。

## Direction contract

**THESIS** 一条知识一张硬卡，顶边一行打字标题，交叉引用另立在右侧。它拒绝品类默认的「圆角白卡 + 柔和阴影 + 胶囊筛选」，也拒绝把工作屏做成报纸或海报。

**OWN-WORLD** 马尼拉卡纸 `#e9e5db` 作地、卡面 `#f2eee4`、墨 `#181818`；图书馆蓝 `#1f4e79` 只承担结构与选中；印章红 `#a8322a` 只给真需要被看见的那一条。零圆角、1–2px 硬偏移替换模糊阴影、蓝色格线、打字机字形与借出章。

**STORY** 访客一眼认出这是一盒被认真维护的资料：每条是什么、系统对它做了什么、它和谁有关，三件事都能立刻读到，并且能马上开始一条新的。

**FIRST VIEWPORT** 左侧 200px 马尼拉导航栏（九项中文，知识库为当前项，蓝色标记条）；顶栏「知识库」+ 计数 + 检索（⌘K）+ 主操作「新建知识」（蓝实心）+ 次操作「从文件加入」（描边）；筛选行 状态 / 向量 / 来源 / 标签 / 排序，右端「命中 9 / 142」；主区一列硬卡，每张不超过 118px：顶边打字标题、左上 `KM-0142`、右下日期戳、压蓝格线的摘要、meta 行（分块 12 · 已向量化 · 注意力机制 / PyTorch），右侧一叠交叉引用小卡。主操作落在顶栏右端，紧邻检索。

**FORM** 索引卡盒 Card Cabinet —— 我的排序第 1 位（IMPECCABLE'S PICK），`seed:3e5b4b087e29fc24`。用户在决策页改选此卡，覆盖了掷骰指派的「织样稿」（第 7 位）；原指派卡上的三条 raise 随未选方向一并作废，不并入本世界。

**FINISH** unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
