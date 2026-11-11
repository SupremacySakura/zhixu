<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  init,
  use,
  type EChartsCoreOption,
  type EChartsType,
  type ECElementEvent,
} from 'echarts/core'
import { GraphChart } from 'echarts/charts'
import { TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import type {
  KnowledgeGraphEdge,
  KnowledgeGraphNode,
  RelationType,
} from '../../services/apis/knowledge'
import {
  graphEdgeColor,
  graphNodeSize,
  resolveGraphPalette,
  type GraphPalette,
} from '../../utils/knowledgeGraph'
import { relationTypeLabel } from '../../utils/relation'

// 按需注册：只打包力导向图、提示框与 Canvas 渲染器，避免整包引入 echarts。
use([GraphChart, TooltipComponent, CanvasRenderer])

const props = withDefaults(
  defineProps<{
    nodes: KnowledgeGraphNode[]
    edges: KnowledgeGraphEdge[]
    selectedId?: number | null
    /** 测试或嵌入场景可直接注入解析好的令牌颜色。 */
    palette?: GraphPalette
  }>(),
  { selectedId: null, palette: undefined },
)

const emit = defineEmits<{ 'node-click': [id: number] }>()

const container = ref<HTMLDivElement | null>(null)
let chart: EChartsType | null = null
let activePalette: GraphPalette | null = null

function handleClick(params: ECElementEvent) {
  if (params.dataType !== 'node' || !params.data || typeof params.data !== 'object') return
  const id = Number((params.data as { id?: string | number }).id)
  if (Number.isFinite(id)) emit('node-click', id)
}

function buildOption(): EChartsCoreOption {
  const palette = activePalette ?? resolveGraphPalette(container.value)
  const nodes = props.nodes.map(node => ({
    id: String(node.id),
    name: node.title,
    value: node.degree,
    symbolSize: graphNodeSize(node.degree),
    selected: node.id === props.selectedId,
    itemStyle: {
      color: palette.source[node.source_type] || palette.source.default,
      borderColor: palette.primary || palette.label,
      borderWidth: node.id === props.selectedId ? 3 : 0,
    },
    label: {
      show: true,
      color: palette.label,
      fontSize: 12,
      width: 120,
      overflow: 'truncate',
    },
  }))
  const links = props.edges.map(edge => ({
    id: String(edge.id),
    source: String(edge.source_id),
    target: String(edge.target_id),
    relation_type: edge.relation_type,
    lineStyle: {
      color: graphEdgeColor(palette, edge.source),
      width: edge.status === 'suggested' ? 1.5 : 2,
      type: edge.status === 'suggested' ? 'dashed' : 'solid',
      opacity: edge.status === 'suggested' ? 0.65 : 0.9,
      curveness: 0.08,
    },
  }))
  return {
    tooltip: {
      trigger: 'item',
      backgroundColor: palette.surface,
      borderColor: palette.hairline,
      textStyle: { color: palette.label, fontSize: 12 },
      formatter: (params: {
        dataType?: string
        data?: { name?: string; value?: number; relation_type?: string }
      }) => {
        if (params.dataType === 'edge') {
          const type = params.data?.relation_type as RelationType | undefined
          return type ? relationTypeLabel(type) : '关系'
        }
        return params.data?.name ? `${params.data.name}（度数 ${params.data.value ?? 0}）` : ''
      },
    },
    series: [
      {
        type: 'graph',
        layout: 'force',
        roam: true,
        draggable: true,
        selectedMode: 'single',
        edgeSymbol: ['none', 'arrow'],
        edgeSymbolSize: 6,
        data: nodes,
        links,
        force: {
          repulsion: 260,
          edgeLength: [70, 140],
          gravity: 0.08,
          layoutAnimation: true,
        },
        emphasis: { focus: 'adjacency', lineStyle: { width: 3 } },
        labelLayout: { hideOverlap: true },
      },
    ],
  }
}

function render() {
  if (!chart) return
  chart.setOption(buildOption(), true)
}

function handleResize() {
  chart?.resize()
}

onMounted(() => {
  if (!container.value) return
  activePalette = props.palette ?? resolveGraphPalette(container.value)
  chart = init(container.value)
  chart.on('click', handleClick)
  render()
  window.addEventListener('resize', handleResize)
})

watch(
  () => [props.nodes, props.edges, props.selectedId, props.palette],
  () => {
    if (props.palette) activePalette = props.palette
    render()
  },
)

onBeforeUnmount(() => {
  window.removeEventListener('resize', handleResize)
  chart?.off('click', handleClick)
  chart?.dispose()
  chart = null
})
</script>

<template>
  <div
    ref="container"
    class="h-[560px] w-full touch-none overscroll-contain select-none"
    role="img"
    aria-label="知识关系力导向图"
  />
</template>
