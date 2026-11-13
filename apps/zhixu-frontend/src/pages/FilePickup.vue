<script setup lang="ts">
import { computed, ref } from 'vue'
import { downloadByPickupCode } from '../services/apis/file'
import { formatBytes, iconForListItem } from '../utils/file'
import AppButton from '../components/ui/AppButton.vue'
import AppIcon from '../components/ui/AppIcon.vue'
import BrandMark from '../components/ui/BrandMark.vue'

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
  <!-- 取件页：一个入口动作 + 一张结果卡。
       六位取件码是「填空」不是「六个盒子」——每格只画下沿一条墨线，
       六格落在同一条水平线上，所以整屏只多一条线（DESIGN.md 硬上限 ≤5）。 -->
  <div class="flex min-h-screen flex-col">
    <main
      id="main-content"
      tabindex="-1"
      class="flex flex-1 flex-col items-center justify-center px-4 py-12"
    >
      <div class="w-full max-w-[480px]">
        <div class="mb-6 flex items-center">
          <BrandMark />
        </div>

        <!-- 入口卡 -->
        <div class="cabinet-card p-6 sm:p-8">
          <div class="mb-8 text-center">
            <h1 class="text-display mb-2">文件取件</h1>
            <p class="text-body text-label-secondary">请输入您的 6 位提取码获取文件</p>
          </div>

          <div class="space-y-6">
            <!-- 6 位取件码：输入自动跳格 / Backspace 回退 / 方向键移动 / 粘贴分发。
                 六个格子共用**一条**下沿墨线（填空，不是六个盒子）——
                 整屏因此只多一条线，且失败时抖的是这条线连同码字，不是六个框各自抖。 -->
            <div
              class="pickup-line"
              :class="{ 'is-error': errorMessage, 'code-shake': errorMessage }"
            >
              <div class="grid grid-cols-6 gap-1 sm:gap-2">
                <input
                  v-for="(_, index) in codeDigits"
                  :key="index"
                  :ref="el => setInputRef(el as HTMLInputElement | null, index)"
                  :aria-label="`取件码第${index + 1}位`"
                  class="pickup-cell h-14 w-full min-w-0 cursor-default border-0 bg-transparent text-center font-type text-title uppercase text-label caret-primary placeholder:text-label-tertiary"
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
            </div>

            <p v-if="errorMessage" class="text-center text-body font-medium text-danger">
              {{ errorMessage }}
            </p>

            <AppButton
              aria-label="提取文件"
              tone="primary"
              size="lg"
              icon="material-symbols:key"
              class="w-full"
              style="--atom-control-height: 48px"
              :disabled="!canSubmit"
              :loading="isLoading"
              @click="handleRetrieve"
            >
              {{ isLoading ? '提取中...' : '提取文件' }}
            </AppButton>
          </div>

          <!-- 卡内页脚：与动作是同级块，按 Hairline 纪律用留白分组，不画线 -->
          <div class="mt-7 flex items-center justify-center gap-1.5 text-note text-label-tertiary">
            <AppIcon
              icon="material-symbols:lock-clock"
              class="atom-glyph"
              :style="{ '--atom-glyph-size': '14px' }"
            />
            仅限有效期内提取
          </div>
        </div>

        <!-- 结果卡：成功才出现，与入口卡同宽同材质 -->
        <div v-if="successState" class="mt-6 cabinet-card p-6 sm:p-8">
          <div class="mb-5 flex items-center gap-2 text-primary">
            <AppIcon
              icon="material-symbols:check-circle"
              class="atom-glyph"
              :style="{ '--atom-glyph-size': '18px' }"
            />
            <h3 class="text-title">已提取文件</h3>
          </div>
          <div class="flex flex-col gap-5 sm:flex-row sm:items-center sm:gap-6">
            <div
              class="relative flex h-20 w-16 flex-none items-center justify-center"
              :class="[downloadedFile.iconBg, downloadedFile.iconFg]"
            >
              <AppIcon
                :icon="downloadedFile.typeIcon"
                class="atom-glyph"
                :style="{ '--atom-glyph-size': '32px' }"
              />
              <span
                class="absolute bottom-0 right-0 bg-label px-1 text-micro text-surface-tertiary"
              >
                {{ downloadedFile.extLabel }}
              </span>
            </div>
            <div class="min-w-0 flex-1 text-center sm:text-left">
              <h4 class="truncate text-title" :title="downloadedFile.name">
                {{ downloadedFile.name }}
              </h4>
              <p class="mt-1 text-note text-label-secondary">
                {{ downloadedFile.size }} • 下载成功
              </p>
            </div>
            <AppButton
              aria-label="重新下载文件"
              tone="secondary"
              size="lg"
              icon="material-symbols:download"
              class="w-full sm:w-auto"
              style="--atom-control-height: 44px"
              @click="handleRetrieve"
            >
              重新下载
            </AppButton>
          </div>
        </div>

        <!-- 页脚不是一条带：没有底色、没有分隔线，只是一句安静的说明 -->
        <p class="mt-6 text-center text-note text-label-tertiary">
          传输过程全程加密，提取码过期后将无法下载文件。
        </p>
      </div>
    </main>
  </div>
</template>

<style scoped>
/* 失败 shake：0 → -8 → 8 → -5 → 5 → 0。
   时长走世界的 sheet 档（220ms）+ settle 曲线；位移只有 translateX，
   没有回弹，落地即停。reduced-motion 由 src/style.css 全局兜底，
   这里不再重复一份（重复的那份是死代码）。 */
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
  animation: pickup-code-shake 220ms var(--ease-settle);
}

/* 填空式码行：六格共用一条下沿墨线。
   线画在容器上而不是每格上 —— 六段各自成线就是六条水平线，
   而一条连续的线只算一条（DESIGN.md 硬上限 ≤5）。
   失败时整行换危险色并抖动，抖的是这条线连同码字。 */
.pickup-line {
  border-bottom: 1.5px solid var(--color-rule);
  transition-property: border-color;
  transition-duration: var(--transition-duration-standard);
  transition-timing-function: var(--ease-settle);

  &.is-error {
    border-bottom-color: var(--color-danger);
  }

  /* 焦点在任意一格时，整条线换成结构蓝：这是「当前正在填这一行」的提示，
     比给单格加一个框更符合填空的语义。 */
  &:focus-within:not(.is-error) {
    border-bottom-color: var(--color-primary);
  }
}

/* 填空式码格：iOS 上 input 默认有内阴影与最小行高，先清干净，
   否则下沿那条墨线会被撑出第二条边。 */
.pickup-cell {
  -webkit-appearance: none;
  appearance: none;
  padding-inline: 0;
  background-color: transparent;
}

/* 触屏设备上 56px 已经够，但竖屏手机横向 6 格会挤到 44px 以下，
   给一个下限保住热区。 */
@media (pointer: coarse) {
  .pickup-cell {
    min-height: 44px;
  }
}
</style>
