---
task_id: frontend-chinese-typography-baseline-20261112
status: completed
created: 2026-11-12
last_reviewed: 2026-11-12
applicable_to: "中文长文排版、字号行距体系、Tailwind v4 字级令牌、阅读页布局"
---

# 中文排版事实基线：面向长时间阅读的具体数值与规则（#32）

> 本文回答 GitHub 议题 #32（父议题 #18「UI 世界替换：纸感编辑版面」）。
> 它是未来 `DESIGN.md` v2「字型与版面」章节的**前置事实**，不是设计决策本身——
> 决策见 `## 待定与风险` 中需要拍板的四项。
>
> 体例说明：本文正文用 `## 背景 / ## 结论 / ## 依据 / ## 待定与风险` 四节，
> 与 `docs/tasks/README.md` 模板（结果/教训/可沉淀模式）刻意不同，以对齐议题 #32 的验收要求；
> 可复用教训并入 `## 依据` 末尾的「取证方法」小节。
>
> 引用约定：`[clreq §x]` 指 W3C《Requirements for Chinese Text Layout》对应章节锚点（链接见 `## 依据`）。
> **clreq 全文没有任何 px / CSS pixel / 屏幕相关数值**（`px`、`CSS pixel`、`device pixel` 命中数均为 0），
> 因此本文所有 px 值均为「pt→px（×4/3）+ 行距百分比」的**换算推导**，不是规范原文，已在表中标注。

## 背景

「知序」的产品第一动作是**读**：知识详情、分块（chunks）、演化对比、对话记录，全是长文。
UI 世界替换为「纸感编辑版面（Editorial Paper）」后，中文字号与行距体系是这个世界的地基——
纸感的来源不是纸纹理和阴影，而是版心比例、行距、字重克制与规则线。

现状（实测于 `research/chinese-typography-baseline` 分支，基线 `0d64988`）：

- `apps/zhixu-frontend/src/style.css:68-97` 有 8 档令牌，但**前端源码里 `text-[Npx]` 任意值共 191 处、16 种不同字号**
  （`text-[13px]` 38、`[15px]` 35、`[18px]` 33、`[16px]` 31、`[20px]` 16、`[14px]` 13……），
  各类任意值合计 309 处。设计令牌事实上没有约束力。
- 正文令牌是 `--text-body: 15px / 22px`（行距 46.7%，低于 clreq 50% 下限），
  而真正承载长文的却是不带行高配对的 `apps/zhixu-frontend/src/pages/KnowledgeChunks.vue:203`
  `text-subhead leading-7`（14px / 28px）。
- 阅读容器是 `apps/zhixu-frontend/src/pages/KnowledgeDetail.vue:130` 与 `KnowledgeChunks.vue:96` 的
  `mx-auto max-w-6xl`（72rem = **1152px**）。14px 下约 82 字/行，**远超 clreq 横排 48 字上限**。
- 没有 `letter-spacing` 体系（仅 title/display 的 −0.02em 与 caption 的 +0.02em），没有 `--font-serif`，
  没有 measure / 段距 / 缩进令牌，没有中西混排与标点处理。

一句话：现有体系是「App 界面字级」，不是「阅读版面字级」。本笔记给出后者的一组可落地数值。

## 结论

### 1. 可直接落地的数值表（阅读版式 reading scale）

数值按「正文 16px、中文黑体或宋体、屏幕长文」这一目标场景给出。
`依据` 列写来源类型：**规范** = 标准原文要求；**实践** = 产品实测取值；**推导** = 由规范换算/推断。

| 条目 | 建议值 | 依据 |
| --- | --- | --- |
| 正文字号 | **16px** | 实践：掘金 16px、少数派 15px、端传媒 18px、微信读书默认 21px；Butterick 网页 15–25px；推导：clreq 五号 10.5pt ≒ 14px 是印刷惯例，屏幕需上调一档 |
| 正文行高 | **28px（1.75）** | 规范：clreq line-gap 50%–100% ⇒ 1.5–2.0×（16px 对应 24–32px）；实践：typo.css 1.75、掘金 1.75、少数派 1.8、微信读书 1.905 |
| 正文 line-gap | **12px（字号的 75%）** | 规范：clreq「75% 落在 50%–100% 中段」，且明确不超过 100% |
| 段间距 | **1em = 16px**（与首行缩进二选一） | 规范：clreq 允许「不缩进 + 段间距」一法；实践：掘金 `p{margin:22px 0}`、端传媒 `1.33333em`、typo.css `1.2em`、Butterick 50–100% 字号 |
| 首行缩进 | **0（屏幕长文）**；纸感复刻可选 **2em = 32px** | 规范：clreq 书籍标准 2em、且「各段落间不使用间距」；实践：微信读书 `text-indent:0!important`、少数派/端传媒全文 `text-indent` 为 0 |
| measure（字数） | **36–40 字**，硬上限 **48 字** | 规范：clreq 正文 17–40 字、横排 ≤48 字；jlreq 横排 ≤40 字 |
| measure（px @16px） | **576–640px**，硬上限 **768px** | 推导 + Tailwind 容器对照见 §2 |
| 标题字阶（h1→h3） | **24 / 21 / 18**（相对正文 +12.5% / +16.7% / +14.3%） | 规范：clreq 逐级放大 10%–20% |
| 页标题（display） | **30px**（正文 +87.5%，跨两级） | 推导：clreq 只约束「逐级 10–20%」，页标题可跨级 |
| 标题行高 | **26 / 30 / 34 / 38px（1.35–1.4）** | 实践：typo.css 1.35、端传媒 `--leading-heading:1.4`、少数派 1.25–1.4；规范未给标题行高 |
| 标题字重 | **600**（宋体 600 / 黑体 600），不用 700+ | 规范：clreq「宋体作标题需加强字重」「黑体作小标需加重」但无数值；实践：Windows「用 Semibold 不用 Bold」、AntD 只 400/500、少数派 500/600、端传媒 700/600 |
| 标题 letter-spacing | **0em** | 规范：clreq 紧排（负字距）「不应用于书籍标题与内文的排列上」⇒ 现有 −0.02em 应移除 |
| 正文 letter-spacing | **0em** | 规范：clreq 密排「原则上字符外框彼此紧贴」 |
| 小字 letter-spacing | **0em**（可选 +0.01em，不超过 +0.02em） | 实践：Material 3 小字号 tracking 为正（12px 档 +0.4px）；规范：clreq 疏排仅限短文本 |
| 中西混排间距 | **`text-autospace: normal`（= 1/8 ic）**，不要用 letter-spacing 模拟 | 规范：clreq「不超过 1/4 em」；CSS Text 4 定值 1/8 ic；实践：typo.css / zh.wikipedia 已默认开启 |
| 数字 | 表格与统计用 `tabular-nums`；散文中保持比例数字 | 实践：AntD 建议 tabular-nums；规范：clreq「阿拉伯数字常用比例或等宽字体」 |
| 最小可用字号 | **12px**（元信息/标签）；**宋体不低于 16px** | 规范：clreq 六号 7.875pt ≒ 10.5px 为印刷可读下限、字号体系「仅供参考」；实践：Apple HIG 最小 10–11pt 且「避免 light 字重，尤其小字号」 |
| 对齐方式 | 正文 `text-align: justify` + `line-break: strict`，`@supports` 失败回落左对齐 | 规范：clreq「原则上应该进行两端对齐」；实践：微信读书 justify、掘金/Windows 左对齐（分歧，见待定 §3） |
| 字阶档位数量 | 阅读页 **5 档** + UI 复用，总量 ≤ 8 档 | 实践：AntD「字阶控制在 3–5 种」；现状 16 种 |

### 2. measure 与 Tailwind v4 / CSS 的对应

Tailwind v4 容器令牌（`--container-*`，`packages/tailwindcss/theme.css:333-345`）与 16px 正文字数的换算：

| Tailwind | rem | @16px 根字号 | @16px 正文字数 | 判断 |
| --- | --- | --- | --- | --- |
| `max-w-xl` | 36rem | 576px | 36 字 | 可用（偏紧，clreq 中段） |
| `max-w-2xl` | 42rem | 672px | 42 字 | **略超 40 字，建议不用作正文** |
| `max-w-3xl` | 48rem | 768px | 48 字 | 正好是 clreq 硬上限，**不可超出** |
| `max-w-4xl` | 56rem | 896px | 56 字 | 超限 |
| `max-w-6xl` | 72rem | 1152px | 72 字 | 现状容器，**严重超限** |

结论：正文容器**不要直接用 Tailwind 预设档**，而是引入语义令牌 `--measure`（`40em`），
这样正文字号变化时 measure 的字数保持不变（clreq：行长应为字号整数倍；40em 在任意字号下都是 40 字）。
窄屏回落 `width: 100%` + 左右 24px 内边距。

### 3. 建议的令牌草案（Tailwind v4 `@theme`，可直接改 `style.css`）

```css
@theme {
  /* 衬线：v4 默认 --font-serif 无中文字体，必须显式声明 */
  --font-serif:
    'Source Han Serif SC', 'Noto Serif SC', 'Songti SC', 'STSong', SimSun, ui-serif, Georgia, serif;

  /* 阅读版式：measure 与行距用「相对单位」，字数不随字号漂移 */
  --measure: 40em;          /* 640px @16px = 40 字（clreq 17–40 中上限） */
  --measure-wide: 48em;     /* 768px @16px = 48 字（clreq 横排硬上限，仅大屏备选） */
  --leading-reading: 1.75;  /* clreq line-gap 50%–100%，取 75% */
  --leading-heading: 1.4;   /* 1.35–1.4 区间 */
  --paragraph-gap: 1em;     /* 与首行缩进二选一 */
  --tracking-cjk: 0em;      /* clreq 密排；标题同样为 0（不得用负字距） */

  /* 阅读字阶：5 档，行高按 1.75 / 1.4 实算成 px 保持栅格 */
  --text-prose: 16px;       --text-prose--line-height: 28px;      --text-prose--font-weight: 400;
  --text-h3: 18px;          --text-h3--line-height: 26px;         --text-h3--font-weight: 600;
  --text-h2: 21px;          --text-h2--line-height: 30px;         --text-h2--font-weight: 600;
  --text-h1: 24px;          --text-h1--line-height: 34px;         --text-h1--font-weight: 600;
  --text-display: 30px;     --text-display--line-height: 38px;    --text-display--font-weight: 600;
  --text-prose-small: 14px; --text-prose-small--line-height: 22px;--text-prose-small--font-weight: 400;
}

/* 正文容器（阅读页唯一排版入口） */
.measure {
  max-width: var(--measure);
  line-height: var(--leading-reading);
  letter-spacing: var(--tracking-cjk);
  line-break: strict;              /* 禁则：line-break 三家均已支持 */
  text-autospace: normal;          /* 中西混排 1/8 ic；CSS Text 4 */
  text-spacing-trim: normal;       /* 标点挤压；目前仅 Chromium，可渐进增强 */
  text-wrap: pretty;
  hanging-punctuation: allow-end;  /* 仅 Safari 26.5+，无副作用 */
}
```

### 4. 六个问题的直接答复（议题 #32 原文）

1. **字体分工**：clreq 说宋体是「中文印刷最常用字体」，黑体「传统印刷品少用作内文」，
   标题「黑体或宋体加粗」皆可。**没有唯一正确答案**，业界已出现分裂：
   微信读书/掘金/少数派/澎湃正文全用黑体（宋体只用于书名、封面等短文本），
   端传媒反向——`--font-heading: serif`、`--font-body: sans`（标题宋、正文黑）。
   双方论据见 `## 待定与风险` §1。
2. **字号与行距区间**：正文 15–18px 是实践区间，16px 是众数；行高 1.7–1.9
   （clreq 允许 1.5–2.0，且明确「超过 1.0 倍字号不会增加易读性」）；标题与正文的比例用 clreq 的逐级 10%–20%。
3. **measure**：36–40 字、576–640px（@16px）；48 字 / 768px 是硬上限，现状 1152px 必须收窄。
4. **标点挤压、字距、中西混排**：正文字距 0（密排）；中西混排用 `text-autospace`（1/8 ic，规范上限 1/4 em），
   不要用 `letter-spacing` 模拟；行尾全角标点占半字、相邻双标点压到 1.5 字宽（`text-spacing-trim` 覆盖）。
5. **缩进、段距、分节**：clreq 书籍体例是 2em 缩进 + 段间零间距；
   屏幕实践统一改为「零缩进 + 段间距」，两者不可并用（Butterick 明确「Don't use both」）。
   分节靠标题上留白（1.2em）+ 规则线，不靠空行。
6. **小字号衬线边界**：**没有一手规范给出「宋体低于 X px 不可用」的数值**。
   可给的推理链：宋体笔画有粗细对比与装饰端部 → 小字号下细笔画落到 1px 以下易糊 → 需专为小字号设计的字面。
   实证支撑：Adobe Source Serif 4 提供 `opsz` 轴 8–60（默认 14），并为 Caption/Small Text 单独切字面；
   而思源宋体（Noto Serif SC）只有 `wght` 轴、**没有 opsz**，即中文衬线一个设计通吃所有字号。
   ⇒ 保守边界：正文级 ≥16px 才用宋体，14px 为警戒线，**<14px 禁用宋体**（此为推导结论，非规范数值）。

## 依据

### A. 规范与标准（W3C clreq 为主，逐字引用）

**1. 行距（最关键数值）** — [clreq §7.1.1.5](https://www.w3.org/TR/clreq/#x7-1-1-5-considerations-when-designing-the-type-area)

> "The line gap for the type area is commonly set to a value between 50% and 100% of the height of the character frame used for the type area. A shorter line gap can be chosen in cases where the line length is short or the font size of the type area is relatively small. On the other hand, the line gap usually does not exceed the font size. Increasing the line gap beyond the font size does not improve the reading experience."

中文原文：「版心的行距多半介于字型大小的 50%–100% 之间，当行长较短或字型较小时，行距设定也会相对较小。
反之，行距一般不会超过字型大小，就算超过字型大小，也不会因而增加易读性。」

clreq 给出的定义：`line height = font size + line gap`，即 line-gap 50%–100% ⇔ **line-height 1.5–2.0**。
「不要超过 2.0」是硬性经验上限——现有 `KnowledgeChunks.vue` 的 14/28（2.0）已顶格，
而 `--text-body` 的 15/22（1.467 / gap 46.7%）**低于下限**。

**2. 行长（measure）** — 同节

> "For the sake of reference, most books have line lengths of body text in the range between 17 to 40 characters. To prevent excessive line breaks that could cause difficulty in reading, the line length of body text should not be less than 10 characters. For horizontal writing mode, the line length of body copy should not exceed 48 characters. For vertical writing mode, the line length of body copy should not exceed 55 characters."

另：「一行的行长应为字型大小的整数倍，各行的位置尽可能头尾对齐。」（这句是 `--measure: 40em` 用 em 而非 px 的直接理由。）

**3. 标题比例与字体** — [clreq §7.1.3.2](https://www.w3.org/TR/clreq/#x7-1-3-2-font-selection-and-heading-font-size)

> "The font size of headings is usually larger than that of the main text. When this rule is applied, the characters in the heading should be 10% to 20% larger. And the font size of higher level headings is larger than the size of smaller size headings."

> "Typefaces for headings: Both Hei or bold Song are usually used. Other type face designs like Yuan and Kai are sometimes used as well."

> "In letterpress printing, when Song is used as the type face of the heading, it is always designed with a larger size; while in digital printing, due to its special typography, simply increasing the font size is not enough, the characters usually use bold style to indicate emphasis."

（clreq 对字重只有定性表述，`font weight` 一词全文命中 0——**没有字重数值可引**。）

**4. 首行缩进与段间距** — [clreq §6.2.1.1](https://www.w3.org/TR/clreq/#x6-2-1-1-first-line-indents)

> "Chinese publications usually apply 2-em wide first-line indents. In some cases, publications like magazines with multi-column content and less text in each column might apply a 1-em wide first-line indent."

> "In principle, there is no extra spacing between paragraphs in Chinese books. The space between the last line of the previous paragraph and the first line of the next paragraph stays the same as the space between the rest of the lines in the paragraph. If a blank line is inserted after a paragraph, usually it indicates the end of the section or chapter."

clreq 并列四种段落区分法，其中第三种正是屏幕实践的通行做法：
「所有段落首行皆不缩排，在段落与段落间加入一定程度的间距，作为间隔。部分书籍与杂志使用这种方式。」
注意英文 Note 的警告：采用段间距后**不应再用空行表示分节**，否则「留空过多造成体例不良」。

**5. 字距：正文字距为 0** — [clreq §6.3.1](https://www.w3.org/TR/clreq/#x6-3-1-principles-for-arranging-characters-during-chinese-composition)

> "When composing a line with characters, in principle, no extra spacing appears between their character frames. This is called solid setting."

「疏排（loose setting）」仅用于「字数少的标题求平衡、图表说明、诗词、儿童书籍」，示例值为 1/4 em、1/3 em、1/2 em、1 em。
[紧排 §6.3.1.3](https://www.w3.org/TR/clreq/#x6-3-1-3-reduced-inter-character-spacing) 追记：
「reduced inter-character spacing」只用于杂志标题/广告，**「不应用于书籍标题与内文的排列上」**——
因此现有 `--text-title-1--letter-spacing: -0.02em` 与 `-0.01em` 这类负字距，对中文正文与标题都属违规用法。

**6. 中西混排** — [clreq §6.3.3](https://www.w3.org/TR/clreq/#x6-3-3-mixed-text-composition-in-horizontal-writing-mode)

> "In principle, there is tracking or spacing between an adjacent Han character and a Western character or a European numerals of up to 1/4 em, except at the line start or end."

例外：「在中文点号前后、中文开始夹注符号之后、结束夹注符号之前的西文字母或阿拉伯数字，不调整字距或加入空白。」
实现方案 clreq 给了两条：插入 U+0020 SPACE，或用 CSS 自动间距（1/4 em 为上限，非定值）。

**7. 标点挤压** — [clreq §6.3.2](https://www.w3.org/TR/clreq/#x6-3-2-2-adjustment-of-adjacent-punctuation-marks)、[§6.3.2.3](https://www.w3.org/TR/clreq/#x6-3-2-3-compression-of-punctuation-marks-at-line-start-or-line-end)

> "Many publications in Taiwan do not adjust punctuation width, while most publications in Mainland China and Hong Kong do adjust punctuation width."

挤压规则：「如果任意两个相邻标点符号占用 2 个字宽，应当缩减成 1.5 个字宽。」
行尾全角标点（引 GB/T 15834—2011 §5.1.10）：「当标点符号排在一行末尾时，若为全角字符则应占半角字符的宽度（即半个字位置）。」

**8. 最小字号** — [clreq §3 号数表](https://www.w3.org/TR/clreq/)

> "Size 5 is usually used for body text. Newspapers and magazines use both Size 5 and New Size 5. The acceptable minimum size for the text in content is Size 6 (7.875 pt ≒ 2.8 mm). If a smaller size is used, it will be difficult to read due to the complex structure of Han characters."

号数表本身声明「仅列出常见的一些换算数值，仅供参考，不作为规范性规定」；按 1pt = 4/3 px 换算：
五号 10.5pt ≒ 14px、新五号 9pt ≒ 12px、六号 7.875pt ≒ 10.5px、小四 12pt ≒ 16px、四号 14pt ≒ 18.7px、三号 16pt ≒ 21.3px。

**9. 对齐** — [clreq §6.2.2.1](https://www.w3.org/TR/clreq/#x6-2-2-1-necessity-for-line-adjustment)

> "与西文排版不同，中文排版特别是书籍正文排版极少使用左齐右不齐，原则上应该进行两端对齐。"

**10. 行间标点的行距下限** — [clreq §5.6.1](https://www.w3.org/TR/clreq/#x5-6-1-handling-interlinear-punctuation)

> "the line gap for single-sided setting must not be smaller than half of the font size, while the line gap for double-sided setting must not be smaller than 5/8 (=1/2+1/8) of the font size."

即任何情况下 line-gap ≥ 50%（line-height ≥ 1.5），这是第二条独立来源的下限。

**11. jlreq 对照（日文，仅作交叉验证）** — [jlreq](https://www.w3.org/TR/jlreq/)

> "It is common that the line gap for the kihon-hanmen is set to a value between half-em spacing and the one em spacing... one em spacing or close to it is more appropriate when the line length is longer than 35 characters."

> "The best line length (number of characters per line) is around 52 characters, maximum, in vertical writing mode, and 40 characters, maximum, in horizontal writing mode."

与 clreq 的 50%–100% 行距、40/48 字上限一致，互为佐证。jlreq 同样**没有 px / 屏幕数值**。

**12. CSS Text Level 4（可实现层）** — [css-text-4](https://www.w3.org/TR/css-text-4/#inter-script-spacing)

> "The amount of space introduced by these keywords is 1/8 of the CJK advance measure, i.e 0.125ic..."

> Note: "Spacing conventions vary, but values typically range from 1/4ic to as low as 1/8ic, with 1/4ic being more common in historical contexts due to metal type limitations and 1/6ic or thinner being more common in proportional typesetting. Because these spaces are inserted by default..., CSS uses 1/8ic in order to be conservative in its interference."

即：clreq 的 1/4 em 是**上限**，CSS 取 1/8 ic 作为保守默认值——两者不矛盾，实现取 1/8 ic。

**13. WCAG 2.2 SC 1.4.12 Text Spacing（可访问性下限，非设计值）** — [Understanding SC 1.4.12](https://www.w3.org/WAI/WCAG22/Understanding/text-spacing)

> "Line height (line spacing) to at least 1.5 times the font size; Spacing following paragraphs to at least 2 times the font size; Letter spacing (tracking) to at least 0.12 times the font size; Word spacing to at least 0.16 times the font size."

规范义 Note 1 明确：「Content is not required to use these text spacing values」——它是**用户覆盖样式后不得丢失内容**的容错要求，
本项目的实际取值必须保证在用户注入这些值后布局不塌（即不要写死行高容器、不要 `overflow:hidden` 裁切）。

### B. 业界实践（实测取值，全部读自公开 CSS 或官方文档）

**1. 微信读书（weread.qq.com）** — 22 个 CSS chunk（阅读器正文段实读）

- 正文段：`margin-bottom:1em; font-size:21px; line-height:40px; text-indent:0!important`（默认档 **21/40 = 1.905**，落在 clreq 50%–100% 中上段）
- 7 档字号（`.fontLevel1..7`）：18/35、21/40、24/46、28/54、32/61、36/69、42/80
- 阅读宽：`.app_content{max-width:1000px}` + `.readerChapterContent{margin:0 100px}` ⇒ 正文宽 **800px**，
  21px 下约 **38 字/行**（正落在 clreq 17–40 上沿）
- 正文 `text-align:justify`（**两端对齐**），`text-indent:0`（**不用首行缩进**），段间距 `1em`
- 字体：`PingFang SC` 系黑体栈出现 130 次；`SourceHanSerifCN-Bold` 70 次但只用于书名/封面/书评标题等短文本（20px、28px 档）⇒ **正文黑体、宋体做短标题**
- 章节标题用 `em`：`h1{font-size:1.6em}`、`h2{1.5em}`、`h3{1.4em}`（比例远小于 clreq 的 10–20%，是 App 内嵌阅读的克制做法）

**2. 掘金（juejin.cn）** — `xitu_juejin_web/11.3ae6a9f.css`

- `.markdown-body{font-size:16px;line-height:1.75}`（**16/28**）
- `.markdown-body p{margin:22px 0; text-align:left}`（**段间距 1.375em**、**左对齐**）
- 标题 `h1 24 / h2 20 / h3 18 / h4-h6 16`，统一 `line-height:1.5`
- 正文容器 `.entry-view{max-width:600px}` ⇒ 16px 下 **37.5 字/行**

**3. 少数派（sspai.com）** — `post.sspai.com/post.15ed451c.css`（需 UA + Referer）

- 变量：`--size-ArticleBody:15px; --line-height-ArticleBody:1.8`（**15/27**）、
  `--size-ArticleH2:24px; --line-height-ArticleH2:1.4`、`--size-ArticleH3:20px; ...1.4`、
  `--space-ArticleElement:24px`
- 文章容器 `.article__section__wrapper{max-width:728px}`（15px 下约 **48.5 字**，正好踩 clreq 48 字上限）
- 标题 `.article__title{font-size:30px;font-weight:500;line-height:1.25}`（**500 而非 700**：纸感/克制字重的现成范本）
- 全文 `text-indent` 命中 0、无衬线字体关键字 ⇒ 黑体 + 零缩进

**4. 端传媒（theinitium.com）** — `assets/built/app-j-_zm9S9.css`（226KB 实读）

- **字体分工反例**：`--font-heading: var(--font-serif)`、`--font-body: var(--font-sans)`；
  `h1..h6{font-family:var(--font-heading)}` / `body,article.ghost-content{font-family:var(--font-body)}`
  ⇒ **标题宋、正文黑**，且真备 Noto Serif SC/TC 衬线栈
- 行高令牌：`--leading-body:1.5; --leading-heading:1.4; --leading-display:1.3; --leading-paragraph:2`
- measure：`--container-content:680px`，`.ghost-content{max-width:var(--container-content); line-height:2}`
- 正文 `.prose-lg{font-size:1.125rem; line-height:1.77778}`（**18/32**）、`p{margin:1.33333em 0}`（**24px**）
- `text-indent` 全文仅 1 处且为 `0`

**5. 澎湃（thepaper.cn）** — `_next/static/css/pages/_app.ee3d253b.css`

- 正文 `font-family: PingFangSC-Regular, Microsoft Yahei, Hiragino Sans GB, sans-serif !important`（黑体），
  另有 `SimSun` 仅用于局部 ⇒ 传统媒体同样是「黑体正文」

**6. typo.css（中文网页排版基线库）** — https://github.com/sofish/typo.css

- `.typo{font-size:1rem; line-height:1.75; line-break:strict; hanging-punctuation:allow-end; text-autospace:ideograph-alpha ideograph-numeric; text-spacing-trim:normal}`
- `.typo-readable{max-width:48em}`（其文档自述直接对应 clreq「横排最大 48 字」，并把 1.75 解释为「75% line gap」）
- 标题 `font-weight:600; line-height:1.35; letter-spacing:0; text-wrap:balance`
- `.typo-justify{text-align:justify}` 需**显式 opt-in**（默认不对齐，承认实践分歧）

**7. Ant Design（中文 UI 字阶的事实标准）** — https://ant.design/docs/spec/font-cn

- 「主字体 14，与之对应的行高为 22」；「字阶的选择尽量控制在 3-5 种之间」；「只出现 regular 与 medium（400/500）」
- 源码侧公式（`components/theme/themes/shared/genFontSizes.ts`）：`getLineHeight = (fontSize) => (fontSize + 8) / fontSize`
  ⇒ 行距恒等 8px：14→22、16→24、12→20、20→28、28→36。**这套「固定 8px 行距」逻辑与中文长文无关**，
  现有令牌的 15/22、12/16 也不符合它——说明当前字级是历史拼接，没有统一推导。

**8. 其他一手对照**

- Windows / Fluent type ramp（https://learn.microsoft.com/en-us/windows/apps/design/style/typography）：
  `Text 14/20`、`Text 18/24`、`Display semibold 20/28 / 28/36`；「Bold and Italic styles are not part of the Windows type ramp. **Use Semibold instead of Bold** for emphasis.」；
  「**Serif fonts are good for presenting large amounts of text**」「Sans-serif fonts are a great choice for headings and UI elements」；
  每行 50–60 letters（西文口径）；「Microsoft YaHei UI」为简中 UI 字体、「SimSun Regular — **A legacy Chinese UI font**」
- Apple HIG（https://developer.apple.com/design/human-interface-guidelines/typography）：
  「**In general, avoid light font weights**... especially when text is small」；平台最小字号 iOS 11pt / macOS 10pt。
  **该页对中文字体与中文排版零覆盖**（`Chinese`/`PingFang` 命中 0）
- Material 3 typescale（`material-web` tokens）：body-large 16/24 tracking +0.5px、body-medium 14/20、body-small 12/16；
  **小字号 tracking 为正、大字号为 0，从不用负字距**
- Butterick's Practical Typography（https://practicaltypography.com/summary-of-key-rules.html）：
  「Point size... **15–25 pixels on the web**」；「Line spacing should be **120–145%** of the point size」；
  「average line length **45–90 characters**」；
  「Use first-line indents...**or** use 4–10 points of space between paragraphs. **Don't use both.**」（缩进与段距二选一）
- 中文文案排版指北（https://github.com/sparanoid/chinese-copywriting-guidelines）：
  「中英文之間需要增加空格」「中文與數字之間需要增加空格」，度數/百分比例外 ⇒ 社区惯例层面的中西混排依据
- 维基百科简中站 `ext.gadget.text-spacing`：`:root{text-autospace:normal}` ⇒ 大型中文站点已默认开启自动中西间距
- Adobe Source Serif 4（https://blog.adobe.com/en/publish/2021/03/04/source-serif-gets-optical-sizes）：
  五个 optical size（Caption / Small Text / Text / Subhead / Display），Google Fonts 元数据 `opsz` 轴 **8–60，默认 14**
- Noto Serif SC / Noto Sans SC（Google Fonts 元数据）：**只有 `wght` 轴，没有 `opsz`** ⇒ 中文衬线无小字号专用字面
- Tailwind CSS v4 `theme.css`：`--font-serif: ui-serif, Georgia, Cambria, 'Times New Roman', Times, serif` —— **默认衬线栈里没有中文字体**，中文宋体必须自己声明

### C. 浏览器可用性（决定哪些规则能当主手段） — MDN browser-compat-data

| 特性 | Chrome | Firefox | Safari | 结论 |
| --- | --- | --- | --- | --- |
| `line-break` | 58 | 69 | 11 | 可直接用于禁则 |
| `text-autospace` | 140 | 145 | 18.4 | Baseline 2025，可用作主手段（v4 项目面向现代浏览器） |
| `text-spacing-trim` | 123 | ✗ | ✗ | **仅 Chromium**，只能渐进增强 |
| `text-justify: inter-character` | 145 | 55 | ✗ | 需回落 |
| `text-wrap: balance/pretty` | 114 | 121 | 17.4 | 可用于标题/正文 |
| `hanging-punctuation` | ✗ | ✗ | 26.5 | 只能当装饰，不可依赖 |
| `font-variant-east-asian` | 63 | 34 | 9.1 | 可用 |

⇒ **标点挤压不能作为唯一手段**：`text-spacing-trim` 在 Firefox/Safari 不生效，
需要接受「Chrome 上更精致」的渐进增强，或未来在内容层做标点预压缩（成本高，不建议）。

### D. 仓内现状（本次实测，作为落地基线）

- 任意字号：`text-[Npx]` 191 处 / 16 种（`13/15/18/16/20/14/17/22/28/11/10/8/42/34/32/24px`），
  全部任意值 309 处——问题不在「多一个档位」，而在**没有可用的档位**。
- 令牌行距：`--text-body 15/22`（gap 46.7%，低于 clreq 50% 下限）、
  `--text-caption 12/16`（gap 33.3%，远低于下限，只适合纯 UI 标签）。
- 长文实际用的 `text-subhead leading-7` = **14/28（gap 100%，顶格）**，
  且 `--text-subhead` 自带 `+0.01em` 字距（与 clreq 密排冲突，14px 上正字距影响可忽略但应清理）。
- 标题负字距 −0.02em / −0.01em：对中文是「紧排」，clreq 明确禁止用于标题。
- 容器 `max-w-6xl` = 1152px（72 字/行 @16px），是 clreq 上限的 1.5 倍。
- 无 `--font-serif`：DESIGN.md:75 与 `style.css:62` 的字体栈全为无衬线，衬线世界替换需新增。

### E. 取证方法（可复用教训）

- **clreq 与 jlreq 都没有 px / 屏幕数值**，不要去找「clreq 推荐 16px」——不存在；
  px 只能由 pt 号数表 ×4/3 与行距百分比推导，写文档时**必须标注「推导」**，否则就是编数值。
- 中文产品的真实取值只能从**公开 CSS 文件**读：正文排版规则常在按需加载的 chunk 里，
  首页 CSS 往往只有 UI 样式（微信读书、语雀都是这样）。语雀正文样式在独立渲染器包里，未取到。
- 抓取易失败：知乎全站 403/CAPTCHA；Apple HIG 是 JS shell（需 `r.jina.ai` 代理）；
  m3.material.io 静态页只有 "This website requires JavaScript."；
  sspai 的 CSS 需带 `Referer: https://sspai.com/` 才返回 200。
- **权威数值优先读源码而非文档页**：Material 3 的 typescale 与 AntD 的行高公式都来自仓库源码，
  文档页要么需 JS 渲染、要么与实现不同步。

## 待定与风险

以下四项需要用户/设计拍板，且**前三项直接决定 `## 结论` 表格是否按原样落地**。

### 1. 正文用宋体还是黑体（议题第 1 题，无定论）

| | 正文宋体（衬线） | 正文黑体（无衬线） |
| --- | --- | --- |
| 规范依据 | clreq：「Song is currently the most common typeface style used in Chinese printing」「普遍使用于内文文字」 | clreq：「受到万维网、数字出版的影响，也有少数书籍开始使用黑体作为内文字体」——是「少数」 |
| 纸感 | 直接对应纸书体例；「编辑版面」世界的字面来源 | 纸感靠版心/行距/规则线承担，字面偏「屏幕」 |
| 长文耐力 | 笔画对比带来字行方向感（阅读时字行更像一条线） | 灰度均匀、抗糊；小字号与低分屏更稳 |
| 小字号风险 | **高**：思源宋体无 opsz 轴，小字号细笔画易糊（见 §6 推理） | 低 |
| 业界 | 端传媒（标题宋、正文黑）、微信读书（书名宋、正文黑）——**没有找到正文宋体的大厂中文产品** | 微信读书、掘金、少数派、澎湃正文均为黑体 |
| 字体可得性 | macOS `Songti SC`、Windows `SimSun`（微软标记为 legacy）、可自托管 Noto Serif SC（可变字重 200–900） | 系统栈充足 |

两个可行方案：
- **A（保守，推荐先做）**：正文黑体、**标题与引用块宋体**——即端传媒方案，风险最低，纸感由标题与版心承担。
- **B（激进，纸感最强）**：正文宋体，但正文不得小于 16px，且必须自带 Noto Serif SC 兜底（Windows 的 SimSun 在 16px 屏幕上观感差）。

若选 B，需接受：移动端低分屏中文宋体观感不稳定；`font-weight: 600` 在只有 Regular/Bold 两档的
`Songti SC`/`SimSun` 上会触发浏览器合成加粗（字形变胖发虚），需显式限定 400/700 或自托管可变字体。

### 2. 首行缩进 2em 还是段间距 1em（纸感与屏幕惯例的正面冲突）

clreq 的书籍标准是「2em 缩进 + 段落间零行距」，而屏幕产品（微信读书、少数派、端传媒、掘金）
**一致选择「零缩进 + 段间距」**。Butterick 明确二者不可并用。
选 2em 缩进能立刻获得「纸书」辨识度，代价是：知识库正文常由 Markdown 渲染，
段落不长、含大量列表/引用/代码块时，缩进会与这些块元素打架（clreq 自己也承认第④种「语气未完段落不缩排」的例外）。
**建议**：正文用段间距；若一定要纸感，只在「长文阅读模式」里开缩进，且列表/引用/代码块内不缩进。

### 3. 两端对齐还是左对齐

clreq 说「原则上应该进行两端对齐」，但实践分裂：微信读书 justify、掘金/Windows/typo.css 默认左对齐。
中文两端对齐的风险小于西文（汉字等宽、不会出现词间大洞），但**混排大量英文/URL/代码时会产生难看的间隙**。
且 `text-justify: inter-character` 在 Safari 不支持（Chrome 145 / Firefox 55 才有）。
**建议**：纯中文段落 justify，含代码/URL 的块级元素与移动端回落左对齐。

### 4. 未查实的地方（写在这里，避免后续被当成事实）

- **没有一手来源给出「宋体低于 X px 不可用」的数值**。本文的 16px 边界是推理 + 实践，不是规范。
- **Material Design 与 Apple HIG 都没有中文（CJK）排版指引**：Material 的 typescale 只有西文 tracking；
  Apple HIG typography 页对中文字体零覆盖。所以「大厂规范里的中文数值」这一路取证是空的，
  中文数值只能来自 clreq + 具体产品 CSS。
- 微信读书**正文的宋体开关默认值未确认**（阅读器设置里有「宋体/黑体」，默认档推测为黑体，未取到证据）。
- 语雀、Notion 中文、flomo 的正文排版取值**未取到**：语雀正文样式在独立 doc 渲染器包里，
  Notion 的正文样式是运行时注入，flomo 首页无外链 CSS。
- 中文衬线自托管方案（子集化后的 Noto Serif SC 体积、FOUT 影响）未评估，属实现阶段成本。
- `text-spacing-trim` 的挤压效果与 clreq「相邻双标点 1.5 字宽」不完全等价（CSS 处理的是标点内侧留白），
  两者对应关系本文未逐条核对。

### 5. 与现状的迁移风险

- 把正文从 14px/28px 抬到 16px/28px，同屏信息量下降约 12%，需确认分块页/对比页的版式不会因此碎裂。
- 收窄容器（1152px → 640px）改动面在 `KnowledgeDetail.vue:130` 与 `KnowledgeChunks.vue:96` 两处，
  但内部依赖 `max-w-6xl` 做栅格的子元素（表格、chunk 对照双栏）需要单独处理——
  双栏对照本身就该突破 measure，应显式使用 `--measure-wide` 而非继承正文宽度。
- 移除标题负字距后，现有 28px/34px 标题会因为失去「收紧」而在视觉上略松，需同步重测行高，
  这正是本文把标题行高定到 1.35–1.4（而非现状 34/28 = 1.21）的原因。
