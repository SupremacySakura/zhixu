<script setup lang="ts">
/**
 * 画布原型 · 变体 A —— 「暗色岛」（#31 方向：画布是唯一允许深色的地方）
 * 外壳保持卡盒世界的浅色，只有画布区域是深石墨；节点用真实标题，边用真实关系类型。
 */
import { computed } from 'vue'

interface Node {
  id: number
  title: string
  degree: number
}
interface Edge {
  id: number
  source_id: number
  target_id: number
  relation_type: string
}

const props = defineProps<{ nodes: Node[]; edges: Edge[]; num: (id: number) => string }>()

/** 确定性布局：有边的节点排内环，孤立节点排外圈。不用力导向，避免每次截图不同。 */
const layout = computed(() => {
  const linked = new Set<number>()
  for (const edge of props.edges) {
    linked.add(edge.source_id)
    linked.add(edge.target_id)
  }
  const inner = props.nodes.filter(n => linked.has(n.id))
  const outer = props.nodes.filter(n => !linked.has(n.id))
  const positions = new Map<number, { x: number; y: number }>()
  inner.forEach((node, index) => {
    const angle = (index / Math.max(1, inner.length)) * Math.PI * 2 - Math.PI / 2
    positions.set(node.id, { x: 400 + Math.cos(angle) * 190, y: 290 + Math.sin(angle) * 165 })
  })
  outer.forEach((node, index) => {
    const angle = (index / Math.max(1, outer.length)) * Math.PI * 2 - Math.PI / 2
    positions.set(node.id, { x: 400 + Math.cos(angle) * 330, y: 290 + Math.sin(angle) * 250 })
  })
  return { positions, linked }
})

const nodeRadius = (degree: number) => 9 + Math.min(degree, 6) * 2

const edgeColor = (type: string) =>
  type === 'contradicts' ? '#c4554a' : type === 'updates' ? '#7fa8c9' : '#5a7591'
</script>

<template>
  <div class="flex min-h-0 flex-1 flex-col">
    <header class="flex h-16 flex-none items-center gap-4 border-b border-hairline px-6">
      <div>
        <h1 class="text-title">知识图谱</h1>
        <p class="mt-0.5 font-type text-[10.5px] tracking-[0.06em] text-label-secondary">
          {{ nodes.length }} 张卡 · {{ edges.length }} 条交叉引用 · 局部一跳
        </p>
      </div>
      <div class="ml-auto flex gap-2">
        <button type="button" class="cabinet-btn h-8 px-3 text-[12.5px]">回到最近</button>
        <button type="button" class="cabinet-btn h-8 px-3 text-[12.5px]">图例</button>
      </div>
    </header>

    <div class="flex min-h-0 flex-1">
      <div class="min-h-0 flex-1 bg-[#16191c]">
        <svg viewBox="0 0 800 580" class="h-full w-full" role="img" aria-label="知识关系画布">
          <g v-for="edge in edges" :key="edge.id">
            <line
              v-if="layout.positions.get(edge.source_id) && layout.positions.get(edge.target_id)"
              :x1="layout.positions.get(edge.source_id)!.x"
              :y1="layout.positions.get(edge.source_id)!.y"
              :x2="layout.positions.get(edge.target_id)!.x"
              :y2="layout.positions.get(edge.target_id)!.y"
              :stroke="edgeColor(edge.relation_type)"
              :stroke-width="edge.relation_type === 'contradicts' ? 1.6 : 1"
              :stroke-dasharray="edge.relation_type === 'updates' ? '4 3' : undefined"
            />
          </g>
          <g v-for="node in nodes" :key="node.id">
            <template v-if="layout.positions.get(node.id)">
              <circle
                :cx="layout.positions.get(node.id)!.x"
                :cy="layout.positions.get(node.id)!.y"
                :r="nodeRadius(node.degree)"
                :fill="layout.linked.has(node.id) ? '#e9e5db' : 'transparent'"
                :stroke="layout.linked.has(node.id) ? '#e9e5db' : '#66707a'"
                stroke-width="1"
              />
              <text
                :x="layout.positions.get(node.id)!.x"
                :y="layout.positions.get(node.id)!.y + nodeRadius(node.degree) + 12"
                text-anchor="middle"
                :fill="layout.linked.has(node.id) ? '#c9c2b2' : '#7d858e'"
                font-size="9.5"
                font-family="'Courier Prime', ui-monospace, monospace"
              >
                {{ num(node.id) }}
              </text>
            </template>
          </g>
        </svg>
      </div>

      <!-- 右侧关系清单：与画布同一份数据，文字可读 -->
      <aside class="w-[280px] flex-none border-l border-hairline bg-surface px-4 py-4">
        <p class="font-type text-[9.5px] uppercase tracking-[0.2em] text-label-tertiary">
          关系清单
        </p>
        <div v-for="edge in edges" :key="edge.id" class="mt-2 border-b border-hairline pb-1.5">
          <p class="font-type text-[9px] uppercase tracking-[0.12em] text-primary">
            {{ edge.relation_type }}
          </p>
          <p class="mt-0.5 truncate text-[11px] text-label">
            {{ nodes.find(n => n.id === edge.source_id)?.title }} →
            {{ nodes.find(n => n.id === edge.target_id)?.title }}
          </p>
        </div>
        <p v-if="!edges.length" class="mt-2 text-[11px] text-label-tertiary">还没有关系。</p>
      </aside>
    </div>
  </div>
</template>
