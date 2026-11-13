/**
 * 世界防通胀守卫（#38 批 2）
 *
 * 为什么需要这条规则：迁移前同一个主按钮有 52 种写法、`rounded-*` 有 323 处、
 * `duration-150` 有 103 处。这些都不是一次性错误，而是**每一次新写代码时的小小
 * 方便**累积出来的。删干净之后再靠人眼守，三个月就会涨回去。
 *
 * 为什么不用现成的 `vue/no-restricted-class`：它只做**精确匹配**，
 * 写 `rounded-full` 能拦住，写 `sm:rounded-full` 就漏了 —— 而响应式前缀
 * 恰恰是最容易随手加的写法。`no-restricted-syntax` 也不行：核心规则只看
 * `<script>` 的 AST，模板里的 class 它根本遍历不到（实测 0 报错、exit 0）。
 *
 * 为什么必须用 `defineTemplateBodyVisitor` 注册 `VAttribute`：模板 AST 里的
 * class 是 VAttribute/VLiteral 节点，只有模板遍历器能进到那里。注意 eslint-plugin-vue
 * 的包根**不导出** `utils`，所以不能 `import { defineTemplateBodyVisitor }`，
 * 只能从 `context.sourceCode.parserServices` 上取。
 *
 * 为什么要剥变体前缀：`sm:` / `md:` / `hover:` / `focus:` / `dark:` / `group-hover:`
 * 都可以叠在同一个工具类前面，且能叠好几层（`sm:group-hover:rounded-full`）。
 * 守卫关心的是**最终落到元素上的那个工具类**，所以前缀必须一路剥到底。
 */

/** 变体前缀：`sm:` `hover:` `focus-visible:` `group-hover:` `peer-focus:` 以及任意值变体 `[&>*]:`。 */
const VARIANT = /^(?:[a-z0-9-]+(?:-\[[^\]]*\])?|\[[^\]]+\]):/

/**
 * 六类回潮。每类给出「人话原因」，报错时直接说清为什么不行，
 * 而不是只说「被禁用了」——被禁用的规则没人愿意查第二遍。
 */
const BANS = [
  {
    id: 'radius',
    // 九个 radius 令牌全是 0px，世界里的圆只有线环（atom-ring 的 50%）。
    // rounded-none 是「明确声明不要圆角」，与世界同向，不拦。
    test: /^rounded(?:-(?:xs|sm|md|lg|xl|2xl|3xl|4xl|full))?$/,
    message:
      '圆角在世界里恒为 0（--radius-* 全是 0px）。删掉这个类，不要用 rounded 表达层次；圆只允许出现在 atom-ring 线环上。',
  },
  {
    id: 'arbitrary-font-size',
    // 任意值字号绕过了字级表，行高与字重都不跟着走。
    test: /^text-\[[^\]]*(?:px|rem|em)\]$/,
    message:
      '任意值字号绕过了字级表（--text-display/title/read/lead/body/note/micro）。改用语义档；图标尺寸走 atom-glyph 的 --atom-glyph-size。',
  },
  {
    id: 'framework-shadow',
    // 七档框架阴影全是 none；世界里只有三档硬偏移。
    test: /^shadow-(?:2xs|xs|sm|md|lg|xl|2xl)$/,
    message:
      '框架阴影七档在世界里全是 none。层级用线、留白、硬偏移三选二；要阴影就用 shadow-card / shadow-popover / shadow-floating。',
  },
  {
    id: 'material',
    // 毛玻璃与 material 令牌都退役了。
    test: /^(?:bg-material(?:-thick)?|backdrop-blur(?:-\S+)?|backdrop-saturate(?:-\S+)?)$/,
    message:
      '毛玻璃退役：backdrop-blur / backdrop-saturate / bg-material 都不再生效。表面用 bg-canvas / bg-surface / bg-surface-secondary / bg-surface-tertiary。',
  },
  {
    id: 'container',
    // 容器宽度只有三档令牌，max-w-6xl 是框架默认值。
    test: /^max-w-(?:6xl|7xl|5xl|4xl)$/,
    message:
      '容器宽度走令牌：max-w-measure（正文 40em）/ max-w-panel（面板 56rem）/ max-w-wide（宽 72rem）。',
  },
  {
    id: 'duration',
    // 三档时长是 micro 90 / standard 160 / sheet 220，没有第四档。
    test: /^duration-(?:\d+|\[[^\]]+\])$/,
    message:
      '过渡时长只有三档：micro 90ms（默认，不写即落档）/ standard 160ms / sheet 220ms。删掉 duration-* 覆盖，或写 duration-standard / duration-sheet。',
  },
  {
    id: 'scale',
    // 按压缩放被世界判死，按压反馈只走底色。
    // 只拦「交互反馈」前缀：active: / hover: / group-hover: / focus: 系列。
    // 无前缀的 scale（拖拽中 scale-[1.01]、菜单入场初值 scale-[0.95]）是状态驱动的
    // 几何，不是假物理，不拦。
    raw: true,
    test: /^(?:active|hover|group-hover|group-active|focus|focus-visible|focus-within|peer-hover|peer-focus):(?:[a-z0-9-]+:)*scale-/,
    message:
      '按压缩放退役（active:scale-*、hover:scale-*）。按压反馈只改底色：active:bg-primary-pressed / active:bg-surface-secondary。',
  },
  {
    id: 'transition-all',
    // transition-all 会把布局属性一起过渡，滚动时掉帧。
    test: /^transition-all$/,
    message:
      '不要 transition-all：它连 width/height 都过渡，滚动时掉帧。列出真正在变的属性，常见的是 transition-colors。',
  },
  {
    id: 'spin',
    // 旋转图标被 AppLoading 的线环取代。
    test: /^animate-(?:spin|ping|bounce|pulse)$/,
    message:
      '加载态只有 AppLoading（1s linear 的 1px 线环）。图标不许自己转；骨架屏用静态 bg-hairline 条。',
  },
  {
    id: 'focus',
    // 焦点环全局一处，在 base 层。这条要匹配**原始** token：前缀本身就是触发条件。
    raw: true,
    test: /^(?:focus|focus-visible|focus-within):(?:outline-\S+|ring-\S+)$/,
    message:
      '焦点环全局只有一处（@layer base 的 :focus-visible，2px 实心蓝 + offset 2px）。删掉组件里的 outline-* / ring-* 焦点样式。',
  },
]

/** 剥掉响应式前缀（sm:/md:/lg:/xl:/2xl:/max-*:），保留状态前缀。 */
function stripResponsive(raw) {
  let token = raw
  for (;;) {
    const next = token.replace(/^(?:sm|md|lg|xl|2xl|max-sm|max-md|max-lg|max-xl|max-2xl):/, '')
    if (next === token) return token
    token = next
  }
}

/** 剥掉全部变体前缀，返回最后一层工具类。 */
function baseToken(raw) {
  let token = raw
  for (;;) {
    const next = token.replace(VARIANT, '')
    if (next === token) return token
    token = next
  }
}

/** 逐个 token 检查，返回命中的禁令。 */
function findBan(raw) {
  if (typeof raw !== 'string' || raw.length === 0) return null
  // 响应式/状态前缀必须先剥掉：`sm:rounded-full` 与 `rounded-full` 是同一条禁令。
  const base = baseToken(raw)
  for (const ban of BANS) {
    // focus 禁令看的是「有没有写 focus:ring-*」这个动作本身，
    // 其余禁令看的是剥掉变体前缀后落到元素上的工具类。
    if (ban.test.test(ban.raw ? stripResponsive(raw) : base)) return ban
  }
  return null
}

/** 把一个表达式节点里所有字符串字面量摊平成 token 列表（含数组/对象/三元/模板串）。 */
function tokensOf(node, out = []) {
  if (!node || typeof node !== 'object') return out
  switch (node.type) {
    case 'Literal':
      if (typeof node.value === 'string') out.push(...node.value.split(/\s+/))
      break
    case 'TemplateLiteral':
      for (const q of node.quasis ?? []) out.push(...(q.value?.cooked ?? '').split(/\s+/))
      for (const e of node.expressions ?? []) tokensOf(e, out)
      break
    case 'ArrayExpression':
      for (const el of node.elements ?? []) tokensOf(el, out)
      break
    case 'ObjectExpression':
      for (const prop of node.properties ?? []) {
        if (prop.type === 'Property') {
          tokensOf(prop.key, out)
          tokensOf(prop.value, out)
        }
      }
      break
    case 'ConditionalExpression':
      tokensOf(node.test, out)
      tokensOf(node.consequent, out)
      tokensOf(node.alternate, out)
      break
    case 'LogicalExpression':
    case 'BinaryExpression':
      tokensOf(node.left, out)
      tokensOf(node.right, out)
      break
    case 'UnaryExpression':
      tokensOf(node.argument, out)
      break
    case 'CallExpression':
      for (const a of node.arguments ?? []) tokensOf(a, out)
      break
    default:
      break
  }
  return out.filter(Boolean)
}

/** 把 VAttribute 的 value 摊成 token 列表。静态 class="a b" 与动态 :class="..." 都要管。 */
function attrTokens(valueNode) {
  if (!valueNode) return []
  if (valueNode.type === 'VExpressionContainer') {
    return tokensOf(valueNode.expression)
  }
  if (valueNode.type === 'VLiteral') {
    return (valueNode.value ?? '').split(/\s+/).filter(Boolean)
  }
  return []
}

const rule = {
  meta: {
    type: 'problem',
    docs: {
      description: '拦截世界规范里已退役的六类 Tailwind 写法（含响应式前缀变体）',
    },
    schema: [
      {
        type: 'object',
        properties: {
          // 迁移期挂账：暂时无法修的历史遗留写在这里，格式是「文件名:原始 token」。
          allow: { type: 'array', items: { type: 'string' } },
        },
        additionalProperties: false,
      },
    ],
    messages: {
      banned: '{{reason}}（命中：{{token}}）',
    },
  },

  create(context) {
    const options = context.options[0] ?? {}
    const allow = new Set(options.allow ?? [])
    const filename = context.filename ?? context.getFilename?.() ?? ''

    // eslint-plugin-vue 把模板遍历器挂在 parserServices 上；包根不导出 utils，
    // 所以只能这样取。取不到就静默不检查（非 .vue 文件本来也没有模板）。
    const services = context.sourceCode?.parserServices ?? context.getSourceCode().parserServices
    const defineTemplateBodyVisitor = services?.defineTemplateBodyVisitor
    if (typeof defineTemplateBodyVisitor !== 'function') return {}

    function report(node, raw) {
      const ban = findBan(raw)
      if (!ban) return
      // 挂账同时比对「原始 token」与「剥前缀后的 token」：
      // 有些历史遗留只在某个断点上违规，写全 `sm:rounded-full` 更精确。
      const base = baseToken(raw)
      if (allow.has(`${filename}:${raw}`) || allow.has(`${filename}:${base}`)) return
      if (allow.has(raw) || allow.has(base)) return
      context.report({
        node,
        messageId: 'banned',
        data: { reason: ban.message, token: raw },
      })
    }

    return defineTemplateBodyVisitor({
      VAttribute(node) {
        // 静态 class="…"：key.name === 'class'
        // 动态 :class="…"：key.name === 'bind'，真正的属性名在 key.argument.name
        const key = node.key ?? {}
        const isClass = key.name === 'class' || (node.directive && key.argument?.name === 'class')
        if (!isClass) return
        for (const raw of attrTokens(node.value)) report(node, raw)
      },
    })
  },
}

export default {
  meta: { name: 'no-world-drift' },
  rules: {
    'no-world-drift': rule,
  },
}
