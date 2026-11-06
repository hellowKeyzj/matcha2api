import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import { provideComponentTheme, useComponentTheme } from '../useComponentTheme'
import BaseDialog from '@/components/common/BaseDialog.vue'

const Consumer = defineComponent({
  setup() {
    const theme = useComponentTheme()
    return () => h('span', { 'data-theme': theme }, theme)
  }
})

describe('application component theme', () => {
  it('flows through ordinary wrappers and teleported dialogs without page setup', () => {
    const Root = defineComponent({
      setup() {
        provideComponentTheme('theme-dala')
        return () => h('section', [h(Consumer), h(BaseDialog, { show: true, title: 'Global theme' }, { default: () => h(Consumer) })])
      }
    })
    const wrapper = mount(Root, { attachTo: document.body })
    expect(document.querySelector('.modal-content.theme-dala [data-theme="theme-dala"]')).not.toBeNull()
    expect(wrapper.find('[data-theme="theme-dala"]').exists()).toBe(true)
    wrapper.unmount()
  })

  it('does not require an application provider in isolated component consumers', () => {
    const wrapper = mount(Consumer)
    expect(wrapper.attributes('data-theme')).toBe('')
    wrapper.unmount()
  })
})
