import { onScopeDispose, ref } from 'vue'
import {
  addKnowledge,
  getKnowledge,
  retryKnowledge,
  type KnowledgeItem,
} from '../services/apis/knowledge'
import type { FileDisplayItem, ToastType } from '../types/file'
import { isKnowledgeActive, knowledgeError, stripKnowledgeExtension } from '../utils/knowledge'

export function useKnowledgeImport(notify: (message: string, type?: ToastType) => void) {
  const target = ref<FileDisplayItem | null>(null)
  const title = ref('')
  const error = ref('')
  const busy = ref(false)
  const item = ref<KnowledgeItem | null>(null)
  const panelOpen = ref(false)
  const panelError = ref('')
  const revision = ref(0)
  let controller: AbortController | null = null
  let mutation: AbortController | null = null
  let timer: ReturnType<typeof setTimeout> | undefined
  let disposed = false
  const stop = () => {
    controller?.abort()
    clearTimeout(timer)
  }
  async function poll() {
    stop()
    if (!item.value || disposed) return
    const current = new AbortController()
    controller = current
    try {
      const data = await getKnowledge(item.value.id, current.signal)
      if (current.signal.aborted || disposed) return
      item.value = data
      panelError.value = ''
      revision.value++
      if (isKnowledgeActive(data.status)) timer = setTimeout(poll, 2500)
    } catch (e) {
      if (!current.signal.aborted && !disposed) panelError.value = knowledgeError(e)
    }
  }
  function track(data: KnowledgeItem) {
    stop()
    item.value = data
    panelOpen.value = true
    panelError.value = ''
    revision.value++
    if (isKnowledgeActive(data.status)) timer = setTimeout(poll, 1500)
  }
  function open(file: FileDisplayItem) {
    target.value = file
    title.value = stripKnowledgeExtension(file.name)
    error.value = ''
  }
  async function submit() {
    if (!target.value || busy.value) return
    const value = title.value.trim()
    if (!value || Array.from(value).length > 200) {
      error.value = '请输入 1–200 字符的知识标题'
      return
    }
    busy.value = true
    error.value = ''
    mutation = new AbortController()
    try {
      const data = await addKnowledge(target.value.id, value, mutation.signal)
      if (disposed) return
      target.value = null
      track(data)
      notify(
        isKnowledgeActive(data.status) ? '已提交加入任务，原文件保持不变' : '此文件已有知识条目',
        'success',
      )
    } catch (e) {
      if (!disposed) error.value = knowledgeError(e)
    } finally {
      busy.value = false
    }
  }
  async function retry() {
    if (!item.value || busy.value) return
    busy.value = true
    mutation = new AbortController()
    try {
      const data = await retryKnowledge(item.value.id, mutation.signal)
      if (disposed) return
      track(data)
      notify('已重新提交加入任务', 'info')
    } catch (e) {
      if (!disposed) panelError.value = knowledgeError(e)
    } finally {
      busy.value = false
    }
  }
  onScopeDispose(() => {
    disposed = true
    stop()
    mutation?.abort()
  })
  return {
    target,
    title,
    error,
    busy,
    item,
    panelOpen,
    panelError,
    revision,
    open,
    submit,
    track,
    retry,
    poll,
  }
}
