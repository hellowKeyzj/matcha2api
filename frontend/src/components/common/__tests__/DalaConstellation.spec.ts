import { afterEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import DalaConstellation from '../DalaConstellation.vue'

afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals() })

describe('DalaConstellation', () => {
  function setup(reduced = false) {
    const draw = vi.fn()
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue({ clearRect: draw, setTransform: vi.fn(), beginPath: vi.fn(), moveTo: vi.fn(), lineTo: vi.fn(), closePath: vi.fn(), stroke: vi.fn() } as unknown as CanvasRenderingContext2D)
    const disconnect = vi.fn()
    vi.stubGlobal('ResizeObserver', class { observe() {} disconnect = disconnect })
    vi.stubGlobal('IntersectionObserver', class { observe() {} disconnect = disconnect })
    const remove = vi.fn()
    vi.stubGlobal('matchMedia', () => ({ matches: reduced, addEventListener: vi.fn(), removeEventListener: remove }))
    const request = vi.fn(() => 7)
    const cancel = vi.fn()
    vi.stubGlobal('requestAnimationFrame', request)
    vi.stubGlobal('cancelAnimationFrame', cancel)
    return { draw, disconnect, remove, request, cancel }
  }
  it('renders a static constellation for reduced motion without scheduling frames', () => {
    const state = setup(true)
    const wrapper = mount(DalaConstellation)
    expect(state.draw).toHaveBeenCalled()
    expect(state.request).not.toHaveBeenCalled()
    wrapper.unmount()
    expect(state.disconnect).toHaveBeenCalledTimes(2)
    expect(state.remove).toHaveBeenCalledWith('change', expect.any(Function))
  })
  it('cancels the animation and observers on route exit', () => {
    const state = setup()
    const wrapper = mount(DalaConstellation)
    expect(state.request).toHaveBeenCalledTimes(1)
    wrapper.unmount()
    expect(state.cancel).toHaveBeenCalledWith(7)
    expect(state.disconnect).toHaveBeenCalledTimes(2)
  })
})
