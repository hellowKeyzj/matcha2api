import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import DalaConstellation from '../DalaConstellation.vue'

const renderer = vi.hoisted(() => ({ resize: vi.fn(), draw: vi.fn(), dispose: vi.fn() }))
vi.mock('../constellationScene', async importOriginal => {
  const original = await importOriginal<typeof import('../constellationScene')>()
  return { ...original, decodeConstellation: vi.fn(() => ({})), createConstellationRenderer: () => renderer }
})

afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); vi.clearAllMocks() })

function setup(reduced = false) {
  const rect = vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue({ width: 1280, height: 720 } as DOMRect)
  const disconnect = vi.fn()
  let resize: ResizeObserverCallback
  let intersection: IntersectionObserverCallback
  vi.stubGlobal('ResizeObserver', class {
    constructor(callback: ResizeObserverCallback) { resize = callback }
    observe() {}
    disconnect = disconnect
  })
  vi.stubGlobal('IntersectionObserver', class {
    constructor(callback: IntersectionObserverCallback) { intersection = callback }
    observe() {}
    disconnect = disconnect
  })
  const motion = { matches: reduced, addEventListener: vi.fn(), removeEventListener: vi.fn() }
  vi.stubGlobal('matchMedia', () => motion)
  const request = vi.fn(() => 7)
  const cancel = vi.fn()
  vi.stubGlobal('requestAnimationFrame', request)
  vi.stubGlobal('cancelAnimationFrame', cancel)
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, arrayBuffer: async () => new ArrayBuffer(0) }))
  const hidden = vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
  return {
    disconnect, motion, request, cancel, hidden,
    setVisible: (value: boolean) => intersection([{ isIntersecting: value } as IntersectionObserverEntry], {} as IntersectionObserver),
    setSize: (width: number, height: number) => {
      rect.mockReturnValue({ width, height } as DOMRect)
      resize([], {} as ResizeObserver)
    },
  }
}

describe('DalaConstellation lifecycle', () => {
  it('loads local geometry and releases the renderer, observers, frame and listeners', async () => {
    const state = setup()
    const remove = vi.spyOn(window, 'removeEventListener')
    const wrapper = mount(DalaConstellation)
    await flushPromises()
    expect(wrapper.attributes('aria-hidden')).toBe('true')
    expect(fetch).toHaveBeenCalledWith(expect.stringContaining('constellation.bin'), expect.objectContaining({ signal: expect.any(AbortSignal) }))
    expect(state.request).toHaveBeenCalledOnce()
    expect(renderer.resize).toHaveBeenCalledWith(1280, 720, expect.any(Number), .5)
    wrapper.unmount()
    expect(state.cancel).toHaveBeenCalledWith(7)
    expect(renderer.dispose).toHaveBeenCalledOnce()
    expect(state.disconnect).toHaveBeenCalledTimes(2)
    expect(state.motion.removeEventListener).toHaveBeenCalledWith('change', expect.any(Function))
    expect(remove).toHaveBeenCalledWith('pointermove', expect.any(Function))
    expect(remove).toHaveBeenCalledWith('blur', expect.any(Function))
  })

  it('renders reduced motion statically but updates theme and scroll poses', async () => {
    const state = setup(true)
    const wrapper = mount(DalaConstellation, { props: { progress: 0 } })
    await flushPromises()
    expect(state.request).not.toHaveBeenCalled()
    renderer.draw.mockClear()
    await wrapper.setProps({ light: true, progress: 3 })
    expect(renderer.draw).toHaveBeenCalledWith(0, expect.objectContaining({ morph: 1, entrance: 1 }), true, { x: 0, y: 0 })
    wrapper.unmount()
  })

  it('positions auth artwork inside a full-width canvas instead of narrowing its viewport', async () => {
    const state = setup()
    const wrapper = mount(DalaConstellation, { props: { align: 'left' } })
    await flushPromises()
    expect(renderer.resize).toHaveBeenLastCalledWith(1280, 720, expect.any(Number), .34)
    state.setSize(900, 987)
    expect(renderer.resize).toHaveBeenLastCalledWith(900, 987, expect.any(Number), .5)
    state.setSize(390, 844)
    expect(renderer.resize).toHaveBeenLastCalledWith(390, 844, expect.any(Number), .5)
    state.setSize(1169, 987)
    expect(renderer.resize).toHaveBeenLastCalledWith(1169, 987, expect.any(Number), .34)
    await wrapper.setProps({ align: 'center' })
    expect(renderer.resize).toHaveBeenLastCalledWith(1169, 987, expect.any(Number), .5)
    wrapper.unmount()
  })

  it('pauses offscreen, hidden and on context loss; resumes with a single loop', async () => {
    const state = setup()
    const wrapper = mount(DalaConstellation)
    await flushPromises()
    state.setVisible(false)
    state.request.mockClear()
    expect(state.cancel).toHaveBeenCalledWith(7)
    state.setVisible(true)
    expect(state.request).toHaveBeenCalledOnce()
    state.hidden.mockReturnValue(true)
    document.dispatchEvent(new Event('visibilitychange'))
    state.request.mockClear()
    document.dispatchEvent(new Event('visibilitychange'))
    expect(state.request).not.toHaveBeenCalled()
    state.hidden.mockReturnValue(false)
    document.dispatchEvent(new Event('visibilitychange'))
    expect(state.request).toHaveBeenCalledOnce()
    await wrapper.trigger('webglcontextlost')
    state.request.mockClear()
    await wrapper.trigger('webglcontextrestored')
    expect(renderer.dispose).toHaveBeenCalledOnce()
    expect(state.request).toHaveBeenCalledOnce()
    wrapper.unmount()
  })

  it('aborts loading on unmount instead of starting a detached animation', async () => {
    setup()
    let complete!: (value: { ok: boolean; arrayBuffer(): Promise<ArrayBuffer> }) => void
    vi.mocked(fetch).mockImplementation(() => new Promise(resolve => { complete = resolve }) as Promise<Response>)
    const wrapper = mount(DalaConstellation)
    const signal = vi.mocked(fetch).mock.calls[0][1]?.signal
    wrapper.unmount()
    expect(signal?.aborted).toBe(true)
    complete({ ok: true, arrayBuffer: async () => new ArrayBuffer(0) })
    await flushPromises()
    expect(renderer.draw).not.toHaveBeenCalled()
    expect(requestAnimationFrame).not.toHaveBeenCalled()
  })
})
