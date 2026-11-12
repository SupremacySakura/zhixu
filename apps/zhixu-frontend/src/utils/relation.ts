import type { RelationType } from '../services/apis/knowledge'

// 关系类型的中文文案：详情页与后续图谱页共用，避免各处重复定义。
export const RELATION_TYPE_LABELS: Record<RelationType, string> = {
  related: '相关',
  depends_on: '依赖',
  supplements: '补充说明',
  contrasts: '对比',
  updates: '更新替代',
  contradicts: '矛盾',
}

export const RELATION_TYPE_OPTIONS: { value: RelationType; label: string }[] = [
  { value: 'related', label: RELATION_TYPE_LABELS.related },
  { value: 'depends_on', label: RELATION_TYPE_LABELS.depends_on },
  { value: 'supplements', label: RELATION_TYPE_LABELS.supplements },
  { value: 'contrasts', label: RELATION_TYPE_LABELS.contrasts },
  { value: 'updates', label: RELATION_TYPE_LABELS.updates },
  { value: 'contradicts', label: RELATION_TYPE_LABELS.contradicts },
]

export const relationTypeLabel = (type: RelationType) => RELATION_TYPE_LABELS[type] ?? type
