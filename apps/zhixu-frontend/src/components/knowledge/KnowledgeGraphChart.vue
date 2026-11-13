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
  graphEdgeStyle,
  graphNodeBorderType,
  graphNodeSize,
  resolveGraphPalette,
  type GraphPalette,
} from '../../utils/knowledgeGraph'
import { relationTypeLabel } from '../../utils/relation'

// 按需注册：只打包力导向图、提示框、Canvas 渲染器与标签布局，避免整包引入 echarts。
use([GraphChart, TooltipComponent, CanvasRenderer, LabelLayout])

/** 节点描边宽：1px 在 18px 的方块上读不出线型差异，2px 才让实线/虚线/点线分得开。 */
const NODE_BORDER_WIDTH = 2

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
    // 方形节点（AC③）：尺寸仍由度数决定，只是把圆形换成方形。
    symbol: 'rect',
    symbolSize: graphNodeSize(node.degree),
    itemStyle: {
      // 填充统一纸白 —— 深底上 15.23:1。语义不靠填充色（8 种来源只有 4 个可见色，
      // 这条路已经用到极限），靠描边样式承载。
      color: palette.nodeFill,
      borderColor: palette.nodeBorder,
      borderWidth: NODE_BORDER_WIDTH,
      borderType: graphNodeBorderType(node.source_type),
    },
    // 标签在方块右侧的深底上，用岛上文字色；纸世界的 #181818 在 #16191c 上只有 1.01:1。
    label: {
      show: true,
      position: 'right',
      distance: 5,
      color: palette.label,
      fontSize: 12,
      width: 120,
      overflow: 'truncate',
    },
  }))
  const links = props.edges.map(edge => {
    const style = graphEdgeStyle(palette, edge)
    return {
      id: String(edge.id),
      source: String(edge.source_id),
      target: String(edge.target_id),
      relation_type: edge.relation_type,
      // 箭头必须逐边给：edgeSymbol 是 series 级，没法一条边无箭头、另一条双向。
      symbol: style.symbol,
      lineStyle: {
        color: style.color,
        width: style.width,
        // 数值数组才是 ECharts 的 dash 写法；'1 3' 是 SVG 的 stroke-dasharray，会被静默忽略。
        type: style.type,
        // 用 opacity 让 canvas 自己混色，而不是预合成一个更暗的 hex
        // —— 预合成会让「调透明度」这个轴在 option 里彻底消失。
        opacity: style.opacity,
        curveness: 0.08,
      },
    }
  })
  return {
    // 深色岛由 canvas 自己铺底，避免图表与容器之间出现一帧纸色。
    backgroundColor: palette.island,
    tooltip: {
      trigger: 'item',
      backgroundColor: palette.nodeFill,
      borderColor: palette.nodeBorder,
      textStyle: { color: palette.nodeInk, fontSize: 12 },
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
            borderColor: palette.selectRing,
          },
        },
        // series 级箭头只作为兜底；真正的箭头形状由每条 link 的 symbol 决定。
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
        // 高亮只提不透明度，不改线宽 —— 改线宽会把「四轴编码」里的宽度轴抹掉。
        emphasis: { focus: 'adjacency', lineStyle: { opacity: 1 } },
        /**
         * 标签避让必须**按 dataType 分流**，不能用固定的 { hideOverlap: true }。
         *
         * 固定值的实测后果（19 条边 / 32 个节点）：边标签只剩 1 条可见，另外 18 条
         * 被 hideOverlap 判为重叠而整个隐藏。而 hideOverlap 的判定结果**随缩放变化**
         * —— 缩放后幸存的那一条从 di=3 跳到 di=9。用户看到的现象就是「关系文字不
         * 跟着线走」：其实位置一直是对的（距曲线恒 5–10px，缩放前后不变），是**可见
         * 的那一条换成了别的边**，所以文字看起来从一条线跳到了另一条线上。
         *
         * 分流后：边标签恒 19/19 全显（它们本来就贴着各自的线，不构成重叠困扰），
         * 节点标签仍走 hideOverlap 防止标题互相压住。
         */
        labelLayout: (params: { dataType?: string }) => ({
          hideOverlap: params.dataType !== 'edge',
        }),
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
