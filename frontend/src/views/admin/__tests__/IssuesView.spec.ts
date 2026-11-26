import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter, RouterView } from 'vue-router'
import type { DesktopIssue, DesktopIssueAttachment, DesktopIssueDetail, DesktopIssueListResponse } from '@/api/admin/issues'
import IssuesView from '../IssuesView.vue'
import IssueDetail from '../desktop-issues/IssueDetail.vue'

const { get, patch, post, showError } = vi.hoisted(() => ({
  get: vi.fn(), patch: vi.fn(), post: vi.fn(), showError: vi.fn()
}))

vi.mock('@/api/client', () => ({ apiClient: { get, patch, post } }))
vi.mock('@/api/admin', async () => ({ issuesAPI: (await import('@/api/admin/issues')).default }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess: vi.fn() }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/i18n', () => ({ getLocale: () => 'en', i18n: { global: { t: (key: string) => key } } }))

interface RequestConfig {
  signal?: AbortSignal
  params?: Record<string, unknown>
}

interface PendingRequest {
  method: string
  path: string
  config?: RequestConfig
  payload?: unknown
  resolve: (data: unknown) => void
  reject: (error: Error) => void
}

const listPath = '/admin/issues'
const issuesPath = (id: number) => `${listPath}/${id}`
const videoPath = (id: number) => `${issuesPath(id)}/attachments/2`
const manifestPath = (id: number) => `${issuesPath(id)}/attachments/3/manifest`
const now = '2026-01-01T00:00:00Z'
const requests: PendingRequest[] = []
let wrapper: VueWrapper | undefined
const createObjectURL = vi.fn()
const revokeObjectURL = vi.fn()

function issue(id: number, title = `Issue ${id}`): DesktopIssue {
  return {
    id, title, description: `Report ${id}`, reporter_email: `reporter${id}@example.test`, reporter_username: '',
    kind: 'bug', severity: 'major', status: 'open', labels: [], assignee_name: `Owner ${id}`, fix_version: `v${id}`,
    app_version: '', client_os: '', client_arch: '', client_route: '', client_locale: '', client_meta: {},
    has_diagnostics: false, image_count: 0, video_count: 0, comment_count: 0, created_at: now, updated_at: now
  }
}

function listing(...items: DesktopIssue[]): DesktopIssueListResponse {
  return {
    items, total: items.length, page: 1, page_size: 20, pages: 1,
    counts: { all: items.length, open: items.length, in_progress: 0, resolved: 0, closed: 0 }
  }
}

function attachment(issueId: number, id: number, category: DesktopIssueAttachment['category'], fileName: string): DesktopIssueAttachment {
  return { id, issue_id: issueId, category, file_name: fileName, mime_type: '', size_bytes: 1024, sha256: '0123456789abcdef', created_at: now }
}

function detail(data: DesktopIssue, attachments: DesktopIssueAttachment[] = []): DesktopIssueDetail {
  return { issue: data, attachments, events: [], reporter_issue_count: 1 }
}

function capture(method: string, path: string, config?: RequestConfig, payload?: unknown) {
  // Deliberately let a canceled request still settle to exercise ownership, not just Axios cancellation.
  return new Promise((resolve, reject) => {
    requests.push({ method, path, config, payload, resolve: (data) => resolve({ data }), reject })
  })
}

function lastRequest(path: string, method = 'GET') {
  const matching = requests.filter((request) => request.path === path && request.method === method)
  expect(matching.length, `${method} ${path}`).toBeGreaterThan(0)
  return matching[matching.length - 1]
}

async function respond(request: PendingRequest, data: unknown) {
  request.resolve(data)
  await flushPromises()
}

async function open(path = listPath) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      {
        path: listPath, name: 'AdminIssues', component: IssuesView,
        children: [{
          path: ':id([1-9]\\d*)', name: 'AdminIssueDetail', component: IssueDetail,
          props: route => ({ issueId: Number(route.params.id) })
        }]
      },
      { path: '/admin/users', component: { template: '<p>Users</p>' } }
    ]
  })
  await router.push(path)
  wrapper = mount(RouterView, { global: { plugins: [router], stubs: { teleport: true } } })
  return { view: wrapper, router }
}

function button(view: VueWrapper, label: string) {
  const match = view.findAll('button').find((element) => element.text().trim() === label)
  if (!match) throw new Error(`Button not found: ${label}`)
  return match
}

function previewButton(view: VueWrapper, fileName: string) {
  const match = view.findAll('button').find((element) => element.text().includes(fileName))
  if (!match) throw new Error(`Preview not found: ${fileName}`)
  return match
}

const note = 'textarea[placeholder="admin.issues.detail.notePlaceholder"]'
const assignee = 'input[placeholder="admin.issues.detail.assigneePlaceholder"]'
const fixVersion = 'input[placeholder="admin.issues.detail.fixVersionPlaceholder"]'

beforeEach(() => {
  requests.length = 0
  vi.clearAllMocks()
  get.mockImplementation((path, config) => capture('GET', path, config))
  patch.mockImplementation((path, payload, config) => capture('PATCH', path, config, payload))
  post.mockImplementation((path, payload, config) => capture('POST', path, config, payload))
  createObjectURL.mockImplementation(() => `blob:preview-${createObjectURL.mock.calls.length}`)
  vi.stubGlobal('URL', class extends URL {
    static createObjectURL = createObjectURL
    static revokeObjectURL = revokeObjectURL
  })
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = undefined
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

describe('client feedback list', () => {
  it('keeps the newest filter result when the previous response comes back last', async () => {
    const { view } = await open()
    const previous = lastRequest(listPath)
    await button(view, 'admin.issues.filters.diagnosticsOnly').trigger('click')
    const latest = lastRequest(listPath)
    expect(latest.config?.params?.has_diagnostics).toBe(true)

    await respond(latest, listing(issue(43, 'Matches current filter')))
    await respond(previous, listing(issue(42, 'Old unfiltered result')))

    expect(view.text()).toContain('Matches current filter')
    expect(view.text()).not.toContain('Old unfiltered result')
    expect(previous.config?.signal?.aborted).toBe(true)
  })

  it('invalidates the old list as soon as search input changes, before the debounce fires', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    const { view } = await open()
    const previous = lastRequest(listPath)
    await view.get('input[placeholder="admin.issues.searchPlaceholder"]').setValue('#43')

    await respond(previous, listing(issue(42, 'Outdated search result')))
    expect(view.text()).not.toContain('Outdated search result')
    expect(view.get('button[title="common.refresh"]').attributes()).toHaveProperty('disabled')
    expect(requests).toHaveLength(1)

    await vi.advanceTimersByTimeAsync(240)
    const latest = lastRequest(listPath)
    expect(latest.config?.params?.search).toBe('#43')
    await respond(latest, listing(issue(43)))
    expect(view.text()).toContain('Issue 43')
  })

  it('ignores outdated failures and keeps loading until the current request settles', async () => {
    const { view } = await open()
    const previous = lastRequest(listPath)
    await button(view, 'admin.issues.filters.diagnosticsOnly').trigger('click')
    const latest = lastRequest(listPath)
    previous.reject(new Error('Old list failure'))
    await flushPromises()

    expect(showError).not.toHaveBeenCalled()
    expect(view.text()).not.toContain('Old list failure')
    expect(view.get('button[title="common.refresh"]').attributes()).toHaveProperty('disabled')

    latest.reject(new Error('Current list failure'))
    await flushPromises()
    expect(view.get('[role="alert"]').text()).toContain('Current list failure')
    expect(view.get('button[title="common.refresh"]').attributes()).not.toHaveProperty('disabled')
  })

  it('cancels the pending search debounce when leaving the page', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    const { view, router } = await open()
    const previous = lastRequest(listPath)
    await view.get('input[placeholder="admin.issues.searchPlaceholder"]').setValue('later')
    await router.push('/admin/users')
    await vi.advanceTimersByTimeAsync(240)
    expect(requests).toHaveLength(1)
    expect(previous.config?.signal?.aborted).toBe(true)
  })
})

describe('client feedback detail', () => {
  it('keeps next/previous available while only the current detail GET can render', async () => {
    const { view, router } = await open()
    await respond(lastRequest(listPath), listing(issue(42), issue(43), issue(44)))
    await router.push(issuesPath(42))
    const old = lastRequest(issuesPath(42))

    const next = () => view.get('[title="admin.issues.actions.next"]').trigger('click')
    await next()
    await flushPromises()
    const middle = lastRequest(issuesPath(43))
    await next()
    await flushPromises()
    expect(router.currentRoute.value.path).toBe(issuesPath(44))
    await respond(lastRequest(issuesPath(44)), detail(issue(44)))
    await respond(old, detail(issue(42, 'Wrong detail')))
    middle.reject(new Error('Stale detail failure'))
    await flushPromises()

    expect(view.get('h1').text()).toBe('Issue 44')
    expect(showError).not.toHaveBeenCalled()
    expect(old.config?.signal?.aborted).toBe(true)
    expect(middle.config?.signal?.aborted).toBe(true)
    expect(requests.filter((request) => request.path === listPath)).toHaveLength(1)
    await view.get('[title="admin.issues.actions.prev"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe(issuesPath(43))
  })

  it('does not carry drafts to another issue or let a late note replace it', async () => {
    const { view, router } = await open(issuesPath(42))
    await respond(lastRequest(issuesPath(42)), detail(issue(42)))
    await view.get(note).setValue('First issue note')
    await view.get(assignee).setValue('First issue unsaved owner')
    await view.get(fixVersion).setValue('first-fix')
    await button(view, 'admin.issues.actions.addNote').trigger('click')
    const save = lastRequest(`${issuesPath(42)}/notes`, 'POST')
    expect(save.payload).toEqual(expect.objectContaining({ body: 'First issue note', fix_version: 'first-fix' }))

    await router.push(issuesPath(43))
    await respond(lastRequest(issuesPath(43)), detail(issue(43)))
    expect(view.get<HTMLTextAreaElement>(note).element.value).toBe('')
    expect(view.get<HTMLInputElement>(assignee).element.value).toBe('Owner 43')
    expect(view.get<HTMLInputElement>(fixVersion).element.value).toBe('v43')
    await view.get(note).setValue('Second issue note')
    await respond(save, detail(issue(42, 'Wrong save response')))

    expect(save.config?.signal?.aborted).toBe(true)
    expect(view.get('h1').text()).toBe('Issue 43')
    expect(view.get<HTMLTextAreaElement>(note).element.value).toBe('Second issue note')
    await button(view, 'admin.issues.actions.addNote').trigger('click')
    expect(lastRequest(`${issuesPath(43)}/notes`, 'POST').payload).toEqual(expect.objectContaining({ body: 'Second issue note' }))
  })

  it('ignores an old PATCH even after returning to the same issue ID', async () => {
    const { view, router } = await open(issuesPath(42))
    await respond(lastRequest(issuesPath(42)), detail(issue(42)))
    await button(view, 'admin.issues.status.in_progress').trigger('click')
    const old = lastRequest(issuesPath(42), 'PATCH')

    await router.push(listPath)
    await router.push(issuesPath(42))
    await respond(lastRequest(issuesPath(42)), detail(issue(42, 'Current visit')))
    await view.get(assignee).setValue('Current visit owner')
    await respond(old, detail(issue(42, 'Previous visit')))

    expect(view.get('h1').text()).toBe('Current visit')
    expect(view.get<HTMLInputElement>(assignee).element.value).toBe('Current visit owner')
    expect(old.config?.signal?.aborted).toBe(true)
  })

  it('cancels and releases old previews when leaving and reopening the same issue', async () => {
    const { view, router } = await open(issuesPath(42))
    const image = attachment(42, 1, 'image', 'screen.png')
    const video = attachment(42, 2, 'video', 'recording.webm')
    await respond(lastRequest(issuesPath(42)), detail(issue(42), [image, video]))
    await respond(lastRequest(`${issuesPath(42)}/attachments/1`), new Blob(['image']))
    expect(view.get('img').attributes('src')).toBe('blob:preview-1')
    await previewButton(view, 'recording.webm').trigger('click')
    const oldVideo = lastRequest(videoPath(42))

    await router.push(issuesPath(43))
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:preview-1')
    expect(view.find('video').exists()).toBe(false)
    await router.push(issuesPath(42))
    await respond(lastRequest(issuesPath(42)), detail(issue(42), [video]))
    await previewButton(view, 'recording.webm').trigger('click')
    const newVideo = lastRequest(videoPath(42))
    await respond(oldVideo, new Blob(['wrong video']))

    expect(view.find('video').exists()).toBe(false)
    expect(createObjectURL).toHaveBeenCalledTimes(1)
    expect(oldVideo.config?.signal?.aborted).toBe(true)
    await respond(newVideo, new Blob(['current video']))
    expect(view.get('video').attributes('src')).toBe('blob:preview-2')
    await router.push('/admin/users')
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:preview-2')
  })

  it('keeps diagnostics responses in the visit that requested them', async () => {
    const { view, router } = await open(issuesPath(42))
    const diagnostics = attachment(42, 3, 'diagnostics', 'diagnostics.zip')
    const openManifest = () => button(view, 'admin.issues.actions.inspectManifest').trigger('click')
    await respond(lastRequest(issuesPath(42)), detail(issue(42), [diagnostics]))
    await openManifest()
    const old = lastRequest(manifestPath(42))
    await router.push(issuesPath(43))
    await respond(lastRequest(issuesPath(43)), detail(issue(43), [attachment(43, 3, 'diagnostics', 'other.zip')]))
    await openManifest()
    const latest = lastRequest(manifestPath(43))
    await respond(latest, { entries: [{ name: 'current.log', size_bytes: 1 }] })
    await respond(old, { entries: [{ name: 'wrong.log', size_bytes: 1 }] })

    expect(view.text()).toContain('current.log')
    expect(view.text()).not.toContain('wrong.log')
    expect(old.config?.signal?.aborted).toBe(true)
  })
})
