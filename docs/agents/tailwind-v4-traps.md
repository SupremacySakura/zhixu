# Tailwind v4 陷阱清单

本仓库用 Tailwind **v4.2.1**（CSS-first 配置，`@theme` 而非 `tailwind.config.js`）。以下每一条都是实测踩出来的，**不是从文档抄的**。

## 命名空间：写错键名 = 静默不生成

v4 的主题键有固定命名空间。键名写错**不报错、不警告，就是不生成任何工具类**。

| 想要 | 正确的键 | 写错的后果 |
|---|---|---|
| 过渡时长 `duration-*` | `--transition-duration-*` | 写 `--duration-*` → 零工具类 |
| z 序 `z-*` | `--z-index-*` | 写 `--z-*` → 零工具类 |
| 缓动 `ease-*` | `--ease-*` | — |
| 字体 `font-*` | `--font-*` | — |
| 容器宽度 `max-w-*` | `--container-*` | — |
| 圆角 `rounded-*` | `--radius-*` | — |
| 阴影 `shadow-*` | `--shadow-*` | — |

`--spacing` 是**单标量**（0.25rem），没有刻度表。`p-4` = `calc(var(--spacing) * 4)`。

**验证方法**：改完主题后必须编译一次并 grep 产物，确认类真的生成了。见下方"怎么验证"。

## `@layer components` 的类不能被 `@apply`

```
Cannot apply unknown utility class 'cabinet-btn'
```

原因：`@layer components` 里的类是普通 CSS 规则，不是 utility，`@apply` 只认 utility。

**修法**：把骨架类写成 `@utility`。

```css
/* ✗ 报错 */
@layer components {
  .cabinet-btn { padding: 8px 14px; }
}
.btn-primary { @apply cabinet-btn; }

/* ✓ 可行 */
@utility cabinet-btn { padding: 8px 14px; }
.btn-primary { @apply cabinet-btn; }
```

**副作用**：`@utility` 是**按需生成**的——源码里零调用就不产出 CSS。`@layer components` 则始终输出。所以迁移期把类从 `@layer components` 挪到 `@utility` 后，产物 CSS 会变小（本次实测 20,924 → 18,186 字符），**这不是 bug**。

## 默认过渡可以被 `@theme` 覆写

`--default-transition-duration` 与 `--default-transition-timing-function` 可以在 `@theme` 里覆写，且**支持变量间接引用**：

```css
@theme {
  --default-transition-duration: var(--transition-duration-micro);  /* 90ms */
  --default-transition-timing-function: var(--ease-settle);
}
```

覆写后，所有**不带** `duration-*` / `ease-*` 的 `transition-*` 自动落到该值。

**这条改变了处置策略**：如果新世界的标准时长就是默认值，那存量代码里的 `duration-150` 应该**删除**而不是换算成 `duration-standard`——删掉就自动继承新默认值，多写一个类反而是冗余。（本次实测：103 处 `duration-150` 全部删除，产物落到 90ms。）

## 默认阴影档位可以整体置 none

`--shadow-2xs/xs/sm/md/lg/xl/2xl` 七档在 `@theme` 里全置 `none` 后，`shadow-md` 之类的产物变成 `--tw-shadow: none`。这比逐处删 `shadow-*` 类安全——留着类名但视觉上无阴影，迁移期不会因为漏删而突然出现框架阴影。

## 变体前缀

- `pointer-coarse:` 是**内建变体**（`@media (pointer: coarse)`），不需要插件
- 任意变体可以用 `[&>*]:` 这类写法，解析时要按层剥离前缀才能拿到真实类名
- 断点变体 `sm:` / `md:` / `lg:` 同样是前缀，做静态检查时要单独处理

## 怎么验证（每次改主题必做）

`npx` 在本机不可用。用仓库自带的 tailwindcss 包直接编译：

```js
import { compile } from 'tailwindcss'

const compiler = await compile('@import "tailwindcss";', {
  base: process.cwd(),
  loadStylesheet: async (id, base) => ({ path: ..., base, content: ... }),
})
const css = compiler.build(['p-4', 'duration-standard', 'shadow-md'])
```

**脚本必须放进 `apps/zhixu-frontend` 目录再跑**。放在 `/tmp` 会 `ERR_MODULE_NOT_FOUND`（找不到 `tailwindcss`）。

然后 grep 产物确认：

```bash
grep -o '\.duration-standard{[^}]*}' dist/assets/*.css
```

零命中 = 键名写错了或没被调用。
