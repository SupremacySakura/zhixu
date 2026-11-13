import axios from 'axios'
import { useUserStore } from '../stores/user'
import { toast } from '../utils/toastBus'

// 全局错误提示走唯一的总线链路：由应用外壳里唯一挂载的提示条渲染。
// 同时保留控制台警告——它不依赖任何 UI，是排查线上问题时的最后一份证据。
function showToast(message: string) {
  toast(message, 'error')
  console.warn('Toast: ', message)
}

function redirectToLogin() {
  // 使用硬跳转，确保路由跳转在拦截器中可控
  window.location.href = '/login'
}

// 请求实例
const request = axios.create({
  timeout: 30000, // 请求超时 30s
})

// 请求拦截：附带 token
request.interceptors.request.use(
  config => {
    const token = useUserStore().token
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  error => {
    return Promise.reject(error)
  },
)

request.interceptors.response.use(
  config => {
    // 更新 token/refreshToken 时机：登录、注册、刷新
    const url = config?.config?.url
    if (
      url &&
      (url.includes('/auth/login') ||
        url.includes('/auth/register') ||
        url.includes('/auth/refresh'))
    ) {
      const data = config?.data?.data || {}
      if (data?.token) {
        useUserStore().setToken(data.token)
      }
      if (data?.refreshToken && (useUserStore() as any).setRefreshToken) {
        ;(useUserStore() as any).setRefreshToken(data.refreshToken)
      }
    }
    return config
  },
  async error => {
    const { config, response, message, code } = error
    const status = response?.status

    // 全局错误处理
    if (status === 403) {
      showToast('无权限')
      return Promise.reject(error)
    }
    if (status === 500) {
      showToast('服务器错误')
      return Promise.reject(error)
    }

    // 网络错误重试：指数退避，最多2次
    const isNetwork =
      !response && (code === 'ECONNABORTED' || (message ?? '').toString().includes('Network Error'))
    if (isNetwork) {
      ;(config as any).__retryCount = (config as any).__retryCount || 0
      if ((config as any).__retryCount < 2) {
        ;(config as any).__retryCount += 1
        const backoff = Math.pow(2, (config as any).__retryCount) * 1000
        await new Promise(resolve => setTimeout(resolve, backoff))
        return request(config)
      }
    }

    // 401：后端当前只提供短期 access token，直接清理登录态
    if (status === 401) {
      showToast('请登录')
      useUserStore().logout()
      redirectToLogin()
      return Promise.reject(error)
    }

    return Promise.reject(error)
  },
)

export default request
