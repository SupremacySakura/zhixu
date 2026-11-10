import { beforeEach, describe, expect, it, vi } from 'vitest'
import request from '../request'
import { getKnowledgeTags, listTags, replaceKnowledgeTags } from './tag'

vi.mock('../request', () => ({
  default: { get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn() },
}))

const mockedRequest = vi.mocked(request)
const page = { items: [{ id: 1, name: '工作', usage_count: 2 }], total: 1 }

describe('tag api', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('lists tags from the response envelope', async () => {
    mockedRequest.get.mockResolvedValueOnce({ data: { code: 0, msg: 'ok', data: page } })
    await expect(listTags()).resolves.toEqual(page)
    expect(mockedRequest.get).toHaveBeenCalledWith('/api/tags', { signal: undefined })
  })

  it('replaces knowledge tags with the full name set', async () => {
    mockedRequest.get.mockResolvedValueOnce({ data: { code: 0, msg: 'ok', data: page } })
    await expect(getKnowledgeTags(10)).resolves.toEqual(page)
    expect(mockedRequest.get).toHaveBeenCalledWith('/api/knowledge/10/tags', {
      signal: undefined,
    })

    mockedRequest.put.mockResolvedValueOnce({ data: { code: 0, msg: 'ok', data: page } })
    await expect(replaceKnowledgeTags(10, ['工作'])).resolves.toEqual(page)
    expect(mockedRequest.put).toHaveBeenCalledWith(
      '/api/knowledge/10/tags',
      { tags: ['工作'] },
      { signal: undefined },
    )
  })

  it('throws the server message when the envelope reports an error', async () => {
    mockedRequest.get.mockResolvedValueOnce({ data: { code: 1001, msg: '未登录', data: null } })
    await expect(listTags()).rejects.toThrow('未登录')
  })
})
