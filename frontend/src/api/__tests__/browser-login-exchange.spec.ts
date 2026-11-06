import axios, { type AxiosAdapter, type AxiosInstance } from 'axios'
import { afterEach, beforeEach, describe, expect, it, vi, type MockInstance } from 'vitest'

vi.mock('@/i18n', () => ({ getLocale: () => 'en' }))

let apiClient: AxiosInstance
let exchange: typeof import('@/api/auth')['exchangeBrowserLoginTicket']
let refresh: MockInstance<typeof import('@/api/tokenRefresh')['refreshAuthTokens']>
const redirect = vi.fn()
const oldCredentials = {
  auth_token: 'old-access', refresh_token: 'old-refresh',
  auth_user: JSON.stringify({ id: 1 }), token_expires_at: '123',
}

beforeEach(async () => {
  vi.resetModules()
  localStorage.clear()
  sessionStorage.clear()
  redirect.mockClear()
  for (const [key, value] of Object.entries(oldCredentials)) localStorage.setItem(key, value)
  // Observe full-page redirects without allowing a failed assertion to navigate jsdom.
  const location = {
    pathname: '/auth/desktop',
    get href() { return 'http://localhost/auth/desktop' },
    set href(value: string) { redirect(value) },
  }
  vi.stubGlobal('window', new Proxy(window, {
    get(target, property, receiver) {
      return property === 'location' ? location : Reflect.get(target, property, receiver)
    },
  }))
  const tokenRefresh = await import('@/api/tokenRefresh')
  refresh = vi.spyOn(tokenRefresh, 'refreshAuthTokens').mockRejectedValue(
    Object.assign(new axios.AxiosError('unexpected refresh'), { response: { status: 401 } }),
  )
  apiClient = (await import('@/api/client')).apiClient
  exchange = (await import('@/api/auth')).exchangeBrowserLoginTicket
})

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

function expectOldSessionUntouched() {
  expect(refresh).not.toHaveBeenCalled()
  expect(redirect).not.toHaveBeenCalled()
  expect(sessionStorage.getItem('auth_expired')).toBeNull()
  for (const [key, value] of Object.entries(oldCredentials)) expect(localStorage.getItem(key)).toBe(value)
}

describe('browser login exchange with the real API client', () => {
  it('omits the existing bearer and forwards the abort signal without persisting the response', async () => {
    const controller = new AbortController()
    const pair = { access_token: 'new-access', refresh_token: 'new-refresh', user: { id: 2 } }
    const adapter = vi.fn<AxiosAdapter>(async (config) => ({
      data: { code: 0, data: pair }, status: 200, statusText: 'OK', headers: {}, config,
    }))
    apiClient.defaults.adapter = adapter
    await expect(exchange('A'.repeat(43), controller.signal)).resolves.toEqual(pair)
    expect(adapter).toHaveBeenCalledTimes(1)
    const config = adapter.mock.calls[0][0]
    expect(config.url).toBe('/auth/browser-login/exchange')
    expect(config.method).toBe('post')
    expect(JSON.parse(config.data)).toEqual({ ticket: 'A'.repeat(43) })
    expect(config.signal).toBe(controller.signal)
    expect(config.headers.toJSON()).not.toHaveProperty('Authorization')
    expectOldSessionUntouched()
  })

  it('does not refresh, retry, redirect, or erase old credentials for an invalid-ticket 401', async () => {
    const adapter = vi.fn<AxiosAdapter>(async (config) => {
      const response = {
        data: { code: 401, reason: 'INVALID_BROWSER_LOGIN_TICKET', message: 'Invalid ticket' },
        status: 401, statusText: 'Unauthorized', headers: {}, config,
      }
      throw new axios.AxiosError('Invalid ticket', 'ERR_BAD_REQUEST', config, {}, response)
    })
    apiClient.defaults.adapter = adapter
    await expect(exchange('A'.repeat(43))).rejects.toMatchObject({
      status: 401, reason: 'INVALID_BROWSER_LOGIN_TICKET',
    })
    expect(adapter).toHaveBeenCalledTimes(1)
    expect(adapter.mock.calls[0][0].headers.toJSON()).not.toHaveProperty('Authorization')
    expectOldSessionUntouched()
  })
})
