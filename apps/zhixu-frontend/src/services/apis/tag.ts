import request from '../request'
import type { ResponseData } from '../types'

export interface Tag {
  id: number
  name: string
  usage_count: number
}

export interface TagPage {
  items: Tag[]
  total: number
}

function unwrap<T>(data: ResponseData<T>): T {
  if (data.code !== 0 || data.data == null) throw new Error(data.msg || '标签请求失败')
  return data.data
}

export async function listTags(signal?: AbortSignal) {
  return unwrap((await request.get<ResponseData<TagPage>>('/api/tags', { signal })).data)
}

export async function createTag(name: string, signal?: AbortSignal) {
  return unwrap((await request.post<ResponseData<Tag>>('/api/tags', { name }, { signal })).data)
}

export async function renameTag(id: number, name: string, signal?: AbortSignal) {
  return unwrap(
    (await request.put<ResponseData<Tag>>(`/api/tags/${id}`, { name }, { signal })).data,
  )
}

export async function deleteTag(id: number, signal?: AbortSignal) {
  return unwrap(
    (await request.delete<ResponseData<{ id: number }>>(`/api/tags/${id}`, { signal })).data,
  )
}

export async function getKnowledgeTags(knowledgeId: number, signal?: AbortSignal) {
  return unwrap(
    (await request.get<ResponseData<TagPage>>(`/api/knowledge/${knowledgeId}/tags`, { signal }))
      .data,
  )
}

export async function replaceKnowledgeTags(
  knowledgeId: number,
  names: string[],
  signal?: AbortSignal,
) {
  return unwrap(
    (
      await request.put<ResponseData<TagPage>>(
        `/api/knowledge/${knowledgeId}/tags`,
        { tags: names },
        { signal },
      )
    ).data,
  )
}
