/**
 * 原型数据：全部走真实接口（31 条真实中文知识、14 条真实关系边），
 * 不编造数字 —— impeccable 的 real content 要求。
 */
import { computed, ref, watch } from 'vue'
import {
  getKnowledge,
  getKnowledgeGraph,
  listKnowledge,
  type KnowledgeItem,
} from '../services/apis/knowledge'
import { renderKnowledgeContent } from '../utils/markdown'

interface GraphNode {
  id: number
  title: string
  degree: number
  source_type: string
}
interface GraphEdge {
  id: number
  source_id: number
  target_id: number
  relation_type: string
}

const relationLabel: Record<string, string> = {
  related: '相关',
  depends_on: '依赖',
  supplements: '补充',
  contrasts: '对照',
  updates: '更新',
  contradicts: '矛盾',
}

export function useSliceData() {
  const items = ref<KnowledgeItem[]>([])
  const total = ref(0)
  const loading = ref(true)
  const error = ref('')
  const chunksTotal = ref(0)

  /** 列表接口不返回正文（content 恒为空），卡片摘录要逐条取详情 —— 只取首屏可见的前 14 条。 */
  const contentById = ref(new Map<number, string>())
  const article = ref<KnowledgeItem | null>(null)
  const articleLoading = ref(true)

  const graph = ref<{ nodes: GraphNode[]; edges: GraphEdge[] }>({ nodes: [], edges: [] })

  const vectorReady = computed(() => items.value.filter(i => i.vector_state === 'ready').length)
  const pending = computed(() => items.value.filter(i => i.vector_state === 'pending').length)
  const failed = computed(() => items.value.filter(i => i.vector_state === 'partial').length)

  const relationsByItem = computed(() => {
    const map = new Map<number, { label: string; id: number; title: string }[]>()
    for (const edge of graph.value.edges) {
      const pairs: [number, number][] = [
        [edge.source_id, edge.target_id],
        [edge.target_id, edge.source_id],
      ]
      for (const [from, to] of pairs) {
        const node = graph.value.nodes.find(n => n.id === to)
        if (!node) continue
        const list = map.get(from) ?? []
        if (list.length < 3) {
          list.push({
            label: relationLabel[edge.relation_type] ?? edge.relation_type,
            id: to,
            title: node.title,
          })
          map.set(from, list)
        }
      }
    }
    return map
  })

  const shortDate = (value: string) => {
    const date = new Date(value)
    return Number.isNaN(date.getTime())
      ? '—'
      : `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
  }

  const num = (id: number) => `KM-${String(id).padStart(4, '0')}`

  const plain = (text: string) =>
    text
      .replace(/^#.*$/gm, '')
      .replace(/[#*`>]/g, '')
      .replace(/\s+/g, ' ')
      .trim()

  const excerpt = (item: KnowledgeItem) =>
    (contentById.value.get(item.id) ?? '')
      .replace(/^#.*$/gm, '')
      .replace(/[#*`>]/g, '')
      .replace(/\s+/g, ' ')
      .trim()
      .slice(0, 220)

  /** 逐条取正文（列表接口 content 恒空），只取前 16 条，用于卡片摘录与长文样本。 */
  async function loadContents() {
    const slice = items.value.slice(0, 16)
    const entries = await Promise.all(
      slice.map(async item => {
        try {
          const detail = await getKnowledge(item.id)
          return [item.id, detail.content ?? ''] as const
        } catch {
          return [item.id, ''] as const
        }
      }),
    )
    const map = new Map<number, string>()
    for (const [id, content] of entries) if (content) map.set(id, content)
    contentById.value = map

    const longest = [...map.entries()].sort((a, b) => b[1].length - a[1].length)[0]
    articleLoading.value = true
    if (longest) {
      // 直接用已取回的正文，避免二次请求失败时退回「列表项 content 为空」的假象。
      const base = items.value.find(i => i.id === longest[0])
      article.value = base ? { ...base, content: longest[1] } : null
    }
    articleLoading.value = false
  }

  async function load() {
    loading.value = true
    error.value = ''
    try {
      const page = await listKnowledge(1, '', '', '', {})
      items.value = page.items
      total.value = page.total
      chunksTotal.value = page.items.reduce((sum, item) => sum + item.chunks_total, 0)
    } catch (e) {
      error.value = e instanceof Error ? e.message : '加载失败'
    } finally {
      loading.value = false
    }
  }

  async function loadGraph() {
    try {
      const data = await getKnowledgeGraph()
      graph.value = { nodes: data.nodes, edges: data.edges }
    } catch {
      graph.value = { nodes: [], edges: [] }
    }
  }

  /** 长文原型取正文最长的一篇 —— 真实内容比造样本更有说服力。 */
  watch(
    items,
    () => {
      void loadContents()
    },
    { once: true },
  )

  void load()
  void loadGraph()

  const articleHtml = computed(() =>
    article.value ? renderKnowledgeContent(article.value.content, article.value.source_type) : '',
  )

  return {
    items,
    total,
    loading,
    error,
    chunksTotal,
    article,
    articleLoading,
    articleHtml,
    graph,
    vectorReady,
    pending,
    failed,
    relationsByItem,
    shortDate,
    num,
    excerpt,
    contentById,
    plain,
  }
}
