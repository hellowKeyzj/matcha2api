import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import PackagesPage from './PackagesPage.vue'

const { getCloudKey, generateCloudKey, showError, showSuccess } = vi.hoisted(() => ({
  getCloudKey: vi.fn(), generateCloudKey: vi.fn(), showError: vi.fn(), showSuccess: vi.fn()
}))
vi.mock('@/api/admin', () => ({ adminAPI: { packages: {
  list: vi.fn().mockResolvedValue([]), getStorage: vi.fn().mockResolvedValue({}), getCloudKey, generateCloudKey
} } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess }) }))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key, te: () => true })
}))
vi.mock('@/components/auth/TotpStepUpDialog.vue', () => ({ default: defineComponent({
  props: ['controller'],
  template: '<div v-if="controller.visible.value"><button @click="controller.onVerified()">verify</button><button @click="controller.onCancel()">cancel</button></div>'
}) }))

const emptyKey = { configured: false, key_id: '', algorithm: '', source: '', public_key: '' }
const configuredKey = { configured: true, key_id: 'cloud-key', algorithm: 'RSA-OAEP-256', source: 'database', public_key: 'public' }
function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => { resolve = done })
  return { promise, resolve }
}
function mountPage() {
  return mount(PackagesPage, { global: { stubs: {
    AppLayout: { template: '<main><slot /></main>' }, BaseDialog: true, Icon: true
  } } })
}
function button(wrapper: ReturnType<typeof mountPage>, label: string) {
  return wrapper.findAll('button').find((item) => item.text().includes(label))!
}

describe('package cloud key step-up', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getCloudKey.mockResolvedValue(emptyKey)
    generateCloudKey.mockResolvedValue(configuredKey)
  })

  it('serializes status loading and generation through the existing step-up dialog', async () => {
    const initial = deferred<typeof emptyKey>()
    getCloudKey.mockReturnValueOnce(initial.promise)
    const wrapper = mountPage()
    await flushPromises()
    expect(button(wrapper, 'admin.packages.cloudKey.generate').attributes('disabled')).toBeDefined()
    initial.resolve(emptyKey)
    await flushPromises()
    generateCloudKey.mockRejectedValueOnce({ code: 'STEP_UP_REQUIRED' })
    const generated = deferred<typeof configuredKey>()
    generateCloudKey.mockReturnValueOnce(generated.promise)
    await button(wrapper, 'admin.packages.cloudKey.generate').trigger('click')
    await flushPromises()
    expect(button(wrapper, 'admin.packages.cloudKey.generating').attributes('disabled')).toBeDefined()
    await button(wrapper, 'admin.packages.actions.refresh').trigger('click')
    await flushPromises()
    expect(getCloudKey).toHaveBeenCalledTimes(1)
    await button(wrapper, 'verify').trigger('click')
    await flushPromises()
    expect(generateCloudKey).toHaveBeenCalledTimes(2)
    generated.resolve(configuredKey)
    await flushPromises()
    expect(wrapper.text()).toContain('cloud-key')
    expect(button(wrapper, 'admin.packages.cloudKey.generate').attributes('disabled')).toBeDefined()
    expect(showError).not.toHaveBeenCalled()
    expect(showSuccess).toHaveBeenCalledWith('admin.packages.messages.cloudKeyGenerated')
    wrapper.unmount()
  })

  it('cancels step-up silently and localizes blocked generation', async () => {
    const wrapper = mountPage()
    await flushPromises()
    generateCloudKey.mockRejectedValueOnce({ code: 'STEP_UP_REQUIRED' })
    await button(wrapper, 'admin.packages.cloudKey.generate').trigger('click')
    await flushPromises()
    await button(wrapper, 'cancel').trigger('click')
    await flushPromises()
    expect(generateCloudKey).toHaveBeenCalledTimes(1)
    expect(showError).not.toHaveBeenCalled()
    generateCloudKey.mockRejectedValueOnce({ code: 'STEP_UP_ADMIN_API_KEY_FORBIDDEN' })
    await button(wrapper, 'admin.packages.cloudKey.generate').trigger('click')
    await flushPromises()
    expect(showError).toHaveBeenCalledWith('stepUp.adminApiKeyForbidden')
    wrapper.unmount()
  })
})
