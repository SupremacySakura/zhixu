<script setup lang="ts">
/**
 * 画布原型 · 变体 B —— 「浅色卡盒图」（画布不深色，走卡盒世界的浅色底 + 蓝格线）
 * 结构性差异：整张画布是可见的格纸，节点是贴着格线的纸片；右侧关系清单变成底部索引带。
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

const layout = computed(() => {
  const linked = new Set<number>()
  for (const edge of props.edges) {
    linked.add(edge.source_id)
    linked.add(edge.target_id)
  }
  const positions = new Map<number, { x: number; y: number }>()
  const inner = props.nodes.filter(n => linked.has(n.id))
  const outer = props.nodes.filter(n => !linked.has(n.id))
  inner.forEach((node, index) => {
    const angle = (index / Math.max(1, inner.length)) * Math.PI * 2 - Math.PI / 2
    positions.set(node.id, { x: 420 + Math.cos(angle) * 175, y: 250 + Math.sin(angle) * 150 })
  })
  outer.forEach((node, index) => {
    const angle = (index / Math.max(1, outer.length)) * Math.PI * 2 - Math.PI / 2
    positions.set(node.id, { x: 420 + Math.cos(angle) * 315, y: 250 + Math.sin(angle) * 215 })
  })
  return { positions, linked }
})
</script>

<template>
  <div class="flex min-h-0 flex-1 flex-col">
    <header class="flex h-16 flex-none items-center gap-4 border-b border-hairline px-6">
      <div>
        <h1 class="text-title">知识图谱</h1>
        <p class="mt-0.5 font-type text-[10.5px] tracking-[0.06em] text-label-secondary">
          {{ nodes.length }} 张卡 · {{ edges.length }} 条引用
        </p>
      </div>
      <div class="ml-auto flex gap-2">
        <button type="button" class="cabinet-btn h-8 px-3 text-[12.5px]">跳数 1</button>
        <button type="button" class="cabinet-btn h-8 px-3 text-[12.5px]">按标签筛</button>
      </div>
    </header>

    <div class="relative min-h-0 flex-1 overflow-hidden bg-surface">
      <div
        class="absolute inset-0 bg-[repeating-linear-gradient(to_right,rgb(31_78_121_/_0.14)_0_1px,transparent_1px_32px),repeating-linear-gradient(to_bottom,rgb(31_78_121_/_0.14)_0_1px,transparent_1px_32px)]"
      ></div>
      <svg
        viewBox="0 0 840 520"
        class="relative h-full w-full"
        role="img"
        aria-label="知识关系画布"
      >
        <g v-for="edge in edges" :key="edge.id">
          <line
            v-if="layout.positions.get(edge.source_id) && layout.positions.get(edge.target_id)"
            :x1="layout.positions.get(edge.source_id)!.x"
            :y1="layout.positions.get(edge.source_id)!.y"
            :x2="layout.positions.get(edge.target_id)!.x"
            :y2="layout.positions.get(edge.target_id)!.y"
            :stroke="edge.relation_type === 'contradicts' ? '#a8322a' : '#1f4e79'"
            :stroke-width="edge.relation_type === 'contradicts' ? 1.4 : 1"
            :stroke-dasharray="edge.relation_type === 'updates' ? '5 3' : undefined"
            opacity="0.75"
          />
        </g>
        <g v-for="node in nodes" :key="node.id">
          <template v-if="layout.positions.get(node.id)">
            <rect
              :x="layout.positions.get(node.id)!.x - 42"
              :y="layout.positions.get(node.id)!.y - 11"
              width="84"
              height="22"
              :fill="layout.linked.has(node.id) ? '#fbf8ef' : '#f2eee4'"
              :stroke="layout.linked.has(node.id) ? '#1f4e79' : 'rgb(24 24 24 / 0.34)'"
              stroke-width="1"
            />
            <text
              :x="layout.positions.get(node.id)!.x"
              :y="layout.positions.get(node.id)!.y + 3.5"
              text-anchor="middle"
              font-size="9"
              :fill="layout.linked.has(node.id) ? '#1f4e79' : '#605949'"
            >
              {{ num(node.id) }}
            </text>
          </template>
        </g>
      </svg>
    </div>

    <div class="flex h-[92px] flex-none gap-3 overflow-x-auto border-t border-hairline px-6 py-2">
      <div
        v-for="edge in edges"
        :key="edge.id"
        class="flex-none border border-[rgb(24_24_24_/_0.34)] bg-surface-tertiary px-2 py-1"
      >
        <p class="font-type text-[9px] uppercase tracking-[0.12em] text-primary">
          {{ edge.relation_type }}
        </p>
        <p class="mt-0.5 max-w-[150px] truncate text-[10px]">
          {{ nodes.find(n => n.id === edge.source_id)?.title }}
        </p>
      </div>
    </div>
  </div>
</template>
