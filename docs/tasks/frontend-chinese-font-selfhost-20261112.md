---
task_id: frontend-chinese-font-selfhost-20261112
status: completed
created: 2026-11-12
last_reviewed: 2026-11-12
applicable_to: "前端字体选型与自托管、中文字体子集化、Vite 字体资源接入"
---

# 中文字体自托管可行性（GitHub #27）

## 背景

「纸感编辑版面（Editorial Paper）」这个设计声音的核心之一是**中文衬线标题**，而当前 `apps/zhixu-frontend/index.html` 第 6-7 行 preconnect、第 9 行外链 Google Fonts 的 Inter、第 13 行外链 Material Symbols，断网即失效；字体栈（`DESIGN.md:74-75`）以 `-apple-system` / `'PingFang SC'` 打头，在 Linux 与 Android 上直接落空。DESIGN.md v2 要替换整套字体栈，必须先钉死三件事实：**中文字体能否合法自托管并子集化、落地方式与体积代价、加载失败时四端各自退到什么字体**。

本文是 DESIGN.md v2 的**前置事实**，只回答事实与代价，不做视觉决策。所有体积数字都是本会话用 `fontTools 4.68` 在自己机器上实测的，测量条件写在各条旁边；凡是没查到一手来源的，一律在「待定与风险」里标出，不作推断。

## 结论

**一句话定案**：自托管**只做中文衬线**——Noto Serif SC 静态 600/700，每个字重切两片（Tier 1 = 687 字首屏用，Tier 2 = 剩余 6249 字），首屏 **2 个请求 / 271.8 KB**，全量 **4 个请求 / 2.58 MB**；正文黑体交给四端系统字体（苹方 / 微软雅黑 / Noto Sans CJK），备选标题字体是霞鹜文楷。同时删掉 Google Fonts 外链与那 3.84 MB 的 Material Symbols 字体，改用本地图标子集。合法性上 Noto / 霞鹜都是 OFL 1.1 且允许子集与网页嵌入；**京華老宋体禁止修改，不能自托管**。

### 1. 选型：标题自托管 Noto Serif SC（备选霞鹜文楷），正文交给系统黑体

**定案：只自托管中文衬线标题，正文交给系统黑体。**

| 位置 | 定案 | 备选 | 关键理由 |
|---|---|---|---|
| 标题（中文衬线） | **Noto Serif SC**（自托管；取 `google/fonts` 的 glyf 可变字体，实例化成静态 600/700） | **霞鹜文楷 LXGW WenKai**（OFL，楷体味更重） | 必须自托管：Android 无中文衬线系统字体（第 4 节）。OFL 1.1 且**无 Reserved Font Name**，子集可沿用原名；glyf 轮廓子集比 CFF 版小 2–2.3 倍 |
| 正文（中文黑体） | **系统黑体**（零下载：苹方 / 微软雅黑 / Noto Sans CJK，三端内置） | **Noto Sans SC** 自托管（仅当要求四端正文像素一致） | 自托管收益最低：四端里三端都内置黑体，只有 Linux 桌面不保证 |
| 西文（可选） | **Inter** 变量字体 latin 子集，自托管 **48256 B（47.1 KB）** | 干脆不引入，交给系统 UI 字体 | 与现在外链的 Inter latin 是同一份数据，自托管只多 47 KB |

**明确不要用**：

- `notofonts/noto-cjk` 仓库的 CFF OTF / CFF2 可变字体 —— 同一字符集下子集比 glyf 版大 2–2.3 倍（数据见依据）。
- **京華老宋体** —— 作者授权原文写死「本字体非开源字体，请勿随意修改本字体文件，更不许传播修改版文件」，子集化就是修改，**不可自托管**。
- **汇文明朝体** 只能当纯装饰备选：字形古雅但仅 14070 字形、缺 GB2312 常用字，且原作者已发新版、修正版仓大概率停更。
- `fontmin` —— 只产 woff/eot/svg，**不产 woff2**，中文场景不适用。

### 2. 落地方式：静态字重 + **只切两片**的 `unicode-range` 子集（不是 101 片，也不是可变字体）

**一句话**：每个字重出 2 个 woff2 —— `Tier 1`（首屏就能用到的字）和 `Tier 2`（剩余常用汉字），用 `unicode-range` 把两者挂到同一个 `font-family` 名下，浏览器自动做到「首屏只下 Tier 1，遇到生字再补 Tier 2」。

四个实测结论支撑这个选择：

1. **`unicode-range` 切片本身是对的，但切片粒度必须极粗。** MDN 原文「用到一个字就整片下载」——片越小越省，但片数一多，请求数和重复表头把收益吃光。三个口径下的实测（serif 600 字重）见本节末尾的表。
2. **可变字体在中文上是负收益。** VF 全轴单文件是静态单字重的 **1.9 倍**（828 字：278.8 KB vs 144.4 KB）；哪怕把轴收窄到 400–700 仍是 1.88 倍；收窄后按 GB2312 全量算，VF 一个文件 2466040 B，而两个静态字重合计 2675772 B ——看起来 VF 略省，但它**首屏必须整包下 2.41 MB**，而两片静态方案首屏只要 135 KB。中文场景 VF 是「用首屏换总包」，不值。
3. **两层切分是必须的，因为产品里用户文档是任意 Markdown。** `apps/zhixu-frontend/src/utils/markdown.ts` 用 markdown-it 渲染，`apps/zhixu-frontend/src/pages/KnowledgeDetail.vue:269` 用 `v-html` 直出，`apps/zhixu-frontend/src/style.css:192-216` 又给 `.knowledge-markdown h1/h2/h3` 套了 `text-title-1/text-title-2/text-headline` —— **用户自己写的文档标题会吃衬线字体**，字符集完全不可预知。所以字体不能只覆盖 UI 文案。
4. **字集取 GB2312 全集 6763 字，不取一级 3755 字。** 一级字对仓内 84 个 md 的 1184 个唯一汉字覆盖 1176（99.3%），全集覆盖 1182（99.8%，缺「標」「題」）；但更重要的是 GB2312 全集才覆盖日常中文写作，一级字会频繁掉字。GBK 全集（21895 字）代价约 3 倍，不建议。

**三种子集化方案在三个字符集口径下的实测成本**（serif 600 字重，测法：解析各方案 CSS 的 `unicode-range` 后逐片累加 woff2 字节；来源分别是 `nssc400.css`+`nssc_sizes.txt`、`cnfs_glyf/static400/result.css`、以及本项目手工两片的子集产物）：

| 要覆盖的字符集 | 手工两片（本方案） | Google Fonts 101 片 | cn-font-split 默认 221 片 |
|---|---|---|---|
| 首屏 UI 字集 687 字 | **1 请求 / 135.2 KB** | 29 片 / 928.7 KB | 34 片 / 948.6 KB |
| 仓内 828 字 | 2 请求 / **1309.4 KB**（T1 135.2 + T2 1174.2） | 21 片 / 831.3 KB | 29 片 / 1062.0 KB |
| GB2312 全集 6934 字 | 2 请求 / **1309.4 KB**（同上，T2 一次吃下 6934 字） | 93 片 / 3067.5 KB | 55 片 / 2029.3 KB |

→ **首屏这一栏是决定性的**：手工两片 **1 个请求 / 135.2 KB**，另外两条路要 29～34 个请求、约 930～950 KB，**贵 7 倍**。中间口径（828 字）手工两片反而比 GF 大，因为 T2 是按 6934 字付的钱不是按 828 字；但一旦目标改成「覆盖 GB2312 全集」（产品真实需求：用户文档是任意中文），手工两片 1309.4 KB 只有 GF 93 片 3067.5 KB 的 **43%**、cn-font-split 55 片 2029.3 KB 的 **64%**，请求数从 55～93 降到 2。

**具体字符集**：`Tier 1` = UI 字集 526 字（`.vue` template 去重 CJK）+ 中文标点 + 常用符号 + ASCII = **687 字**，`unicode-range` 508 段；`Tier 2` = GB2312 全集 + 标点 + 符号 + ASCII 去掉 Tier 1 = **6249 字**，3536 段。

**为什么不直接用 `cn-font-split`**：它是中文字体子集化的事实标准（Apache-2.0，最新 7.4.3，自动生成带 `unicode-range` 的 CSS 与 `local()` 优先项），但它解决的是「几万字的字体怎么切成几百片让人按需拉」的问题；本场景字集已经先砍到 6934 字，再切碎反而变贵（全量 221 片 / 8.32 MB vs 单文件 1.29 MB）。另外它依赖 `koffi@2.10.0` 原生模块，在 `Dockerfile` 的 `node:22-alpine`/musl 上有装不上的风险。

**落地管线（最小）**：`fontTools.subset` 一条命令 × **4 个产物**（衬线 600/700 × Tier1/Tier2）；若再加装正文黑体则是 8 个，产物放 `apps/zhixu-frontend/src/assets/fonts/`，CSS 里 `url()` 引用由 Vite 自动接管（自动识别为 asset、进 build graph、加 hash，`build.assetsInlineLimit` 默认 4096 B 远小于字体文件所以不会被内联）。

```bash
# 从 google/fonts 下载 glyf VF，实例化出静态字重
python -c "from fontTools.ttLib import TTFont; from fontTools.varLib.instancer import instantiateVariableFont as i; \
f=TTFont('NotoSerifSC[wght].ttf'); i(f,{'wght':600},inplace=True,updateFontNames=True); f.save('NotoSerifSC-600.ttf')"

# 子集化（每个字重 × 每层跑一次）
python -m fontTools.subset NotoSerifSC-600.ttf --text-file=tier1.txt \
  --flavor=woff2 --layout-features='*' --no-hinting --name-IDs='*' \
  --output-file=serif-600-t1.woff2
```

**OFL 合规必须做的两件事**：命令里加 `--name-IDs='*'`（**不加的话子集产物的 nameID13/14 的 OFL 声明会被删掉**，代价仅 +564～684 字节），以及在仓库内附 `OFL.txt`（OFL 条件 2 要求「每份副本携带版权声明与本许可」）。注意 Noto 的 `OFL.txt` 版权行是 `Copyright 2012 Google Inc. All Rights Reserved.`，与字体 name 表里的 `(c) 2017-2024 Adobe` 不是同一行，**两个都要留**。

### 3. 体积预算（fontTools 4.68，`--flavor=woff2 --layout-features='*' --no-hinting --name-IDs='*'`，输入为 google/fonts glyf VF 实例化出的静态字重）

**主推方案（定案）：只自托管衬线，2 个字重 × 2 层 = 4 个文件**

理由：第 4 节的判断是「衬线必须自托管、黑体不必」，所以体积预算也只花在衬线上。

| 层 | 文件 | 字节 | KB | 何时下载 |
|---|---|---|---|---|
| Tier 1 | Serif 600（标题，`text-title-3` / `text-headline`） | 138428 | 135.2 | 首屏 |
| Tier 1 | Serif 700（标题，`text-display` / `title-1` / `title-2`） | 139896 | 136.6 | 首屏 |
| **首屏合计** | **2 个请求** | **278324** | **271.8** | — |
| Tier 2 | Serif 600 剩余 6249 字 | 1202368 | 1174.2 | 用户文档出现 Tier 1 之外的字时 |
| Tier 2 | Serif 700 剩余 | 1225488 | 1196.8 | 同上 |
| **全量合计** | **4 个请求** | **2706180** | **≈ 2.58 MB** | 衬线全覆盖任意中文长文 |

**可选加装：正文黑体也自托管（2 字重 × 2 层 = 再 4 个文件）**

| 层 | 文件 | 字节 | KB |
|---|---|---|---|
| Tier 1 | Sans 400 | 104120 | 101.7 |
| Tier 1 | Sans 600（仓内 `font-semibold` 用了 147 处） | 105380 | 102.9 |
| Tier 2 | Sans 400 剩余 | 903700 | 882.5 |
| Tier 2 | Sans 600 剩余 | 921544 | 899.9 |

加装后：首屏 4 个请求 / **487824 B（476.4 KB）**，全量 8 个请求 / **4740924 B（≈4.52 MB）**。**只有当你要求「四端正文像素一致」时才加这一层**；否则正文交给系统黑体，首屏省 **204.6 KB**、全量省 **2034.7 KB**。

**还可以再砍**（按性价比排序）：

- **砍 Serif 600（省 135.2 KB 首屏 + 1174.2 KB Tier 2）**：把 `text-title-3` / `text-headline` 的 600 改成 700，标题只用一档字重 → 首屏降到 **136.6 KB**，全量 **1333.4 KB（约 1.30 MB）**。
- **只想「本仓库文档够用」**（不接受用户任意长文）：Tier 1 再砍到 UI 526 字 → Serif 600 子集 **94620 B（92.4 KB）**、Serif 700 **96052 B（93.8 KB）**，首屏 2 文件约 **186.2 KB**，Tier 2 完全不要。

**对照：单文件不切片**（1 个请求吃下整个字符集，无 `unicode-range`）

| 口径 | Serif 600 | Serif 700 | （加装）Sans 400 | （加装）Sans 600 |
|---|---|---|---|---|
| GB2312 一级 3755 + 标点 + 符号（3926 字） | 741648（724.3 KB） | 753972（736.3 KB） | 564136（550.9 KB） | 574908（561.4 KB） |
| GB2312 全 + 标点 + 符号（6934 字） | 1325420（1294.4 KB） | 1350352（1318.7 KB） | 996520（973.2 KB） | 1015628（991.8 KB） |

→ 单文件方案首屏就要吞 **724 KB～1.29 MB**（且是一个请求一个文件、必须整下），对比两片方案的 Tier 1 **135.2 KB**，差 **5.4–9.6 倍**。**这就是必须切片的原因**。

**请求数对比**：

| 方案 | 首屏请求数 | 首屏字节 | 全量字节 |
|---|---|---|---|
| 现状（Google Fonts 外链） | 2 | 48 KB（Inter latin）+ 3.84 MB（Material Symbols）= **≈ 3.89 MB** | 同左 |
| **本次定案**（2 字重 × 2 层，只衬线） | 2 | **271.8 KB** | ≈ 2.58 MB |
| 加装正文自托管（4 字重 × 2 层） | 4（+ 可选 Inter 1 个） | 476.4 KB | ≈ 4.52 MB |
| 单文件不切片（加装正文，4 字重 × GB2312 全） | 4 | **4.47 MB** | 同左 |
| cn-font-split 默认切片（glyf 源，221 片） | 34（687 字口径）/ 29（828 字口径） | 948.6 KB / 1062.0 KB | 8.32 MB（全量 221 片） |
| Google Fonts 101 片（如果坚持外链） | 29（687 字口径）/ 21（828 字口径） | 928.7 KB / 831.3 KB | 3.03 MB（101 片全下，但只覆盖 15609 码位、缺 GBK 8660 字与 Ext-B） |

→ **移除 Material Symbols 那 3.84 MB 之后，即使按最保守的「4 字重 × 2 层全量 4.52 MB」算，字体总量相对现状也不增加；按定案的「只衬线 2.58 MB」算，甚至比现状还少 1.3 MB，而且彻底脱网。**

**构建复杂度**：**4 条** `fontTools.subset` 命令（每条一个 字重×层；加装正文则 8 条）+ 一次性下载源字体，没有运行时依赖、没有 Node 原生模块。脚本放 `apps/zhixu-frontend/scripts/`（目录目前不存在，要新建），`package.json` 加一条 script。**代价是「子集是快照」**：UI 新增中文文案不会自动进 Tier 1，需要重跑；不过因为 Tier 2 兜着，漏字最多是多下 1～2 个文件，不会白框。

**必须同时改的部署配置**：`apps/zhixu-frontend/nginx.conf`（33 行）**全文没有任何 `expires` / `Cache-Control`**，自托管字体默认没有长缓存，等于白做自托管。要加：

```nginx
location ~* \.(woff2)$ {
    expires 1y;
    add_header Cache-Control "public, immutable";
}
```

`gzip_types`（`nginx.conf:11`）不含 `font/woff2` 是**对的**——woff2 本身已 brotli 压缩，实测 `T1serif700.woff2` 138360 B 经 `gzip -9` 变成 138440 B，反而变大。另外 nginx 官方 `mime.types` 已含 `font/woff2 woff2;`，不需要额外配 MIME。

### 4. 四端回退栈（可直接粘）

**判断先摆前面（这是本题最关键的结论）：**

- **中文衬线标题必须自托管。** 因为 **Android 上根本没有中文衬线系统字体**可回退，而 Android 是最常见的移动端；macOS/iOS/Windows 虽有系统衬线，但 macOS 是宋体、Windows 是 SimSun，观感与「纸感」声音不是一回事，跨端不一致。
- **中文黑体正文不必自托管。** macOS（含 iOS）有苹方、Windows 有微软雅黑、Android 有 Noto Sans CJK——**四端里的三端都内置且质量可用**；只有 Linux 桌面不保证（取决于是否装了 `fonts-noto-cjk`）。正文自托管只在「求跨端像素一致」时才值，代价是 Tier 1 首屏 +104120 B（101.7 KB）+ 105380 B（102.9 KB）= **204.6 KB**。
- **Linux 桌面的真实处境**：它是唯一「黑体也可能裸奔」的端，但桌面浏览器用户占比低，且 `system-ui` 兜底后仍有 DejaVu 等西文+可能的中文回退。**结论是给 Linux 留一条 `Noto Sans CJK SC` / `Source Han Sans SC` / `WenQuanYi Micro Hei` 的回退链，而不用为它单独自托管**。

| 端 | 中文黑体（正文） | 中文衬线（标题） | 结论 |
|---|---|---|---|
| macOS | `PingFang SC`（系统内置，本机实测 `PingFang.ttc` 78222888 B） | `Songti SC`（系统内置，`/System/Library/Fonts/Supplemental/Songti.ttc`） | 有衬线，可回退 |
| iOS | 同上 | 同上 | 有衬线，可回退 |
| Windows 10/11 | `Microsoft YaHei`（Win11 主清单内置，Msyh.ttc v6.25） | `SimSun`（主清单内置，Simsun.ttc v5.18） | 有衬线，但 SimSun 是点阵味老宋，大字号观感差 |
| Linux 桌面 | 装了 `fonts-noto-cjk` 才有 `Noto Sans CJK SC`，否则可能裸奔 | 同上（`Noto Serif CJK SC`） | 不保证，视发行版 |
| **Android** | `Noto Sans CJK SC`（AOSP `zh-Hans` 家族，index 2） | **无系统字体** | **决定性一端** |

→ **Android 是「衬线必须自托管」的决定性理由**：AOSP 的 `lang="zh-Hans"` 家族 9 档字重全部指向 Noto Sans CJK，段内 0 次 `NotoSerifCJK` 引用，中文衬线在 Android 上根本没有系统字体可以回退。Linux 是第二端（黑体也不保证）。macOS / iOS / Windows 都有系统衬线，可以纯回退。

`ZhiXu Serif` / `ZhiXu Sans` 是本项目自托管 `@font-face` 里自取的 family 名（不沿用上游 `Noto Serif SC`，避免与用户机器上装的正版 Noto 混同、也便于将来换字体源而不改 CSS）。

```css
/* 标题：中文衬线。第一位是自托管 @font-face 的 family 名，
   其后全是「加载失败时」的四端系统衬线回退（顺序：有衬线的先，安卓落到黑体兜底） */
--font-serif: 'ZhiXu Serif',
  'Songti SC', '宋体-简', 'STSong', 'STSongti-SC-Regular',      /* macOS / iOS */
  'SimSun', '宋体', 'NSimSun',                                   /* Windows */
  'Noto Serif CJK SC', 'Source Han Serif SC', 'Noto Serif SC',   /* Linux 若装了 */
  'Microsoft YaHei', 'PingFang SC', system-ui, serif;            /* 安卓没有衬线，只能落黑体 */

/* 正文：中文黑体。macOS/iOS/Windows/Android 均内置；web 字体只为「跨端一致」 */
--font-sans: 'ZhiXu Sans',
  -apple-system, BlinkMacSystemFont,
  'PingFang SC', '苹方-简', 'Hiragino Sans GB', '冬青黑体简体中文', 'Heiti SC',
  'Microsoft YaHei', '微软雅黑',
  'Noto Sans CJK SC', 'Source Han Sans SC', 'Noto Sans SC', 'WenQuanYi Micro Hei',
  system-ui, sans-serif;

/* 西文/数字：Inter latin 子集 47 KB，或直接交给系统 */
--font-latin: 'Inter', -apple-system, 'Segoe UI', Roboto, sans-serif;
```

**`local()` 优先的写法有坑，不要依赖它省流量**：

- W3C CSS Fonts 4 规定 `local()` 只匹配 **PostScript 名或 full name**，**不接受 CSS family 别名**（`local("PingFang SC")` 匹配的是 name 表 nameID6/nameID4，不是 CSS 里那个 family 名），而且「authors should include both of these names」——两种拼写都得给。
- MDN 补充：`local()` 必须写在同一份 `src` 列表的**首位**，且「user agents may ignore user-installed fonts when using `local()`」（防指纹），**所以不能把系统字体命中当成保证**。
- 我们自托管的子集是从上游字体改的，如果沿用 `'Noto Serif SC SemiBold'` 这种上游名去写 `local()`，遇到装了同名正版字体的机器会撞名（上游 google/fonts 的 name1 有命名 bug，各字重都叫 `ExtraLight`）——**干脆不写 `local()`，只用 `font-family` 回退**。

**FOUT 度量对齐（可选优化）**：PingFang SC 的 x-height 600 / cap-height 860，Noto Serif SC 是 x-height 514 / cap-height 729 —— 回退到 PingFang 时字明显更大。按 x-height 对齐 `size-adjust: 85.6%`（514/600），按 cap-height 是 `84.8%`（729/860）。`size-adjust` Baseline 2023-09 起可用。

## 依据

### 授权（这是「能不能合法自托管」的核心）

- **OFL 1.1 首句授权动词就包含 `embed` 与 `modify`**：「to use, study, copy, merge, **embed**, **modify**, redistribute, and sell modified and unmodified copies」——子集化 + 网页嵌入都在授权范围内。<https://openfontlicense.org/open-font-license-official-text/>
- **OFL 条件 2（副本须携带版权声明与本许可）**：「provided that each copy contains the above copyright notice and this license. These can be included either as stand-alone text files, human-readable headers or in the appropriate **machine-readable metadata fields** within text or binary files as long as those fields can be easily viewed by the user.」→ 所以要么在子集里保留 name 表项（`--name-IDs='*'`），要么仓库附 `OFL.txt`。同上链接。
- **OFL 条件 3（Reserved Font Name）**：「No Modified Version of the Font Software may use the Reserved Font Name(s) unless explicit written permission is granted by the corresponding Copyright Holder.」→ **Noto Serif SC / Noto Sans SC 没有 RFN**（`google/fonts/main/ofl/notoserifsc/OFL.txt` 4350 字节，版权行 `Copyright 2012 Google Inc. All Rights Reserved.`，`"Reserved Font Name"` 只出现在定义段，没有声明任何具体名字），所以子集可以沿用原名。**霞鹜文楷相反，有 RFN，但它给了 ADDITIONAL PERMISSION**（见下）。
- **OFL FAQ 1.12 明确回答嵌入子集**：「So can I embed OFL fonts in my document? **Yes, either in full or a subset.** The restrictions regarding font modification and redistribution do not apply, as the font is not intended for use outside the document.」<https://openfontlicense.org/ofl-faq/>
- **Noto CJK 仓库 LICENSE 是纯 OFL 1.1**：<https://github.com/notofonts/noto-cjk/blob/main/Serif/LICENSE>
- **霞鹜文楷的 `OFL.txt` 有 `[ADDITIONAL PERMISSION]`**：保留字体名（霞鹜/霞鶩/落霞孤鹜/落霞孤鶩/LXGW）可用于「Modified Versions **subsetted or converted to other formats (e.g., WOFF/WOFF2) solely for web font delivery**」，条件是这些修改版不得作为可安装桌面字体分发到 Google Fonts 等平台 → **允许自托管子集**。<https://github.com/lxgw/LxgwWenKai/blob/main/OFL.txt>
- **京華老宋体：不可自托管。** 作者授权原文第 3 条：「**本字体非开源字体，请勿随意修改本字体文件，更不许传播修改版文件。**」第 1 条允许「嵌入电子产品、软件应用之中」，但子集化属于「修改」，被第 3 条挡住。<https://github.com/cntrump/imitation_typeface_fonts/blob/master/ReadMe.md>
- **汇文明朝体：可自托管（但停更）。** 修正版仓库 `LICENSE` 是 **CC0 1.0 Universal** 全文：<https://github.com/bosswnx/huiwenmincho-improved/blob/main/LICENSE>；原作者授权五条（可免费用于印刷/显示/嵌入电子文档/设备、可自由转发、可作为套装软件一部分销售、不可单独转卖、可用以开发新字体）**没有「禁止修改」条款**。但该仓 README 开头就写「原作者特里王在知乎发布了新的汇文系列字体，修复了许多 bug。此仓库大概率不再继续更新」。字体内嵌 name0 = `Public Domain`。
- **fontsource 的三个 Noto 包都缺 nameID13**（实测子集前 name13 = `None`，name14 有值），所以走 fontsource 路线必须自行附 `OFL.txt` 才能满足条件 2。<https://www.npmjs.com/package/@fontsource/noto-serif-sc>

### 体积与格式（glyf vs CFF 是选型分水岭）

- **同族 CJK 有两条分发线，必须选 glyf 那条。**
  - `google/fonts` 的 glyf 版：`ofl/notoserifsc/NotoSerifSC[wght].ttf` = **25125512 B（23.96 MB）**，**glyf + gvar** 轮廓，fvar wght 200–900（默认 200），cmap **30928** 字形；`ofl/notosanssc/NotoSansSC[wght].ttf` = **17772300 B（16.95 MB）**，同结构。<https://github.com/google/fonts/tree/main/ofl/notoserifsc>
  - `notofonts/noto-cjk` 的 CFF 版：`Serif/OTF/SimplifiedChinese/NotoSerifCJKsc-Regular.otf` = **23.4 MB**（CFF1，cmap 44777）；`Serif/Variable/OTF/NotoSerifCJKsc-VF.otf` = **52.8 MB**（CFF2）；`Serif/SubsetOTF/SC/` 每字重 10.4–12.1 MB。<https://github.com/notofonts/noto-cjk/tree/main/Serif>
- **glyf 子集比 CFF 子集小 2–2.3 倍**（同字符集、同 fontTools 参数）：仓库 828 字 → glyf **147180 B（143.7 KB）** vs CFF `--desubroutinize` **256.4 KB**；GB2312 一级 3755 字 → glyf **681608 B（665.6 KB）** vs CFF **1291.3 KB**；GB2312 全 6763 字 → glyf **1056932 B（1032.1 KB）** vs CFF **2393.9 KB**。
- **「Noto Serif SC 全量 woff2 约 12 MB」——单文件口径**：`NotoSerifCJKsc-Regular.otf` 用 `--unicodes='*'` 全量子集化（`--flavor=woff2`）得到 **12671328 B = 12.08 MB**（耗时 85 s）。**Google Fonts 那个「3 MB」是另一回事**：101 个切片合计 3182264 B（3.03 MB），因为跨片共享了大量 CFF 子程序；单文件做不到这个量级。
- **可变字体在中文上仍是巨物**（glyf VF 源，fontTools 子集）：

  | 口径 | VF 全轴 200–900 | VF 限轴 400–700 | VF 限轴 600–700 | 静态 600 | 静态 700 |
  |---|---|---|---|---|---|
  | UI 526 字 | 179468（175.3 KB） | 171136（167.1 KB） | 155728（152.1 KB） | 94620（92.4 KB） | 96052（93.8 KB） |
  | GB2312 全（6763–6934 字） | 2442960（2385.7 KB） | 2466040（2408.2 KB） | 2231256（2179.0 KB） | 1325420（1294.4 KB） | 1350352（1318.7 KB） |

  → 限轴之后**总包略省**（2.41 MB vs 两静态 2.60 MB），但**首屏从 135 KB 变成 2408 KB**。中文场景 VF 的价值不在体积（中文字符量太大，gvar delta 摊不薄），只在「连续字重」这个设计功能——本项目字级表只用了 400/500/600/700 四档，不需要连续字重。
- **静态字重由 VF instancer 生成即可**，不必为每个字重下独立源：`fontTools.varLib.instancer.instantiateVariableFont(f, {'wght': 600}, inplace=True, updateFontNames=True)`（产出 `NotoSerifSC-600.ttf` 24578660 B 之类）。
- **`--name-IDs='*'` 是 OFL 合规开关**：不加则子集产物 nameID13/nameID14 = `None`（源字体有值），加后 19 条 name 记录全部保留，代价仅 **+564 B（600）/ +684 B（700）**。

### 子集化与加载方案

- **`unicode-range` 是「用到一个字就整片下载」**：MDN 原文「If the page doesn't use any character in this range, the font is not downloaded; **if it uses at least one, the whole font is downloaded.**」Baseline 2015-07 起。<https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/At-rules/@font-face/unicode-range>
- **实测按需下载成本（三种方案，serif 600 口径）**：
  - UI 字集 687 字：手工两片 → **1 请求 / 135.2 KB**；Google Fonts 101 片 → **29 片 / 928.7 KB**；cn-font-split glyf 默认 221 片 → **34 片 / 948.6 KB**。
  - 仓内 828 字：Google Fonts → **21 片 / 831.3 KB**；cn-font-split → **29 片 / 1062.0 KB**；手工两片 → **2 请求 / 1309.4 KB**（T1 + T2，T2 是按 6934 字付的钱）。
  - GB2312 全集 6934 字：Google Fonts → **93 片 / 3067.5 KB**；cn-font-split → **55 片 / 2029.3 KB**；手工两片 → **2 请求 / 1309.4 KB**。
  → 结论：**首屏与「全集覆盖」两个真实口径下手工两片都显著更优；只有 828 字这个中间口径 GF 更小，但 GF 的覆盖面撑不到 GBK/Ext-B**。上述三栏的测法：用 Python 正则解析各方案 CSS 里每个 `@font-face` 的 `unicode-range`，与目标字符集求交，命中的片按 woff2 实际字节累加。数据源分别是 Google Fonts CSS（`nssc400.css`）+ 各片 content-length（`nssc_sizes.txt`）、`cnfs_glyf/static400/result.css` + 该目录下的 woff2 文件、以及本会话用 fontTools 手工切出的两片产物。
- **Google Fonts 101 片方案的覆盖面**：101 片 `unicode-range` 并集 15609 码位，GB2312 全覆盖、GBK BMP 只覆盖 12242/20902（缺 8660）、**CJK Ext-B 完全不覆盖**；仓库 828 字要 21 片 831.3 KB，GB2312 全集要 **93 片 / 3067.5 KB**。→ 这条路「脱不了网就处处受限」，自托管时没有采用价值。
- **`cn-font-split` 事实标准但本场景不必用**：最新 **7.4.3**、Apache-2.0、`unpackedSize` 6150898、依赖 `koffi@2.10.0`（原生模块）。默认 `chunkSize: 70*1024`、`chunkSizeTolerance: 1*1024`、`maxAllowSubsetsCount: 10`、`css.fontDisplay: 'swap'`，产物多 woff2 + 带 `unicode-range` 的 CSS，且**自动生成 `local()` 优先项**，支持可变字体、有 WASM 版。实测在 glyf 源上默认参数产 **221 片 / 8.32 MB**（687 字需 34 片 / 948.6 KB，828 字需 29 片 / 1062.0 KB，GB2312 全集需 55 片 / 2029.3 KB），在 CFF 源上产 **519 片 / 27.3 MB**（比原 OTF 23.4 MB 还大，因为 CFF 分包无法跨片共享子程序，且每片 name 表 432 B × 519 片 ≈ 224 KB 纯冗余）。<https://github.com/KonghaYao/cn-font-split>
- **`fontmin` 不适用**：1.1.1 / MIT / unpacked 48574 B，描述是「webfont (eot, woff, svg) converter」，**只到 woff，不产 woff2**。<https://www.npmjs.com/package/fontmin>
- **`vite-plugin-font` 是另一条免自建管线的路**：5.1.2 / Apache-2.0 / unpacked 58829 B。两种模式：`full-scale`（预生成全量切片，文档站友好）与「Extremely lightweight optimization」（配 `scanFiles` + import 路径加 `?subsets`，官方称体积约为前者的 **10%**）。产物是纯 CSS、无 runtime 数据、自动转 woff2、自动加 `local()` 适配降 CLS。<https://www.npmjs.com/package/vite-plugin-font>
- **`font-display` 三阶段时序**（block period / swap period / failure period），五值 `auto`/`block`/`swap`/`fallback`/`optional`，Baseline 2020-01 起。<https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/At-rules/@font-face/font-display>
- **`local()` 只匹配 PostScript 名或 full name，不接受 CSS family 别名，两类拼写都要写**：W3C CSS Fonts 4 原文「this string is used to match only the **Postscript name** or the **full font name** in the name table of locally available fonts… **so authors should include both of these names** to assure proper matching across platforms. **Platform substitutions for a given font name must not be used.**」<https://drafts.csswg.org/css-fonts-4/#local-font-fallback>
- **`local()` 必须写在同一份 `src` 列表里，且可能被 UA 忽略**：MDN 原文「When a font is needed, the user agent iterates over the set of references listed using the first one it can successfully activate.」；「If the last `src` descriptor can load a resource and doesn't include a `local()` font, the browser may download external font files and ignore the local version even if there is one available」；「user agents may ignore user-installed fonts when using `local()`」。<https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/At-rules/@font-face/src>
- **`size-adjust` 用于对齐回退字体度量**（缩放 advance/baseline 表），Baseline 2023-09 起，示例即 `src: local("Local Font"); size-adjust: 90%;`。<https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/At-rules/@font-face/size-adjust>

### Vite 与部署

- **字体文件自动识别为 asset、进 build graph、加 hash；CSS 里的 `url()` 同样被接管**：「Common image, media, and font filetypes are detected as assets automatically… Referenced assets are included as part of the build assets graph, will get hashed file names, and can be processed by plugins for optimization.」<https://vite.dev/guide/assets>（页面标注 v8.3.7）
- **`build.assetsInlineLimit` 默认 4096（4 KiB）**，小于此值会被 inline 成 base64——字体文件远大于此，不受影响。<https://vite.dev/config/build-options>
- **`public/` 目录原样拷贝、需根绝对路径引用**（想避 hash 可放这里，但会失去 Vite 的缓存失效能力）。同上 assets 页。
- **nginx 官方 `mime.types` 已含 `font/woff2 woff2;`**，不需要额外配 MIME。<https://raw.githubusercontent.com/nginx/nginx/master/conf/mime.types>

### 四端系统字体

- **Windows 11 官方字体清单**（页面 last updated 2022-06-16）：主清单内置 `Microsoft YaHei` / `Microsoft YaHei UI`（`Msyh.ttc` v6.25）、`SimSun` / `NSimSun`（`Simsun.ttc` v5.18）、`SimSun-ExtB`（`Simsunb.ttf` v5.04）；`KaiTi`（`Simkai.ttf` v5.01）、`SimHei`（`Simhei.ttf` v5.03）、`DengXian`、`FangSong` 位于「Fonts included in **Feature On Demand (FOD)** packages」段，**默认不装**。清单内**没有 Noto**。<https://learn.microsoft.com/en-us/typography/fonts/windows_11_font_list>
- **macOS 实测**（本会话 Darwin 一手）：`PingFang.ttc` 实际位于 `/System/Library/AssetsV2/com_apple_MobileAsset_Font8/86ba2c91f017a3749571a82f2c6d890ac7ffb2fb.asset/AssetData/PingFang.ttc`（**78222888 B**，24 faces，index 3 = `family='PingFang SC' ps='PingFangSC-Regular'`），**不在** `/System/Library/Fonts/`；`Songti.ttc` 在 `/System/Library/Fonts/Supplemental/Songti.ttc`（**66902744 B**，8 faces，含 `Songti SC` Black/Bold/Light/Regular）。`system_profiler SPFontsDataType` 注册 412 个 family，中文相关含 `PingFang SC`、`苹方-简`、`Songti SC`、`宋体-简`、`华文宋体`、`华文黑体`、`Hiragino Sans GB`、`冬青黑体简体中文`、`STSong`。
- **Android AOSP**：`frameworks/base/data/fonts/fonts.xml` 的 `lang="zh-Hans"` 家族 9 档字重（100–900）全部指向 `NotoSansCJK-Regular.ttc` 且 `index="2"`（= SC）；**该段内 0 次 `NotoSerifCJK` 引用**，`fallbackFor="serif"` 只出现在 Ethiopic/Hebrew/Thai/Armenian 等语种段 → 中文衬线无系统字体。注意文件头自标「DEPRECATED: This XML file is no longer a source of the font files installed in the system」→ 见「待定与风险」第 4 条。<https://android.googlesource.com/platform/frameworks/base/+/refs/heads/main/data/fonts/fonts.xml>
- **Debian bookworm `fonts-noto-cjk`** = `1:20220127+repack1-1`，含 Noto Sans/Serif CJK JP/KR/SC/TC + Mono（regular + bold 两档），包 55221.7 kB / 安装后 91003.0 kB；`fonts-noto-cjk-extra` 提供全字重。是否安装取决于发行版与用户操作。<https://packages.debian.org/bookworm/fonts-noto-cjk>

### 顺带发现（同一「离线可用」目标，不属于字体但会一起动）

- **Material Symbols 可变字体 woff2 = 4025760 B（3.84 MB）**，是当前首屏最大的一笔外链下载（`index.html:13` 实测 content-length）。改用 `@iconify/vue/offline`（`@iconify/vue@5.0.0` MIT，exports 有 `"./offline"` → `dist/offline.mjs`，提供 `addCollection`）+ 本地图标子集：前端 105 个唯一图标名 → 子集 JSON **35685 字节，gzip 后 9691 字节**。**这一项省下的 3.84 MB 是字体方案体积预算的主要来源。**
- **仓库有一个已存在的死图标引用**：`apps/zhixu-frontend/src/pages/DashboardPage.vue:86` 的 `material-symbols:insert-drive-file-outline-rounded`，在 `@iconify-json/material-symbols@1.2.99` 的 `icons.json`（16524 图标 + 8177 别名）里不存在，Iconify API 也返回 404（同文件 `description-outline-rounded` 是可用替代）。另有 `apps/zhixu-frontend/src/pages/FilePickup.vue:191/198/206/238` 4 处 `<span class="material-symbols-outlined">` 连字写法，同样依赖那 3.84 MB 字体，离线化必须一并改掉。

## 待定与风险

### 未查实（宁可标着，不许猜）

1. **Apple 官方逐字体的系统版本起点未取到一手**。`developer.apple.com/fonts/system-fonts/` 是 JS 渲染表格，`web_fetch` 只拿到导航外壳，**拿不到「PingFang SC 从哪个版本起内置」这类行**。本文引用的「PingFang SC 自 iOS 9 / OS X 10.11、Songti SC 自 macOS 10.12」来自第三方汇编 <https://wener.me/notes/web/ui/font/glossary>，**不是一手**。若要保证老版本 macOS 的衬线回退，需实测或向 Apple 文档确认。
2. **`Serif/LICENSE` 与 `Sans/LICENSE` 的目录归属未能逐字核对**：`notofonts/noto-cjk` 各目录下的 LICENSE 内容为 OFL 1.1，但仓库根 README 的组织方式（`Serif/`、`Sans/` 各自带 LICENSE）本轮只核对了存在性与文本一致，未比对 git 历史中的替换记录。
3. **汇文明朝体的法律地位是双轨的**：修正版仓 `bosswnx/huiwenmincho-improved` 是 CC0 1.0，但原字体在作者 ReadMe 里是自定义授权（非标准许可），且该仓 README 说原作者已发布新版、此仓大概率停更。**若真要用，须确认采用哪一版，两版条款可能不同。**
4. **AOSP `fonts.xml` 自标 DEPRECATED**，且实际 OEM 设备会用 vendor overlay 覆盖。Android「中文衬线无系统字体」这个结论在 AOSP 一手文件里成立，但**具体设备（尤其国产 ROM）未实测**——如果他们装了 Noto Serif CJK，回退其实能命中。这一条要在真机上补验。
5. **`font-display` 取值未定**：Tier 1 用 `swap` 还是 `optional` 取决于设计师对 FOUT 的容忍度，本文只提供事实（三阶段时序），不做决定。
6. **子集完整性未做端到端验证**：字体覆盖率是本会话用 fontTools 读 cmap 逐字核对的（GB2312 6763 字全覆盖、仓内 md 的非 GB2312 符号 `§ ² é → ↓ ↔ ∈ − ≈ ≠ ≤ ≥ ① ② ③ ④ ─ │ ┌ ┐ └ ┘ ├ ┤ ┬ ┴ ▲ ▶ ◀ ★ ⚠` 已纳入 Tier 1），但**没有在浏览器里跑真实页面对比截图**。「有没有字因为 cmap 有、实际缺轮廓而变豆腐」这类问题只能靠渲染实测发现。
7. **`✅`（U+2705）不在 Noto Serif SC 的 cmap 里**（cmap 30928 实测唯一缺的仓内字符），会落到 emoji 字体。若仓库文档大量使用它，需决定是否换成 `✓`。

### 若用户不接受主选（每条都给替代）

- **不接受 Noto Serif SC（嫌太「正」/太像默认宋体）** → 换 **霞鹜文楷**：OFL 的 ADDITIONAL PERMISSION **明确允许** WOFF2 子集自托管；UI 526 字子集 **116032 B（113.3 KB）**，GB2312 一级 3755 字子集 **859988 B（839.8 KB）**。代价：楷体在正文小字号下可读性差，只能当标题。
- **不接受 Noto Serif SC（嫌太「现代」/想要古籍味）** → 换 **汇文明朝体**（CC0 版）：UI 526 字子集 **303076 B（296.0 KB）**，来源 `Huiwenmincho-improved.woff2` 8245656 B（7.86 MB）。代价：源字体仅 14070 字形、缺 GB2312 常用字（一级就缺「傈」），且停更——**只适合当装饰性标题，不能当正文**。
- **完全不想自托管（放弃中文衬线）** → 标题纯回退系统衬线：macOS/iOS 落 `Songti SC`、Windows 落 `SimSun`、Linux 看有没有装 `fonts-noto-cjk`，**Android 落到黑体**（无衬线可退）。「纸感」声音会只剩暖米白底 + 细规则线，衬线在 Android 上消失——**这是不接受的底线，因为 Android 是最常见的移动端**。
- **不想维护子集脚本** → 直接用 `@fontsource/noto-serif-sc@5.3.1` 的 `chinese-simplified-400.css` 单文件方案：**1509736 B（1474.4 KB）**，cmap 7946（缺 1046 个 GB2312 字，但含全部常用字），零管线、零构建依赖。代价：比自建两片方案首屏（271.8 KB）贵 **5.4 倍**，且必须自己补 `OFL.txt`（包内 nameID13 = None）。
- **想连构建脚本都不写、又要按需加载** → `vite-plugin-font@5.1.2` 的 lightweight 模式（`scanFiles` + `?subsets`），官方称体积约为 full-scale 的 10%；但它是按**代码里静态出现的文案**扫字，扫不到用户 Markdown 里的字，**需要额外兜一层系统字体回退**。

### 落地风险

- **`nginx.conf` 无任何缓存头**：不加 `expires 1y` + `Cache-Control: public, immutable`，每次访问都重新验证，自托管等于白做。这是本次最容易被漏掉的一步。
- **`Dockerfile` 用 `node:22-alpine`**：若改用 `cn-font-split`，其 `koffi` 原生依赖在 musl 上可能装不上；`fontTools` 走 Python 无此风险，但 CI 要额外装 Python + `brotli`（本机实测：Homebrew 的 `pyftsubset` 绑的 python3.11 **没有 brotli**，跑不出 woff2，必须单独建带 brotli 的 venv）。
- **子集是「一次性快照」**：UI 新增中文文案不会自动进 Tier 1。因为 Tier 2 兜着，最坏情况是多下 1～2 个文件，不会出现豆腐；但**新文案会走 Tier 2 的 1.17 MB 延迟下载**，所以改文案后应当重跑脚本（建议挂到 prebuild）。
- **SimSun 回退的观感风险**：Windows 上若自托管字体加载失败，标题落到 `SimSun`——它是带点阵 hinting 的老宋体，大字号下笔画发虚。若要更稳，可在 Windows 上优先 `Microsoft YaHei`（黑体）作衬线回退，但那会丢掉「衬线」这个设计意图，需设计决策。
- **Tier 2 的 `unicode-range` 文本本身有开销**：6249 字展开为 3536 段，纯文本 31296 B（30.6 KB），gzip 后 11797 B（11.5 KB）。4 个字重在同一份 CSS 里各写一遍 → 93933 B / gzip 12896 B（12.6 KB）。这是首屏 CSS 的固定税，可接受，但每多切一层就多一份，**别再多切层**。
