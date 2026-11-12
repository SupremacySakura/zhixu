import { describe, expect, it } from 'vitest'
import { RELATION_TYPE_LABELS, RELATION_TYPE_OPTIONS, relationTypeLabel } from '../relation'

describe('关系类型文案', () => {
  it('矛盾是第六种关系类型，中文标签为「矛盾」', () => {
    expect(RELATION_TYPE_LABELS.contradicts).toBe('矛盾')
    expect(relationTypeLabel('contradicts')).toBe('矛盾')
  })

  it('可选项覆盖全部六种类型且文案与标签表一致', () => {
    expect(RELATION_TYPE_OPTIONS).toHaveLength(6)
    const contradiction = RELATION_TYPE_OPTIONS.find(option => option.value === 'contradicts')
    expect(contradiction?.label).toBe('矛盾')
    for (const option of RELATION_TYPE_OPTIONS) {
      expect(option.label).toBe(RELATION_TYPE_LABELS[option.value])
    }
  })
})
