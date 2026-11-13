<script setup lang="ts">
import { computed, ref } from 'vue'
import { downloadByPickupCode } from '../services/apis/file'
import { formatBytes, iconForListItem } from '../utils/file'
import AppIcon from '../components/ui/AppIcon.vue'

const codeDigits = ref<string[]>(Array.from({ length: 6 }, () => ''))
const inputRefs = ref<(HTMLInputElement | null)[]>(Array.from({ length: 6 }, () => null))
const isLoading = ref(false)
const errorMessage = ref('')
const successState = ref(false)

const downloadedFile = ref({
  name: '',
  size: '',
  typeIcon: '',
  iconBg: '',
  iconFg: '',
  extLabel: '',
})

const getIconForContentType = (filename: string, contentType: string) => {
  if (contentType === 'application/zip' || filename.endsWith('.zip')) {
    return { ...iconForListItem({ type: 'folder', file_type: '' }), ext: 'ZIP' }
  }

  // 简易映射成 FileType 给工具函数使用
  let fileType = 'other'
  if (contentType.startsWith('video/')) fileType = 'video'
  else if (contentType.startsWith('image/')) fileType = 'image'
  else if (contentType.startsWith('audio/')) fileType = 'audio'
  else if (
    contentType.includes('pdf') ||
    contentType.includes('word') ||
    contentType.includes('excel') ||
    contentType.includes('powerpoint') ||
    contentType.startsWith('text/')
  )
    fileType = 'document'

  const iconMeta = iconForListItem({ type: 'file', file_type: fileType })
  const ext = filename.split('.').pop()?.toUpperCase() || 'FILE'

  return { ...iconMeta, ext: ext.substring(0, 4) }
}

const normalizedCode = computed(() => codeDigits.value.join('').toUpperCase())
const canSubmit = computed(() => normalizedCode.value.length === 6 && !isLoading.value)

const setInputRef = (el: HTMLInputElement | null, index: number) => {
  inputRefs.value[index] = el
}

const focusInput = (index: number) => {
  const target = inputRefs.value[index]
  if (target) {
    target.focus()
    target.select()
  }
}

const updateDigit = (index: number, value: string) => {
  const next = value
    .replace(/[^a-zA-Z0-9]/g, '')
    .slice(-1)
    .toUpperCase()
  codeDigits.value[index] = next
}

const handleInput = (index: number, event: Event) => {
  const target = event.target as HTMLInputElement
  updateDigit(index, target.value)
  target.value = codeDigits.value[index]
  if (codeDigits.value[index] && index < 5) {
    focusInput(index + 1)
  }
}

const handleKeydown = (index: number, event: KeyboardEvent) => {
  if (event.key === 'Backspace' && !codeDigits.value[index] && index > 0) {
    focusInput(index - 1)
  }
  if (event.key === 'ArrowLeft' && index > 0) {
    event.preventDefault()
    focusInput(index - 1)
  }
  if (event.key === 'ArrowRight' && index < 5) {
    event.preventDefault()
    focusInput(index + 1)
  }
}

const handlePaste = (event: ClipboardEvent) => {
  event.preventDefault()
  const text = event.clipboardData?.getData('text') ?? ''
  const chars = text
    .toUpperCase()
    .replace(/[^A-Z0-9]/g, '')
    .slice(0, 6)
    .split('')
  if (chars.length === 0) return
  codeDigits.value = Array.from({ length: 6 }, (_, idx) => chars[idx] ?? '')
  focusInput(Math.min(chars.length, 6) - 1)
}

const resetFeedback = () => {
  errorMessage.value = ''
  successState.value = false
}

const mapErrorMessage = (message: string) => {
  if (message.includes('参数')) return '取件码格式不正确，请输入 6 位字母或数字。'
  if (message.includes('不存在')) return '取件码不存在或对应资源已删除。'
  if (message.includes('服务器')) return '服务器处理失败，请稍后重试。'
  if (message.includes('下载')) return message
  return '取件失败，请检查取件码后重试。'
}

const handleRetrieve = async () => {
  if (!canSubmit.value) return
  resetFeedback()
  isLoading.value = true
  try {
    const { fileName, fileSize, contentType } = await downloadByPickupCode(normalizedCode.value)
    const iconInfo = getIconForContentType(fileName, contentType)

    downloadedFile.value = {
      name: fileName,
      size: formatBytes(fileSize),
      typeIcon: iconInfo.icon,
      iconBg: iconInfo.bg,
      iconFg: iconInfo.fg,
      extLabel: iconInfo.ext,
    }

    successState.value = true
  } catch (error) {
    const message = error instanceof Error ? error.message : '取件失败，请稍后重试。'
    errorMessage.value = mapErrorMessage(message)
  } finally {
    isLoading.value = false
  }
}
</script>

<template>
  <div class="flex min-h-screen flex-col">
    <main class="flex flex-1 items-center justify-center p-4 sm:p-8 lg:p-12">
      <!-- Main Container -->
      <div class="w-full max-w-[480px] space-y-8">
        <!-- Entry Card -->
        <div class="bg-surface p-6 shadow-card sm:p-8">
          <div class="mb-7 text-center">
            <h1 class="text-display mb-2 text-label">文件取件</h1>
            <p class="text-body text-label-secondary">请输入您的 6 位提取码获取文件</p>
          </div>
          <div class="space-y-6">
            <!-- 6 位取件码：输入自动跳格 / Backspace 回退 / 方向键移动 / 粘贴分发 -->
            <div class="grid grid-cols-6 gap-2 sm:gap-2.5" :class="errorMessage && 'code-shake'">
              <input
                v-for="(_, index) in codeDigits"
                :key="index"
                :ref="el => setInputRef(el as HTMLInputElement | null, index)"
                :aria-label="`取件码第${index + 1}位`"
                class="aspect-square min-w-0 w-full cursor-default border-[1.5px] text-center font-mono text-title-3 uppercase text-label caret-primary transition-[border-color,background-color,box-shadow] placeholder:text-label-tertiary"
                :class="
                  errorMessage
                    ? 'border-danger bg-surface ring-[3px] ring-danger-tint'
                    : 'border-hairline bg-surface-secondary focus:border-primary focus:bg-surface'
                "
                maxlength="1"
                inputmode="text"
                placeholder="·"
                type="text"
                :autofocus="index === 0"
                :value="codeDigits[index]"
                @input="handleInput(index, $event)"
                @keydown="handleKeydown(index, $event)"
                @paste="handlePaste"
              />
            </div>
            <p v-if="errorMessage" class="text-center text-body font-medium text-danger">
              {{ errorMessage }}
            </p>
            <button
              aria-label="提取文件"
              class="flex h-14 w-full items-center justify-center gap-2 text-headline transition-colors disabled:cursor-not-allowed disabled:opacity-60 knowledge-button-primary"
              :disabled="!canSubmit"
              @click="handleRetrieve"
            >
              <AppIcon icon="material-symbols:key" />
              {{ isLoading ? '提取中...' : '提取文件' }}
            </button>
          </div>
          <div
            class="mt-6 flex items-center justify-center gap-1.5 border-t border-hairline pt-4 text-caption text-label-tertiary"
          >
            <AppIcon
              icon="material-symbols:lock-clock"
              class="atom-glyph"
              :style="{ '--atom-glyph-size': '14px' }"
            />
            仅限有效期内提取
          </div>
        </div>

        <!-- Success State Section -->
        <div v-if="successState" class="space-y-4">
          <div class="flex items-center gap-2 px-2 text-primary">
            <AppIcon
              icon="material-symbols:check-circle"
              class="atom-glyph"
              :style="{ '--atom-glyph-size': '18px' }"
            />
            <h3 class="text-headline">已提取文件</h3>
          </div>
          <div
            class="flex flex-col items-center gap-5 bg-surface p-6 shadow-card sm:flex-row sm:gap-6"
          >
            <div
              class="relative flex h-20 w-16 flex-none items-center justify-center overflow-hidden border border-current/20"
              :class="[downloadedFile.iconBg, downloadedFile.iconFg]"
            >
              <div class="absolute inset-0 bg-current opacity-5"></div>
              <AppIcon
                :icon="downloadedFile.typeIcon"
                class="atom-glyph"
                :style="{ '--atom-glyph-size': '36px' }"
              />
              <div
                class="absolute bottom-1 right-1 bg-current px-1 text-note font-bold text-white"
                :class="downloadedFile.iconFg.replace('text-', 'bg-')"
              >
                {{ downloadedFile.extLabel }}
              </div>
            </div>
            <div class="min-w-0 flex-1 overflow-hidden text-center sm:text-left">
              <h4 class="truncate text-title-3 text-label" :title="downloadedFile.name">
                {{ downloadedFile.name }}
              </h4>
              <p class="mt-1 text-subhead text-label-secondary">
                {{ downloadedFile.size }} • 下载成功
              </p>
            </div>
            <button
              aria-label="重新下载文件"
              class="flex h-12 w-full items-center justify-center gap-2 bg-label px-6 text-headline text-white transition-colors hover:bg-label/85 sm:w-auto"
              @click="handleRetrieve"
            >
              <AppIcon
                icon="material-symbols:download"
                class="atom-glyph"
                :style="{ '--atom-glyph-size': '20px' }"
              />
              重新下载
            </button>
          </div>
        </div>

        <!-- Security Footer -->
        <div class="border-t border-hairline py-4 text-center">
          <p class="text-caption leading-relaxed text-label-tertiary">
            传输过程全程加密，提取码过期后将无法下载文件。
          </p>
        </div>
      </div>
    </main>
  </div>
</template>

<style scoped>
/* 失败 shake：0 → -8 → 8 → -5 → 5 → 0，共 400ms（DESIGN.md §5.2 FilePickup） */
@keyframes pickup-code-shake {
  0%,
  100% {
    transform: translateX(0);
  }
  20% {
    transform: translateX(-8px);
  }
  40% {
    transform: translateX(8px);
  }
  60% {
    transform: translateX(-5px);
  }
  80% {
    transform: translateX(5px);
  }
}

.code-shake {
  animation: pickup-code-shake 400ms cubic-bezier(0.65, 0, 0.35, 1);
}

@media (prefers-reduced-motion: reduce) {
  .code-shake {
    animation: none;
  }
}
</style>
