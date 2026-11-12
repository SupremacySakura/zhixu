/**
 * 消息提示总线：全仓唯一的发消息入口。
 *
 * 存在的理由是收掉「一个组件 + 一个组合式函数 + 一个 store 各发各的」这三条链路：
 * 任何地方（页面动作、请求拦截器、未来的新功能）都只调 toast()，由外壳里唯一
 * 挂载的提示条订阅并渲染，所以界面上永远只有一个提示。
 * 不引第三方库、不持有 UI 状态——它只负责把消息送出去。
 */
export type ToastType = 'success' | 'error' | 'info' | 'warning'
export type ToastMessage = { id: number; message: string; type: ToastType }
type Listener = (message: ToastMessage) => void

const listeners = new Set<Listener>()
let seq = 0

/** 发一条消息；无人订阅时静默丢弃（不排队、不重放）。 */
export function toast(message: string, type: ToastType = 'info'): void {
  const payload: ToastMessage = { id: ++seq, message, type }
  for (const listener of [...listeners]) listener(payload)
}

/** 订阅消息，返回取消订阅函数。 */
export function subscribe(listener: Listener): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}
