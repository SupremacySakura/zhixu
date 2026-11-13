import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import AddKnowledgeDialog from '../../components/knowledge/AddKnowledgeDialog.vue'
vi.mock('../../components/ui/AppIcon.vue', () => ({ default: { template: '<span />' } }))
describe('AddKnowledgeDialog', () => {
  it('opens a modal, submits, and restores focus on close', async () => {
    const trigger = document.createElement('button')
    document.body.append(trigger)
    trigger.focus()
    const wrapper = mount(AddKnowledgeDialog, {
      props: {
        file: { id: 1, name: 'note.md', size: 10 } as never,
        title: '笔记',
        busy: false,
        error: '',
      },
      attachTo: document.body,
    })
    await flushPromises()
    const dialog = document.querySelector('dialog')!
    expect(dialog.open).toBe(true)
    expect(document.activeElement?.tagName).toBe('INPUT')
    dialog
      .querySelector('form')!
      .dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    expect(wrapper.emitted('submit')).toHaveLength(1)
    wrapper.unmount()
    expect(document.activeElement).toBe(trigger)
    trigger.remove()
  })
})
