<script setup lang="ts">
/**
 * 原型样张：知识图谱「暗色岛 + 六关系四轴编码」对照页（issue #31）。
 * 一次性代码（分支 prototype/graph-encoding），不进 main。
 *
 * 已拍板口径（逐字照抄，勿改）：
 *   Q19 暗色岛：只有画布区域暗（#16191c），页头带 64px 悬浮其上保持纸色，侧栏/工具栏不变。
 *       与卡盒世界靠三件事绑定：①同样 1px 规则线 + 零圆角 ②同样 Courier Prime 数字与等宽编号
 *       ③页头带悬浮不换色。不做渐变过渡、不做发光。
 *   Q20 六关系非彩色区分：线型 × 箭头 × 线宽 × 透明度四轴编码，不加第五色相。
 *
 * 实现说明（原型可接受的取舍）：
 *   - 布局是手写 SVG 的确定性坐标，不用力导向：力导向每帧位置不同，截图不可复现。
 *   - 透明度按 #16191c 底「预合成」成实色，等效于 stroke-opacity：
 *     SVG <marker> 不继承 stroke-opacity，预合成才能让箭头与线身同色同透明度。
 *     真实实现走 ECharts lineStyle.opacity，不需要预合成。
 *   - 线宽用 vector-effect="non-scaling-stroke"，保证 1px / 1.5px / 2px 在各视口都是设备像素，
 *     否则 viewBox 缩放会把「线宽轴」稀释掉。
 *   - 色值全部写死（原型）。正式实现应从 style.css 令牌 resolveGraphPalette() 取。
 *     真实数据 31 条知识目前 source_type 全是 manual；web / text 两档为示意值。
 */
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import SliceRail from '../components/prototype/SliceRail.vue'

type RelationKey =
  | 'related'
  | 'depends_on'
  | 'supplements'
  | 'contrasts'
  | 'updates'
  | 'contradicts'

interface RelationSpec {
  key: RelationKey
  zh: string
  /** 线型：SVG stroke-dasharray；null = 实线 */
  dash: string | null
  /** 端点圆头（点线用圆头才是「点」） */
  cap: 'round' | 'butt'
  arrows: 'none' | 'end' | 'both'
  width: number
  alpha: number
  /** 预合成后的实色（#16191c 底 + alpha） */
  stroke: string
  /** 给图例用的四轴读数 */
  spec: string
}

/**
 * 六关系四轴编码表 —— 与 #31 已拍板表格逐字对应。
 * 顺序即图例顺序，也是画布上 01–06 的顺序。
 */
const RELATIONS: RelationSpec[] = [
  {
    key: 'related',
    zh: '相关',
    dash: '1 3',
    cap: 'round',
    arrows: 'none',
    width: 1,
    alpha: 0.55,
    stroke: '#3b4c5c',
    spec: '点线 · 无箭头 · 1px · 0.55',
  },
  {
    key: 'depends_on',
    zh: '依赖',
    dash: null,
    cap: 'butt',
    arrows: 'end',
    width: 1.5,
    alpha: 0.8,
    stroke: '#4c637a',
    spec: '实线 · 单向 · 1.5px · 0.80',
  },
  {
    key: 'supplements',
    zh: '补充',
    dash: null,
    cap: 'butt',
    arrows: 'end',
    width: 1,
    alpha: 0.6,
    stroke: '#3f5062',
    spec: '实线 · 单向 · 1px · 0.60',
  },
  {
    key: 'contrasts',
    zh: '对照',
    dash: '1 3',
    cap: 'round',
    arrows: 'both',
    width: 1,
    alpha: 0.7,
    stroke: '#46596e',
    spec: '点线 · 双向 · 1px · 0.70',
  },
  {
    key: 'updates',
    zh: '更新',
    dash: '5 3',
    cap: 'butt',
    arrows: 'end',
    width: 1.5,
    alpha: 0.8,
    stroke: '#4c637a',
    spec: '虚线 · 单向 · 1.5px · 0.80',
  },
  {
    key: 'contradicts',
    zh: '矛盾',
    dash: '5 3',
    cap: 'butt',
    arrows: 'both',
    width: 2,
    alpha: 0.95,
    stroke: '#bb5248',
    spec: '虚线 · 双向 · 2px · 0.95 · 唯一彩色 #c4554a',
  },
]

interface NodeSpec {
  /** 真实知识条目编号（KM-0057 等） */
  id: number
  /** 真实中文标题；个别超长标题按卡片宽度截断 */
  title: string
  source: 'manual' | 'web' | 'text'
}

/** 六组节点对：每组的边都是真实存在的知识关系（14 条真实边里取 6 条） */
const PAIRS: { a: NodeSpec; b: NodeSpec }[] = [
  {
    a: { id: 57, title: '矢量图与位图的选择', source: 'manual' },
    b: { id: 66, title: '深色模式的对比度陷阱', source: 'web' },
  },
  {
    a: { id: 71, title: '团队共享的术语表值不值得维护', source: 'web' },
    b: { id: 72, title: '为什么需要读原文', source: 'manual' },
  },
  {
    a: { id: 55, title: '缓存失效的四种策略', source: 'manual' },
    b: { id: 68, title: '阅读器需要什么样的进度指示', source: 'text' },
  },
  {
    a: { id: 59, title: '接口设计里的默认值', source: 'web' },
    b: { id: 64, title: '浏览器渲染流水线速记', source: 'manual' },
  },
  {
    a: { id: 58, title: '一次重写的教训', source: 'manual' },
    b: { id: 67, title: '单例模式的现代用法', source: 'web' },
  },
  {
    a: { id: 62, title: '渐进增强还值得吗', source: 'manual' },
    b: { id: 70, title: '文档即代码的边界', source: 'web' },
  },
]

/** 节点描边承载 source_type：manual 实线 / web 虚线 / text 无描边 */
const SOURCES: { key: NodeSpec['source']; zh: string; dash?: string }[] = [
  { key: 'manual', zh: '手工录入 · 实线' },
  { key: 'web', zh: '网页采集 · 虚线', dash: '4 2' },
  { key: 'text', zh: '粘贴文本 · 无描边' },
]

const NODE_W = 158
const NODE_H = 38
const NODE_FILL = '#f2eee4'
const NODE_INK = '#181818'
const ISLAND_BG = '#16191c'
/** 岛内次要文字：定案令牌里没有岛内灰阶，用 #8b939c（对岛 5.67:1） */
const ISLAND_TEXT = '#8b939c'

const viewportWidth = ref(1440)
const syncViewport = () => (viewportWidth.value = window.innerWidth)

onMounted(() => {
  syncViewport()
  window.addEventListener('resize', syncViewport)
})
onBeforeUnmount(() => window.removeEventListener('resize', syncViewport))

/** 单列（<768）与双列（>=768）；图例在 >=1024（lg）时才并排，窄屏下移堆叠 */
const columns = computed(() => (viewportWidth.value >= 768 ? 2 : 1))

/** 从节点中心沿单位向量 d 走到矩形边界外 pad 处（箭头贴纸边留 3px） */
function clipToNode(cx: number, cy: number, dx: number, dy: number, pad: number) {
  const tx = NODE_W / 2 / Math.max(Math.abs(dx), 1e-6)
  const ty = NODE_H / 2 / Math.max(Math.abs(dy), 1e-6)
  const t = Math.min(tx, ty)
  return { x: cx + dx * (t + pad), y: cy + dy * (t + pad) }
}

const layout = computed(() => {
  const cols = columns.value
  const cellW = cols === 1 ? 400 : 470
  const cellH = cols === 1 ? 128 : 230
  const rows = Math.ceil(PAIRS.length / cols)

  const cells = PAIRS.map((pair, index) => {
    const rel = RELATIONS[index]
    const ox = (index % cols) * cellW
    const oy = Math.floor(index / cols) * cellH

    const ax = ox + (cols === 1 ? 12 : 20)
    const ay = oy + (cols === 1 ? 18 : 32)
    const bx = ox + cellW - (cols === 1 ? 12 : 20) - NODE_W
    const by = oy + cellH - (cols === 1 ? 4 : 26) - NODE_H

    const ac = { x: ax + NODE_W / 2, y: ay + NODE_H / 2 }
    const bc = { x: bx + NODE_W / 2, y: by + NODE_H / 2 }
    const len = Math.hypot(bc.x - ac.x, bc.y - ac.y) || 1
    const ux = (bc.x - ac.x) / len
    const uy = (bc.y - ac.y) / len
    const start = clipToNode(ac.x, ac.y, ux, uy, 3)
    const end = clipToNode(bc.x, bc.y, -ux, -uy, 3)

    return {
      index,
      rel,
      captionX: ox + (cols === 1 ? 12 : 20),
      captionY: oy + (cols === 1 ? 13 : 18),
      a: { ...pair.a, x: ax, y: ay },
      b: { ...pair.b, x: bx, y: by },
      line: { x1: start.x, y1: start.y, x2: end.x, y2: end.y },
      marker: `arw-${rel.stroke.slice(1)}`,
    }
  })

  return { width: cellW * cols, height: cellH * rows, cells }
})

/** 有箭头的颜色各要一枚 marker（userSpaceOnUse，箭头不随线宽变大） */
const markers = computed(() => {
  const seen = new Map<string, string>()
  for (const rel of RELATIONS) {
    if (rel.arrows !== 'none') seen.set(`arw-${rel.stroke.slice(1)}`, rel.stroke)
  }
  return Array.from(seen, ([id, stroke]) => ({ id, stroke }))
})
</script>

<template>
  <div class="flex h-screen overflow-hidden bg-canvas">
    <SliceRail />

    <main class="flex min-h-0 min-w-0 flex-1 flex-col">
      <!-- 页头带 64px：纸色、不换色、零圆角、1px hairline 收边（与卡盒世界同一套语法） -->
      <header
        class="flex h-16 flex-none items-center gap-4 border-b border-hairline bg-canvas px-4 sm:px-6"
      >
        <div class="min-w-0">
          <h1 class="truncate text-title text-label">知识图谱 · 六关系编码对照</h1>
          <p class="mt-0.5 truncate font-type text-[10.5px] tracking-[0.06em] text-label-secondary">
            暗色岛 #16191c · 页头带保持纸色 · 零圆角 · 不加第五色相
          </p>
        </div>
        <div class="ml-auto hidden flex-none items-center gap-2 sm:flex">
          <span
            class="border border-rule bg-surface-tertiary px-2 py-1 font-type text-[9.5px] tracking-[0.06em] text-label-secondary"
          >
            CANVAS #16191c
          </span>
          <span
            class="border border-rule bg-surface-tertiary px-2 py-1 font-type text-[9.5px] tracking-[0.06em] text-label-secondary"
          >
            SHELL #e9e5db
          </span>
        </div>
      </header>

      <div class="flex min-h-0 flex-1 flex-col overflow-y-auto lg:flex-row lg:overflow-hidden">
        <!-- 暗色画布：六组节点对，逐组演示一条关系轴 -->
        <div class="flex-none overflow-hidden bg-[#16191c] lg:min-w-0 lg:flex-1">
          <svg
            :viewBox="`0 0 ${layout.width} ${layout.height}`"
            :style="{ aspectRatio: `${layout.width} / ${layout.height}` }"
            class="block h-auto w-full"
            role="img"
            aria-label="六类知识关系的线型、箭头、线宽与透明度对照"
          >
            <defs>
              <marker
                v-for="m in markers"
                :id="m.id"
                :key="m.id"
                viewBox="0 0 10 8"
                refX="9"
                refY="4"
                markerWidth="9"
                markerHeight="8"
                markerUnits="userSpaceOnUse"
                orient="auto-start-reverse"
              >
                <path
                  d="M1 1.2 L8.4 4 L1 6.8"
                  fill="none"
                  :stroke="m.stroke"
                  stroke-width="1.3"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
              </marker>
            </defs>

            <g v-for="cell in layout.cells" :key="cell.rel.key">
              <!-- 组标号 + 关系名：岛内次要文字，不靠框线分隔 -->
              <text
                :x="cell.captionX"
                :y="cell.captionY"
                class="font-type"
                :fill="ISLAND_TEXT"
                font-size="8.6"
                letter-spacing="0.5"
              >
                {{ String(cell.index + 1).padStart(2, '0') }} {{ cell.rel.key }} ·
                {{ cell.rel.zh }}
              </text>

              <!-- 边：线型 / 箭头 / 线宽 / 透明度四轴 -->
              <line
                :x1="cell.line.x1"
                :y1="cell.line.y1"
                :x2="cell.line.x2"
                :y2="cell.line.y2"
                :stroke="cell.rel.stroke"
                :stroke-width="cell.rel.width"
                :stroke-linecap="cell.rel.cap"
                :stroke-dasharray="cell.rel.dash ?? undefined"
                :marker-end="cell.rel.arrows === 'none' ? undefined : `url(#${cell.marker})`"
                :marker-start="cell.rel.arrows === 'both' ? `url(#${cell.marker})` : undefined"
                vector-effect="non-scaling-stroke"
              />

              <!-- 节点：方形零圆角 + 纸白填充；描边样式承载 source_type -->
              <g v-for="node in [cell.a, cell.b]" :key="node.id">
                <rect
                  :x="node.x"
                  :y="node.y"
                  :width="NODE_W"
                  :height="NODE_H"
                  :fill="NODE_FILL"
                  :stroke="node.source === 'text' ? 'none' : NODE_INK"
                  stroke-width="1"
                  :stroke-dasharray="node.source === 'web' ? '4 2' : undefined"
                />
                <text
                  :x="node.x + 11"
                  :y="node.y + 14"
                  class="font-type"
                  :fill="NODE_INK"
                  font-size="7.6"
                  letter-spacing="0.4"
                  opacity="0.66"
                >
                  KM-{{ String(node.id).padStart(4, '0') }}
                </text>
                <text
                  :x="node.x + 11"
                  :y="node.y + 29"
                  class="font-sans"
                  :fill="NODE_INK"
                  font-size="10"
                >
                  {{ node.title }}
                </text>
              </g>
            </g>
          </svg>
        </div>

        <!-- 图例 / 编码表：纸色面板，与画布并列或下移堆叠 -->
        <aside
          class="flex-none border-t border-hairline bg-surface lg:w-[300px] lg:overflow-y-auto lg:border-l lg:border-t-0"
          aria-label="编码图例"
        >
          <div class="border-b border-hairline px-4 py-3">
            <p class="font-type text-[10px] uppercase tracking-[0.2em] text-label-tertiary">
              Legend · 编码表
            </p>
            <h2 class="mt-1 text-[15px] font-bold text-label">六关系 · 四轴编码</h2>
            <p class="mt-1 text-[11.5px] leading-4 text-label-secondary">
              线型 × 箭头 × 线宽 × 透明度，不加第五色相。contradicts 是唯一彩色。
            </p>
          </div>

          <ul class="divide-y divide-hairline">
            <li
              v-for="(rel, index) in RELATIONS"
              :key="rel.key"
              class="flex items-center gap-2.5 px-4 py-2"
            >
              <span class="w-4 flex-none font-type text-[9.5px] text-label-tertiary">
                {{ String(index + 1).padStart(2, '0') }}
              </span>
              <svg width="60" height="16" class="flex-none" aria-hidden="true">
                <rect x="0" y="0" width="60" height="16" :fill="ISLAND_BG" />
                <line
                  x1="5"
                  y1="8"
                  x2="55"
                  y2="8"
                  :stroke="rel.stroke"
                  :stroke-width="rel.width"
                  :stroke-linecap="rel.cap"
                  :stroke-dasharray="rel.dash ?? undefined"
                />
                <path
                  v-if="rel.arrows !== 'none'"
                  d="M51 4.4 L56 8 L51 11.6"
                  fill="none"
                  :stroke="rel.stroke"
                  stroke-width="1.2"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
                <path
                  v-if="rel.arrows === 'both'"
                  d="M9 4.4 L4 8 L9 11.6"
                  fill="none"
                  :stroke="rel.stroke"
                  stroke-width="1.2"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
              </svg>
              <div class="min-w-0">
                <p class="text-[12.5px] font-semibold leading-4 text-label">
                  {{ rel.zh }}
                  <span class="font-type text-[10.5px] font-normal text-label-secondary">
                    {{ rel.key }}
                  </span>
                </p>
                <p class="font-type text-[9.5px] leading-3.5 text-label-tertiary">{{ rel.spec }}</p>
              </div>
            </li>
          </ul>

          <div class="border-t border-hairline px-4 py-3">
            <p class="font-type text-[10px] uppercase tracking-[0.2em] text-label-tertiary">
              Node · source_type 描边
            </p>
            <div class="mt-2 grid grid-cols-3 gap-1.5">
              <div v-for="source in SOURCES" :key="source.key" class="min-w-0">
                <svg viewBox="0 0 64 26" class="block h-auto w-full" aria-hidden="true">
                  <rect
                    x="1"
                    y="1"
                    width="62"
                    height="24"
                    :fill="NODE_FILL"
                    :stroke="source.key === 'text' ? 'none' : NODE_INK"
                    stroke-width="1"
                    :stroke-dasharray="source.dash"
                  />
                  <text x="7" y="17" class="font-sans" :fill="NODE_INK" font-size="9">
                    知识条目
                  </text>
                </svg>
                <p class="mt-1 font-type text-[9.5px] leading-3.5 text-label-secondary">
                  {{ source.key }}
                </p>
                <p class="text-[10px] leading-3.5 text-label-tertiary">{{ source.zh }}</p>
              </div>
            </div>
            <p class="mt-2 text-[10.5px] leading-4 text-label-tertiary">
              节点圆角 0、填充 #f2eee4、标签墨色 #181818。真实数据 31 条目前全是 manual，web / text
              为示意。
            </p>
          </div>

          <div class="border-t border-hairline px-4 py-2.5">
            <p class="font-type text-[9.5px] leading-3.5 text-label-tertiary">
              原型 #31 · prototype/graph-encoding · 一次性代码
            </p>
          </div>
        </aside>
      </div>
    </main>
  </div>
</template>
