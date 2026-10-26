import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import ServiceStatusPanel from '../../components/ui/ServiceStatusPanel.vue'
import type { ServiceHealth } from '../../services/apis/serviceStatus'

const labels: [ServiceHealth, string][] = [
  ['healthy', '运行正常'],
  ['unconfigured', '未配置'],
  ['unauthorized', '认证失败'],
  ['unreachable', '无法连接'],
  ['timeout', '检测超时'],
  ['unhealthy', '服务异常'],
]
describe('ServiceStatusPanel', () => {
  it.each(labels)(
    'renders %s as accessible text, including the server message',
    (status, label) => {
      const wrapper = mount(ServiceStatusPanel, {
        props: {
          report: {
            checked_at: '2026-10-26T08:00:00Z',
            services: [
              { id: 'mastra', status, latency_ms: 0, message: '<script>not executed</script>' },
              { id: 'qdrant', status, latency_ms: null, message: '安全状态说明' },
            ],
          },
          loading: false,
          error: '',
          stale: false,
        },
      })
      expect(wrapper.text()).toContain(label)
      expect(wrapper.text()).toContain('0 ms')
      expect(wrapper.text()).not.toContain('null ms')
      expect(wrapper.find('script').exists()).toBe(false)
      expect(wrapper.text()).toContain('<script>not executed</script>')
      expect(wrapper.get('time').attributes('datetime')).toBe('2026-10-26T08:00:00Z')
      wrapper.unmount()
    },
  )

  it('disables refresh during loading, emits a manual refresh, and labels stale results on failure', async () => {
    const wrapper = mount(ServiceStatusPanel, {
      props: { report: null, loading: true, error: '', stale: false },
    })
    expect(wrapper.get('button').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[role="status"]').text()).toContain('正在检测')
    expect(wrapper.text()).not.toContain('运行正常')
    await wrapper.setProps({ loading: false })
    await wrapper.get('button').trigger('click')
    expect(wrapper.emitted('refresh')).toHaveLength(1)
    await wrapper.setProps({
      report: { checked_at: '2026-10-26T08:00:00Z', services: [] },
      error: '检测失败',
      stale: true,
    })
    expect(wrapper.get('[role="alert"]').text()).toContain('旧结果，不代表最新服务状态')
    expect(wrapper.get('time').attributes('datetime')).toBe('2026-10-26T08:00:00Z')
    wrapper.unmount()
  })
})
