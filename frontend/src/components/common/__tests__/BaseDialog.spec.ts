import { afterEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import BaseDialog from '../BaseDialog.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

describe('BaseDialog', () => {
  it('applies panel styling to the teleported panel without replacing its width', () => {
    const wrapper = mount(BaseDialog, {
      attachTo: document.body,
      props: { show: true, title: 'Account', width: 'wide', panelClass: 'account-editor' },
      global: { stubs: { Icon: true } }
    })

    const panel = document.body.querySelector('.modal-content')
    expect(panel?.classList.contains('account-editor')).toBe(true)
    expect(panel?.classList.contains('lg:max-w-4xl')).toBe(true)
    expect(document.body.querySelector('[role="dialog"]')?.getAttribute('aria-labelledby'))
      .toBe(document.body.querySelector('.modal-title')?.id)
    wrapper.unmount()
  })

  afterEach(() => {
    document.body.innerHTML = ''
    document.body.classList.remove('modal-open')
  })

  it('resets body scroll position when reopened', async () => {
    const wrapper = mount(BaseDialog, {
      attachTo: document.body,
      props: { show: false, title: 'Details' },
      slots: { default: '<div style="height: 2000px">content</div>' },
      global: { stubs: { Icon: true } }
    })

    await wrapper.setProps({ show: true })
    await nextTick()
    const body = document.body.querySelector<HTMLElement>('.modal-body')
    expect(body).not.toBeNull()
    body!.scrollTop = 480

    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true })
    await nextTick()

    expect(document.body.querySelector<HTMLElement>('.modal-body')?.scrollTop).toBe(0)
    wrapper.unmount()
  })
})
