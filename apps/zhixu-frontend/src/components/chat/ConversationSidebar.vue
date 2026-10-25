<script setup lang="ts">
import { onMounted, ref } from 'vue'
import {
  deleteConversation,
  listConversations,
  renameConversation,
  type ConversationSummary,
} from '../../services/apis/chat'

const emit = defineEmits<{
  (e: 'select', id: string): void
  (e: 'new'): void
}>()
const props = defineProps<{ activeId: string }>()

const conversations = ref<ConversationSummary[]>([])
const loading = ref(false)
const error = ref('')
const editingId = ref('')
const editingTitle = ref('')

async function load() {
  loading.value = true
  error.value = ''
  try {
    conversations.value = await listConversations()
  } catch {
    error.value = '会话列表加载失败'
  } finally {
    loading.value = false
  }
}
function startRename(conversation: ConversationSummary) {
  editingId.value = conversation.id
  editingTitle.value = conversation.title
}
async function commitRename() {
  const id = editingId.value
  const title = editingTitle.value.trim()
  editingId.value = ''
  if (!id || !title) return
  try {
    await renameConversation(id, title)
    const target = conversations.value.find(c => c.id === id)
    if (target) target.title = title
  } catch {
    error.value = '重命名失败'
  }
}
async function remove(id: string) {
  try {
    await deleteConversation(id)
    conversations.value = conversations.value.filter(c => c.id !== id)
    if (props.activeId === id) emit('new')
  } catch {
    error.value = '删除失败'
  }
}
function relative(iso: string) {
  const seconds = Math.max(0, (Date.now() - new Date(iso).getTime()) / 1000)
  if (seconds < 60) return '刚刚'
  if (seconds < 3600) return `${Math.floor(seconds / 60)} 分钟前`
  if (seconds < 86400) return `${Math.floor(seconds / 3600)} 小时前`
  return `${Math.floor(seconds / 86400)} 天前`
}
onMounted(load)
defineExpose({ reload: load })
</script>

<template>
  <aside
    class="flex h-full w-64 shrink-0 flex-col border-r border-hairline bg-surface-secondary"
    aria-label="会话历史"
  >
    <div class="flex items-center justify-between px-4 py-4">
      <h2 class="text-caption font-semibold tracking-wide text-label-secondary">会话历史</h2>
      <button type="button" class="knowledge-text-action" @click="emit('new')">＋ 新对话</button>
    </div>
    <div class="min-h-0 flex-1 overflow-y-auto px-2 pb-3">
      <p v-if="loading" role="status" class="px-2 py-4 text-caption text-label-secondary">
        加载中…
      </p>
      <p v-else-if="error" role="alert" class="px-2 py-4 text-caption text-danger">{{ error }}</p>
      <p v-else-if="!conversations.length" class="px-2 py-4 text-caption text-label-secondary">
        还没有历史会话，发起第一轮对话后自动保存。
      </p>
      <ul v-else class="space-y-1">
        <li v-for="conversation in conversations" :key="conversation.id">
          <div
            class="group rounded-md px-3 py-2"
            :class="conversation.id === activeId ? 'bg-primary-tint' : 'hover:bg-surface'"
          >
            <input
              v-if="editingId === conversation.id"
              v-model="editingTitle"
              maxlength="200"
              class="w-full rounded-sm border border-hairline bg-canvas px-2 py-1 text-caption outline-none"
              @keydown.enter.prevent="commitRename"
              @keydown.esc.prevent="editingId = ''"
              @blur="commitRename"
            />
            <template v-else>
              <button
                type="button"
                class="block w-full truncate text-left text-subhead"
                :class="
                  conversation.id === activeId ? 'text-primary-pressed' : 'text-label-primary'
                "
                @click="emit('select', conversation.id)"
              >
                {{ conversation.title || '未命名会话' }}
              </button>
              <div class="mt-1 flex items-center justify-between text-[11px] text-label-secondary">
                <span>{{ relative(conversation.updated_at) }}</span>
                <span class="hidden gap-2 group-hover:flex">
                  <button type="button" @click="startRename(conversation)">改名</button>
                  <button type="button" class="text-danger" @click="remove(conversation.id)">
                    删除
                  </button>
                </span>
              </div>
            </template>
          </div>
        </li>
      </ul>
    </div>
  </aside>
</template>
