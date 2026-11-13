#!/usr/bin/env node
/**
 * 图标守门：模板/映射表里引用的图标名 × 本地图标数据。
 *
 * 为什么需要它：缺图时 Iconify 只静默输出空节点——没有空盒、没有控制台警告，
 * `addCollection()` 返回 undefined 也判不了成败。少了这道闸，图标掉光的同时
 * 类型检查、364 个用例、构建全都照样通过，只有人眼看截图才发现。
 *
 * 差集口径（非测试全口径 106）：
 *   1. `src/**` 下 `.vue` 模板里 `material-symbols:xxx` 字面量（静态 icon 属性）
 *   2. `src/**` 下 `.ts` 映射表里的同一字面量（经 `:icon="card.icon"` 动态绑定使用）
 *   3. `.vue` 里 `material-symbols-outlined` 连字 span 里的下划线名（转驼峰前先归一）
 * 排除 `__tests__`：测试里 mock 掉图标组件，不进运行期。
 *
 * 用法：node scripts/check-icons.mjs        # 差集非空即 exit 1
 */
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, relative } from 'node:path'
import { fileURLToPath } from 'node:url'

const ROOT = fileURLToPath(new URL('..', import.meta.url))
const SRC = join(ROOT, 'src')
const DATA = join(SRC, 'assets/icons.json')

const ICON_RE = /material-symbols:([a-z0-9]+(?:-[a-z0-9]+)*)/g

function walk(dir, acc = []) {
  for (const name of readdirSync(dir)) {
    const abs = join(dir, name)
    if (statSync(abs).isDirectory()) {
      if (name === '__tests__' || name === 'node_modules') continue
      walk(abs, acc)
    } else {
      acc.push(abs)
    }
  }
  return acc
}

const files = walk(SRC)
const local = JSON.parse(readFileSync(DATA, 'utf8'))
const known = new Set(Object.keys(local.icons ?? {}).map(n => `${local.prefix}:${n}`))
for (const alias of Object.keys(local.aliases ?? {})) known.add(`${local.prefix}:${alias}`)

/** @type {Map<string, Set<string>>} */
const referenced = new Map()
const record = (name, file) => {
  if (!referenced.has(name)) referenced.set(name, new Set())
  referenced.get(name).add(relative(ROOT, file))
}

for (const abs of files) {
  if (!/\.(vue|ts)$/.test(abs) || /\.d\.ts$/.test(abs)) continue
  const text = readFileSync(abs, 'utf8')
  for (const m of text.matchAll(ICON_RE)) record(m[0], abs)
  // Material Symbols 连字写法（<span class="material-symbols-outlined">lock_clock</span>）
  if (abs.endsWith('.vue')) {
    for (const m of text.matchAll(/material-symbols-outlined[^>]*>\s*([a-z][a-z0-9_]*)\s*</g)) {
      record(`${local.prefix}:${m[1].replace(/_/g, '-')}`, abs)
    }
  }
}

const missing = [...referenced.keys()].filter(n => !known.has(n)).sort()
const unused = [...known].filter(n => !referenced.has(n)).sort()

console.log(`本地图标数据：${relative(ROOT, DATA)} · ${known.size} 个字形`)
console.log(`引用到的图标：${referenced.size} 个（扫描 ${files.length} 个文件，已排除 __tests__）`)

if (missing.length) {
  console.error(`\n✗ 有 ${missing.length} 个图标在模板/映射表里被引用，但本地数据里没有：`)
  for (const name of missing) {
    console.error(`  ${name}`)
    for (const file of [...referenced.get(name)].sort()) console.error(`    ← ${file}`)
  }
  console.error('\n缺图时 Iconify 只静默输出空节点。请把它们补进 src/assets/icons.json。')
  process.exit(1)
}

if (unused.length) {
  console.log(
    `提示：${unused.length} 个本地字形当前未被引用（无害，可留待后续批次）：${unused.join(', ')}`,
  )
}

/**
 * body 校验：字形内容必须能继承文字色。
 *
 * 为什么名字对上了还要查 body：批 1 把 104 个图标从远程包搬进本地 JSON 时，
 * 剥掉 `<path>` 上的 `fill="currentColor"`，字形全部退化成纯黑。而当时
 * typecheck / 458 个用例 / build / 本脚本**全部通过**——只比对名字的话，
 * 「引用存在且数据存在」这条判断依然成立。只有人眼看截图才发现图标黑成一片。
 *
 * 所以这里补一条：**每个被引用到的字形**（不是全部 107 个，未被引用的不影响
 * 渲染）的 body 必须带 `fill="currentColor"`。缺了就是静默失色，直接拦下。
 * 未被引用的字形只提示，不阻断——它们还没进任何界面。
 */
const colorless = []
for (const name of [...referenced.keys()].sort()) {
  if (!known.has(name)) continue // 缺图已在上一条报过，不重复
  const glyph = local.icons?.[name.slice(local.prefix.length + 1)]
  if (!glyph) continue // alias 指向的实体，跳过（aliases 当前为 0）
  if (!glyph.body?.includes('fill="currentColor"')) {
    colorless.push({ name, files: [...referenced.get(name)].sort(), body: glyph.body ?? '' })
  }
}

if (colorless.length) {
  console.error(
    `\n✗ 有 ${colorless.length} 个被引用的字形 body 缺少 fill="currentColor"，会退化成纯黑：`,
  )
  for (const { name, files, body } of colorless) {
    console.error(`  ${name}`)
    console.error(`    body: ${body.slice(0, 120)}${body.length > 120 ? '…' : ''}`)
    for (const file of files) console.error(`    ← ${file}`)
  }
  console.error('\n字形要跟随文字色（含禁用态、深色画布上的浅色），body 里每个 <path> 都必须带')
  console.error('fill="currentColor"。请在 src/assets/icons.json 里补上后重跑。')
  process.exit(1)
}

console.log('✓ 引用的图标全部存在于本地数据')
console.log(`✓ 被引用的 ${referenced.size} 个字形 body 均带 fill="currentColor"`)
