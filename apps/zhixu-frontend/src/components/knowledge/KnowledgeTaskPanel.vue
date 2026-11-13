<script setup lang="ts">
import { AnimatePresence, Motion } from 'motion-v'
import AppIcon from '../ui/AppIcon.vue'
import type { KnowledgeItem } from '../../services/apis/knowledge'
import { springSnappy } from '../../utils/motion'
import KnowledgeBadge from './KnowledgeBadge.vue'
defineProps<{ item: KnowledgeItem | null; open: boolean; error: string; busy: boolean }>()
const emit = defineEmits<{ close: []; retry: []; refresh: [] }>()
</script>
<template>
  <Teleport to="body">
    <AnimatePresence>
      <Motion
        v-if="open && item"
        :initial="{ opacity: 0, y: 16, scale: 0.98 }"
        :animate="{ opacity: 1, y: 0, scale: 1 }"
        :exit="{ opacity: 0, y: 16 }"
        :transition="springSnappy"
        class="fixed bottom-4 right-4 z-30 w-80 max-w-[calc(100vw-2rem)] border border-hairline bg-surface p-5 shadow-floating"
      >
        <aside aria-label="知识导入进度">
          <div class="mb-4 flex items-center justify-between">
            <h2 class="text-body font-semibold">加入知识库</h2>
            <button
              type="button"
              class="atom-icon-btn"
              aria-label="关闭处理面板"
              @click="emit('close')"
            >
              <AppIcon icon="material-symbols:close-rounded" />
            </button>
          </div>
          <p class="mb-3 break-all text-body">{{ item.source_file_name }}</p>
          <KnowledgeBadge :status="item.status" />
          <p class="mt-4 text-body leading-6 text-label-secondary" role="status">
            {{
              item.status === 'ready'
                ? '正文已提取并保存，原文件保持不变。'
                : item.status === 'failed'
                  ? item.error_message
                  : item.status === 'pending'
                    ? '已提交，等待后台处理。关闭面板不会取消任务。'
                    : '正在提取 Markdown 正文并保存知识条目。'
            }}
          </p>
          <p v-if="error" class="mt-3 text-body text-warning-strong" role="alert">{{ error }}</p>
          <div class="mt-4 flex justify-end gap-2">
            <button v-if="error" type="button" class="atom-text-action" @click="emit('refresh')">
              重新查询
            </button>
            <button
              v-if="item.status === 'failed'"
              type="button"
              class="atom-btn atom-tone-secondary atom-btn-wide"
              :disabled="busy"
              @click="emit('retry')"
            >
              {{ busy ? '提交中…' : '重试加入' }}
            </button>
            <RouterLink
              v-if="item.status === 'ready'"
              :to="`/home/knowledge/${item.id}`"
              class="atom-btn atom-tone-tint atom-btn-wide"
              @click="emit('close')"
              >查看知识</RouterLink
            >
          </div>
        </aside>
      </Motion>
    </AnimatePresence>
  </Teleport>
</template>
