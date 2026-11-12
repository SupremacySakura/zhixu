import { describe, expect, it } from 'vitest'
import { highlightQuote, longestCommonPhrase } from '../evolution'

describe('longestCommonPhrase', () => {
  it('取出两段引文共有的最长片段', () => {
    expect(longestCommonPhrase('演化提醒需要人工确认', '人工确认是必要的')).toBe('人工确认')
  })

  it('没有足够长的共有片段时返回空串', () => {
    expect(longestCommonPhrase('完全不同的说法', '毫不相干的表述')).toBe('')
  })

  it('任一引文为空时返回空串', () => {
    expect(longestCommonPhrase('', '人工确认')).toBe('')
  })
})

describe('highlightQuote', () => {
  it('把命中的片段切成标记段，其余保持原文', () => {
    expect(highlightQuote('演化提醒需要人工确认', '人工确认')).toEqual([
      { text: '演化提醒需要', hit: false },
      { text: '人工确认', hit: true },
    ])
  })

  it('同一片段出现多次时全部标记', () => {
    expect(highlightQuote('确认一次，再确认一次', '确认')).toEqual([
      { text: '确认', hit: true },
      { text: '一次，再', hit: false },
      { text: '确认', hit: true },
      { text: '一次', hit: false },
    ])
  })

  it('没有命中片段时原样返回单段', () => {
    expect(highlightQuote('演化提醒需要人工确认', '')).toEqual([
      { text: '演化提醒需要人工确认', hit: false },
    ])
  })
})
