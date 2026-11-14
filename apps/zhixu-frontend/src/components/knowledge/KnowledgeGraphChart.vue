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
  graphNodeSize,
  graphNodeSymbol,
  resolveGraphPalette,
  type GraphPalette,
} from '../../utils/knowledgeGraph'
import { knowledgeSourceLabel } from '../../utils/knowledge'
import { relationTypeLabel } from '../../utils/relation'

// 按需注册：只打包力导向图、提示框、Canvas 渲染器与标签布局，避免整包引入 echarts。
use([GraphChart, TooltipComponent, CanvasRenderer, LabelLayout])

/**
 * 标签避让只作用于节点，边标签一律跳过。
 *
 * 这里返回 undefined 只让边标签「不参与重叠判定」，**并不足以**让它们留在局部坐标系；
 * 真正起作用的是 attachEdgeLabelOptOut() 里的 disableLabelLayout，见该函数的说明。
 */
function nodeOnlyLabelLayout(params: { dataType?: string }) {
  return params.dataType === 'edge' ? undefined : { hideOverlap: true }
}

const props = withDefaults(
  defineProps<{
    nodes: KnowledgeGraphNode[]
    edges: KnowledgeGraphEdge[]
    selectedId?: number | null
    /** 局部图谱的起点；只有它会拿到书签带。 */
    centerId?: number | null
    /** 测试或嵌入场景可直接注入解析好的令牌颜色。 */
    palette?: GraphPalette
  }>(),
  { selectedId: null, centerId: null, palette: undefined },
)

const emit = defineEmits<{ 'node-click': [id: number] }>()

const container = ref<HTMLDivElement | null>(null)
let chart: EChartsType | null = null
let activePalette: GraphPalette | null = null

function handleClick(params: ECElementEvent) {
  if (params.dataType !== 'node' || !params.data || typeof params.data !== 'object') return
  const id = Number((params.data as { id?: string | number }).id)
  if (!Number.isFinite(id)) return
  emit('node-click', id)
}

function buildNodes() {
  const palette = activePalette ?? resolveGraphPalette(container.value)
  return props.nodes.map(node => {
    const [width, height] = graphNodeSize(node.degree)
    return {
      id: String(node.id),
      name: node.title,
      value: node.degree,
      source_type: node.source_type,
      // 多色书本走 SVG 符号：纸白封面 + 蓝书脊 + 纸白缝线无法用单个 path 表达。
      symbol: graphNodeSymbol({
        width,
        height,
        sourceType: node.source_type,
        center: props.centerId === node.id,
        selected: props.selectedId === node.id,
        palette,
      }),
      symbolSize: [width, height],
      // 标签在书本右侧的深底上，用岛上文字色；纸世界的 #181818 在 #16191c 上只有 1.01:1。
      label: {
        show: true,
        position: 'right',
        distance: 5,
        color: palette.label,
        fontSize: 12,
        width: 120,
        overflow: 'truncate',
      },
    }
  })
}

function buildLinks() {
  const palette = activePalette ?? resolveGraphPalette(container.value)
  return props.edges.map(edge => {
    const style = graphEdgeStyle(palette, edge)
    // 选中节点的邻接边持续提亮，不依赖 hover：抽屉打开后鼠标会离开画布，
    // emphasis 状态随即消失，选中态就只剩一个环。这里把提亮写进数据本身。
    const adjacent =
      props.selectedId != null &&
      (edge.source_id === props.selectedId || edge.target_id === props.selectedId)
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
        opacity: adjacent ? 1 : style.opacity,
        curveness: 0.08,
      },
    }
  })
}

function buildOption(): EChartsCoreOption {
  const palette = activePalette ?? resolveGraphPalette(container.value)
  const nodes = buildNodes()
  const links = buildLinks()
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
        data?: {
          name?: string
          value?: number
          relation_type?: string
          source_type?: string
        }
      }) => {
        if (params.dataType === 'edge') {
          const type = params.data?.relation_type as RelationType | undefined
          return type ? relationTypeLabel(type) : '关系'
        }
        if (!params.data?.name) return ''
        const source = params.data.source_type
          ? ` · 来源：${knowledgeSourceLabel(params.data.source_type)}`
          : ''
        return `${params.data.name}（度数 ${params.data.value ?? 0}）${source}`
      },
    },
    series: [
      {
        type: 'graph',
        layout: 'force',
        roam: true,
        draggable: true,
        // 选中环画在书本 SVG 里（见 graphNodeSymbol）：image symbol 走 ZRImage，
        // select.itemStyle 只会应用 opacity，不会画描边。
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
        // scale: false 是硬约束：ECharts 默认会在 hover 时把 symbol 放大 1.1 倍。
        emphasis: { scale: false, focus: 'adjacency', lineStyle: { opacity: 1 } },
        /**
         * 节点标签防重叠。
         *
         * 边标签**必须**排除在外，且不能只靠回调里返回 undefined —— 那样不够。
         * 见下方 attachEdgeLabelOptOut()：只要 series 上有非空 labelLayout，
         * LabelManager 就会把**所有**标签的宿主元素 setTextConfig({ local: false })，
         * 标签坐标从「局部（跟着线走）」变成「全局（画布绝对）」。边标签的 x/y 写的是
         * 曲线中点局部坐标（Line.js 里 label.x = cp[0]），一旦被当全局解读，缩放后就会
         * 被按 1/scale 反向放大再推到远处 —— 实测缩 12 次偏离 67–311px。
         */
        labelLayout: nodeOnlyLabelLayout,
      },
    ],
  }
}

/**
 * 只更新节点符号与邻接边透明度（选中/中心变化）。
 *
 * 不能走 setOption(notMerge=true)：那会重建 series，重置力导向布局与 roam 缩放。
 * 这里用非 merge 的局部 setOption，只换 data / links；节点 id 不变时 ECharts 会复用图元。
 */
function updateSelectionVisuals() {
  if (!chart) return
  chart.setOption({ series: [{ data: buildNodes(), links: buildLinks() }] })
  // 边样式更新会走 LineDraw.updateData；重新打标，避免后续缩放时边标签被
  // labelLayout 当成全局坐标。边 id 不变时图元会复用，这里是幂等的保险。
  attachEdgeLabelOptOut()
}

/**
 * 让边标签退出 labelLayout 的接管，保住「贴着关系线」的坐标系。
 *
 * 为什么需要这一步：ECharts 的 LabelManager 只要发现 series 上写了非空 labelLayout，
 * 就会在每次布局时遍历 series group，把所有文本元素的宿主 setTextConfig({ local: false })
 * （LabelManager.js updateLayoutConfig 里那句「Force to set local false」）。而关系线标签
 * 原本由 Line.js 设的是 local: true，坐标是曲线中点的**局部**坐标；改成全局后，缩放时
 * 标签既被按 1/scale 反向放大（ECharts 想让字号恒定），坐标又被当画布绝对坐标读，
 * 于是越缩越远。实测：缩 12 次后标签距线 67–311px，用户看到的就是「关系文字飞了」。
 *
 * LabelManager 有个现成的跳过开关：child.getTextContent() 得到的 textEl 若带
 * disableLabelLayout 就整个跳过（ECharts 自己的 treemap 就这么用）。所以顺序必须是
 * **先渲染出图元 → 给边标签打标 → 再挂 labelLayout**：反过来打标时图元还没生成。
 *
 * 只打边标签：节点标签的宿主是节点图元（书本 image），边标签的宿主是 group（线组），
 * 32 节点 / 19 边在实测中一一对应，据此区分不会误伤。
 */
function attachEdgeLabelOptOut() {
  if (!chart) return
  // getModel 在 ECharts 的公开类型里是 private，但它返回的 series 模型是取图元的唯一入口；
  // 这里显式收窄成用到的那几个成员，避免把整棵内部类型拖进组件。
  const model = (
    chart as unknown as {
      getModel?: () => { getSeries: () => unknown[] }
    }
  ).getModel?.()
  if (!model) return
  const series = model.getSeries()[0] as
    | { getGraph?: () => { eachEdge: (cb: (edge: unknown) => void) => void } }
    | undefined
  const graph = series?.getGraph?.()
  if (!graph) return
  graph.eachEdge(edge => {
    const el = (edge as { getGraphicEl?: () => unknown }).getGraphicEl?.() as {
      getTextContent?: () => { disableLabelLayout?: boolean } | null
    } | null
    const textEl = el?.getTextContent?.()
    if (textEl) textEl.disableLabelLayout = true
  })
}

function render() {
  if (!chart) return
  // 分两步：先不带 labelLayout 渲染（此时边标签还没被打标，得先有图元才能打），
  // 打标之后再挂 labelLayout —— 顺序反了边标签就已被 LabelManager 改成全局坐标。
  const option = buildOption()
  const [series] = option.series as object[]
  chart.setOption({ ...option, series: [{ ...series, labelLayout: null }] }, true)
  attachEdgeLabelOptOut()
  chart.setOption({ series: [{ labelLayout: nodeOnlyLabelLayout }] })
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

// 选中/中心变化只换节点符号与邻接边样式，不重建图表。
watch(() => [props.selectedId, props.centerId], updateSelectionVisuals)

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
