import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'
import WorkspaceParticles from '../WorkspaceParticles.vue'

afterEach(() => vi.restoreAllMocks())

describe('WorkspaceParticles', () => {
  it('renders bounded decorative layers without interactive content', () => {
    const wrapper = mount(WorkspaceParticles)
    expect(wrapper.attributes('aria-hidden')).toBe('true')
    expect(wrapper.findAll('svg')).toHaveLength(3)
    expect(wrapper.findAll('circle')).toHaveLength(72)
    wrapper.unmount()
  })

  it('pauses when hidden, resumes when visible and removes its listener', async () => {
    const hidden = vi.spyOn(document, 'hidden', 'get').mockReturnValue(true)
    const remove = vi.spyOn(document, 'removeEventListener')
    const wrapper = mount(WorkspaceParticles)
    await nextTick()
    expect(wrapper.classes()).toContain('is-paused')
    hidden.mockReturnValue(false)
    document.dispatchEvent(new Event('visibilitychange'))
    await nextTick()
    expect(wrapper.classes()).not.toContain('is-paused')
    wrapper.unmount()
    expect(remove).toHaveBeenCalledWith('visibilitychange', expect.any(Function))
  })
})
