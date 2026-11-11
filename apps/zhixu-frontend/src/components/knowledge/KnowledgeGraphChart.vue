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
import { LabelLayout } from 'echarts/features'
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

// 按需注册：只打包力导向图、提示框、Canvas 渲染器与标签布局，避免整包引入 echarts。
use([GraphChart, TooltipComponent, CanvasRenderer, LabelLayout])

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
/** 图表内部当前选中的节点索引；null 表示无选中。 */
let selectedIndex: number | null = null

function handleClick(params: ECElementEvent) {
  if (params.dataType !== 'node' || !params.data || typeof params.data !== 'object') return
  const id = Number((params.data as { id?: string | number }).id)
  if (!Number.isFinite(id)) return
  emit('node-click', id)
}

function buildOption(): EChartsCoreOption {
  const palette = activePalette ?? resolveGraphPalette(container.value)
  const nodes = props.nodes.map(node => ({
    id: String(node.id),
    name: node.title,
    value: node.degree,
    symbolSize: graphNodeSize(node.degree),
    itemStyle: {
      color: palette.source[node.source_type] || palette.source.default,
      borderColor: palette.primary || palette.label,
      borderWidth: 0,
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
        // 选中态只改描边，避免为了高亮重建整张图（重建会丢失力导向位置与缩放）。
        select: {
          itemStyle: {
            borderWidth: 3,
            borderColor: palette.primary || palette.label,
          },
        },
        edgeSymbol: ['none', 'arrow'],
        edgeSymbolSize: 6,
        // 边上直接标注中文关系类型，用较小字号；节点标签由 labelLayout 隐藏重叠。
        edgeLabel: {
          show: true,
          fontSize: 10,
          color: palette.labelSecondary,
          formatter: (params: { data?: { relation_type?: string } }) => {
            const type = params.data?.relation_type as RelationType | undefined
            return type ? relationTypeLabel(type) : ''
          },
        },
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

/** 当前 props.selectedId 对应的节点索引；节点不在图内时视为无选中。 */
function resolveSelectedIndex(): number | null {
  if (props.selectedId == null) return null
  const index = props.nodes.findIndex(node => node.id === props.selectedId)
  return index >= 0 ? index : null
}

/**
 * 通过 select / unselect 动作同步选中态。
 * 不能走 setOption(notMerge=true)：那会重建 series，重置力导向布局与 roam 缩放。
 */
function applySelection() {
  if (!chart) return
  const next = resolveSelectedIndex()
  if (selectedIndex != null && selectedIndex !== next) {
    chart.dispatchAction({ type: 'unselect', seriesIndex: 0, dataIndex: selectedIndex })
  }
  if (next != null && next !== selectedIndex) {
    chart.dispatchAction({ type: 'select', seriesIndex: 0, dataIndex: next })
  }
  selectedIndex = next
}

function render() {
  if (!chart) return
  // setOption 重建 series 后选中态清空，重新按 props.selectedId 选中。
  selectedIndex = null
  chart.setOption(buildOption(), true)
  applySelection()
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
  () => [props.nodes, props.edges, props.palette],
  () => {
    if (props.palette) activePalette = props.palette
    render()
  },
)

// 选中变化只更新选中态，不重建图表。
watch(() => props.selectedId, applySelection)

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
