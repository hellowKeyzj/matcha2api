import { afterEach, describe, expect, it, vi } from 'vitest'

afterEach(() => {
  document.querySelector('base')?.remove()
  vi.unstubAllEnvs()
  vi.resetModules()
})

describe.each(['/', '/sub2api/'])('runtime mount %s', (base) => {
  it('uses the runtime base for assets, routes, APIs and gateway URLs', async () => {
    vi.resetModules()
    vi.stubEnv('VITE_API_BASE_URL', '')
    document.head.insertAdjacentHTML('afterbegin', `<base href="${base}">`)
    const { appPath, stripAppBase } = await import('@/utils/appBase')
    const { getAPIBaseURL, buildApiUrl, buildGatewayUrl } = await import('@/api/url')
    const { shouldMarkAdminUIRequest } = await import('@/api/adminUIRequest')
    expect(appPath('/login')).toBe(`${base}login`)
    expect(appPath(`${base}login`)).toBe(`${base}login`)
    expect(appPath(`${base}login?next=/purchase#form`)).toBe(`${base}login?next=/purchase#form`)
    expect(appPath('https://external.test/logo')).toBe('https://external.test/logo')
    expect(stripAppBase(`${base}admin/ops`)).toBe('/admin/ops')
    expect(getAPIBaseURL()).toBe(`${base}api/v1`)
    expect(buildApiUrl('/auth/refresh')).toBe(`${base}api/v1/auth/refresh`)
    expect(buildGatewayUrl('/v1/models')).toBe(`${location.origin}${base}v1/models`)
    expect(shouldMarkAdminUIRequest('/auth/me', `${base}admin/dashboard`)).toBe(true)
  })
})

it('does not duplicate the mount for its exact path with query or fragment', async () => {
  document.head.insertAdjacentHTML('afterbegin', '<base href="/sub2api/">')
  const { appPath } = await import('@/utils/appBase')
  expect(appPath('/sub2api?from=login')).toBe('/sub2api?from=login')
  expect(appPath('/sub2api#top')).toBe('/sub2api#top')
  expect(appPath('/sub2apix/login')).toBe('/sub2api/sub2apix/login')
})
