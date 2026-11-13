#!/usr/bin/env node
/**
 * 世界防通胀守卫的自测（#38 批 2）
 *
 * 为什么要有这个脚本：守卫的价值全在「能不能拦住」上，而 `eslint .` 跑干净
 * 只说明**现有代码没违规**，说明不了规则本身还活着。规则一旦因为
 * 解析器升级、AST 形状变化、或者谁顺手改了正则而失效，`eslint .` 依旧全绿，
 * 守卫会静默死掉几个月才被发现。
 *
 * 所以这里对 `src/pages/__tests__/fixtures/WorldDrift.fixture.vue` 跑一遍，
 * 断言「该拦的都拦了、不该拦的没拦」。fixture 在 eslint.config.js 里被显式
 * 关掉这条规则，正是为了让本脚本成为它唯一的消费者。
 */
import { Linter } from 'eslint'
import vueParser from 'vue-eslint-parser'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, join } from 'node:path'
import worldDrift from '../eslint-rules/no-world-drift.js'

const here = dirname(fileURLToPath(import.meta.url))
const fixture = join(here, '..', 'src', 'pages', '__tests__', 'fixtures', 'WorldDrift.fixture.vue')

const linter = new Linter()
const code = readFileSync(fixture, 'utf8')

const messages = linter.verify(
  code,
  [
    {
      files: ['**/*.vue'],
      languageOptions: {
        parser: vueParser,
        parserOptions: { ecmaVersion: 'latest', sourceType: 'module' },
      },
      plugins: { world: worldDrift },
      rules: { 'world/no-world-drift': 'error' },
    },
  ],
  fixture,
)

// fixture 用**分段注释**划出两类样本：`应该被拦` 之后到下一个分段注释之前
// 的每个样本行都必须报；`不该被拦` 之后同理都必须不报。
// 用行号集合断言而不是数字符串，这样往段里加样本只管加行、不用改断言。
const lines = code.split('\n')
const mustHit = new Set()
const mustMiss = new Set()
let mode = null
lines.forEach((line, i) => {
  const trimmed = line.trim()
  if (/^<!--.*(应该被拦|must be banned)/i.test(trimmed)) {
    mode = 'hit'
    return
  }
  if (/^<!--.*(不该被拦|must pass)/i.test(trimmed)) {
    mode = 'miss'
    return
  }
  // 分段注释本身、空行、模板结构行都不算样本
  if (
    !mode ||
    trimmed === '' ||
    trimmed.startsWith('<!--') ||
    /^<\/?(template|div|p|button|span)\b/.test(trimmed) === false
  ) {
    return
  }
  // 只认「带 class 的元素行」当样本，避免把闭合标签算进去
  if (!/\bclass\b/.test(trimmed)) return
  ;(mode === 'hit' ? mustHit : mustMiss).add(i + 1)
})

const hit = new Set(messages.map(m => m.line))

// 「应该被拦」的断言：每个 mustHit 行至少报一条。
const missed = [...mustHit].filter(l => !hit.has(l))
// 「不该被拦」的断言：mustMiss 行一条都不能报。
const falsePositives = [...mustMiss].filter(l => hit.has(l))

// 关键单项断言：AC 明写守卫必须拦得住带响应式前缀的写法。
const RESPONSIVE_CASES = [
  'sm:rounded-full',
  'md:max-w-6xl',
  'sm:backdrop-blur-xl',
  'lg:text-[42px]',
]
const reportedTokens = messages.map(m => m.message).join('\n')
const unguardedResponsive = RESPONSIVE_CASES.filter(t => !reportedTokens.includes(t))

let failed = false
const fail = msg => {
  failed = true
  console.error(`✗ ${msg}`)
}

if (missed.length) fail(`应该拦但没拦的行：${missed.join(', ')}`)
if (falsePositives.length) fail(`不该拦却报了的行：${falsePositives.join(', ')}`)
if (unguardedResponsive.length) fail(`响应式前缀变体没拦住：${unguardedResponsive.join(', ')}`)
if (messages.length === 0) fail('守卫一条都没报 —— 规则很可能已经失效')

if (failed) process.exit(1)

console.log(
  `✓ 守卫自测通过：拦 ${messages.length} 条，覆盖 ${mustHit.size} 个应拦样本，` +
    `${mustMiss.size} 个放行样本无误报，响应式前缀 ${RESPONSIVE_CASES.length}/${RESPONSIVE_CASES.length} 拦住`,
)
