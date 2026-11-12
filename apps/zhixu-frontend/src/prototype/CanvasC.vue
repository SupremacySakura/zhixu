<script setup lang="ts">
/**
 * 画布原型 · 变体 C —— 「行列对位图」
 * 结构性差异：不画力导向，改成矩阵 —— 行与列都是知识卡，交叉格子里标关系类型。
 * 这是索引卡盒世界「对位」思路的极端版：一眼看出谁跟谁连、谁孤立。
 */
import { computed } from 'vue'

interface Node {
  id: number
  title: string
}
interface Edge {
  id: number
  source_id: number
  target_id: number
  relation_type: string
}

const props = defineProps<{ nodes: Node[]; edges: Edge[]; num: (id: number) => string }>()

/** 只取有关系的节点 + 少量孤立节点做矩阵，否则 31×31 读不了。 */
const matrixNodes = computed(() => {
  const linked = new Set<number>()
  for (const edge of props.edges) {
    linked.add(edge.source_id)
    linked.add(edge.target_id)
  }
  const withEdges = props.nodes.filter(n => linked.has(n.id))
  const isolated = props.nodes.filter(n => !linked.has(n.id)).slice(0, 2)
  return [...withEdges, ...isolated]
})

const cellOf = computed(() => {
  const map = new Map<string, string>()
  for (const edge of props.edges) {
    map.set(`${edge.source_id}-${edge.target_id}`, edge.relation_type)
    map.set(`${edge.target_id}-${edge.source_id}`, edge.relation_type)
  }
  return map
})

const short = (title: string) => (title.length > 12 ? `${title.slice(0, 12)}…` : title)
const mark = (type: string | undefined) =>
  type === 'contradicts'
    ? '×'
    : type === 'depends_on'
      ? '↓'
      : type === 'updates'
        ? '↻'
        : type === 'supplements'
          ? '+'
          : type === 'contrasts'
            ? '≠'
            : type
              ? '·'
              : ''
</script>

<template>
  <div class="flex min-h-0 flex-1 flex-col">
    <header class="flex h-16 flex-none items-center gap-4 border-b border-hairline px-6">
      <div>
        <h1 class="text-title">关系对位</h1>
        <p class="mt-0.5 font-type text-[10.5px] tracking-[0.06em] text-label-secondary">
          {{ matrixNodes.length }} 张卡参与对位 · {{ edges.length }} 组关系
        </p>
      </div>
      <p class="ml-auto font-type text-[10px] text-label-secondary">
        × 矛盾 · ↓ 依赖 · + 补充 · ≠ 对照 · ↻ 更新 · · 相关
      </p>
    </header>

    <div class="min-h-0 flex-1 overflow-auto px-6 py-4">
      <table class="border-collapse font-type text-[9px]">
        <thead>
          <tr>
            <th
              class="sticky left-0 z-10 bg-canvas px-2 py-1 text-left font-normal text-label-tertiary"
            >
              卡 \ 卡
            </th>
            <th
              v-for="node in matrixNodes"
              :key="node.id"
              class="w-[26px] px-0 py-1 text-center font-normal text-label-tertiary"
              :title="node.title"
            >
              <span class="inline-block origin-bottom-left rotate-[-60deg] whitespace-nowrap">{{
                num(node.id)
              }}</span>
            </th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in matrixNodes" :key="row.id">
            <th
              class="sticky left-0 z-10 max-w-[150px] truncate whitespace-nowrap bg-canvas py-0.5 pr-2 text-left font-normal text-[10px] text-label"
              :title="row.title"
            >
              <span class="text-primary">{{ num(row.id) }}</span> {{ short(row.title) }}
            </th>
            <td
              v-for="col in matrixNodes"
              :key="col.id"
              class="h-[22px] w-[26px] border border-hairline text-center"
              :class="
                cellOf.get(`${row.id}-${col.id}`) === 'contradicts'
                  ? 'text-danger font-bold'
                  : cellOf.get(`${row.id}-${col.id}`)
                    ? 'text-primary'
                    : 'text-label-tertiary'
              "
              :title="cellOf.get(`${row.id}-${col.id}`) ?? ''"
            >
              {{ row.id === col.id ? '—' : mark(cellOf.get(`${row.id}-${col.id}`)) }}
            </td>
          </tr>
        </tbody>
      </table>
      <p class="mt-3 text-[10px] text-label-tertiary">
        空白格代表这两张卡之间还没有建立关系。矩阵是两个方向都标的。
      </p>
    </div>
  </div>
</template>
