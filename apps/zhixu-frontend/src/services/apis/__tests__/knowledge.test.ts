import { beforeEach, describe, expect, it, vi } from 'vitest'
import request from '../../request'
import { getKnowledgeGraph } from '../knowledge'

vi.mock('../../request', () => ({ default: { get: vi.fn() } }))

beforeEach(() => {
  vi.mocked(request.get).mockReset()
  vi.mocked(request.get).mockResolvedValue({
    data: { code: 0, data: { nodes: [], edges: [], truncated: false } },
  })
})

describe('getKnowledgeGraph', () => {
  it('默认只传 depth 与关闭的建议边，不传筛选参数', async () => {
    const signal = new AbortController().signal
    await getKnowledgeGraph({}, signal)
    expect(request.get).toHaveBeenCalledWith('/api/knowledge/graph', {
      params: { depth: 1, include_suggested: 0 },
      signal,
    })
  })

  it('中心、标签与来源筛选一起透传为接口参数', async () => {
    await getKnowledgeGraph({
      centerId: 7,
      includeSuggested: true,
      tagId: 3,
      sourceType: 'web',
    })
    expect(request.get).toHaveBeenCalledWith('/api/knowledge/graph', {
      params: { depth: 1, include_suggested: 1, center_id: 7, tag_id: 3, source_type: 'web' },
      signal: undefined,
    })
  })

  it('零值与空字符串不进入查询串，避免后端把 0 当成非法标签', async () => {
    await getKnowledgeGraph({ centerId: 0, tagId: 0, sourceType: '' })
    expect(request.get).toHaveBeenCalledWith('/api/knowledge/graph', {
      params: { depth: 1, include_suggested: 0 },
      signal: undefined,
    })
  })
})
