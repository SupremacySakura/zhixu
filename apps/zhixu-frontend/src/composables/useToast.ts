import { onScopeDispose, ref } from 'vue'
import type { ToastType } from '../types/file'
import { toast as publish } from '../utils/toastBus'

/**
 * 五个导出接口的签名与语义保持不变（有测试覆盖）：本地 ref 与计时器仍按
 * 「后来的消息替换前一条、到点自动收起、随作用域销毁」工作；唯一的增量是
 * displayToast 额外把消息发到总线上，由外壳里唯一挂载的提示条渲染。
 */
export function useToast(duration = 3000) {
  const toastMessage = ref('')
  const toastType = ref<ToastType>('info')
  const showToast = ref(false)
  let timer: ReturnType<typeof setTimeout> | null = null

  const closeToast = () => {
    showToast.value = false
    if (timer) {
      clearTimeout(timer)
      timer = null
    }
  }

  const displayToast = (message: string, type: ToastType = 'info') => {
    if (timer) clearTimeout(timer)
    toastMessage.value = message
    toastType.value = type
    showToast.value = true
    timer = setTimeout(closeToast, duration)
    publish(message, type)
  }

  onScopeDispose(closeToast)

  return {
    toastMessage,
    toastType,
    showToast,
    displayToast,
    closeToast,
  }
}
