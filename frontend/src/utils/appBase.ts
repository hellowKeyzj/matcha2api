// The backend sets <base> before any modules load. Vite development has no mount.
export const appBasePath = new URL(document.querySelector('base')?.href || '/', window.location.origin).pathname.replace(/\/$/, '')

export function appPath(path: string): string {
  if (!path.startsWith('/') || path.startsWith('//')) return path
  const pathname = path.split(/[?#]/, 1)[0]
  if (appBasePath && (pathname === appBasePath || pathname.startsWith(`${appBasePath}/`))) return path
  return `${appBasePath}${path}`
}

export function stripAppBase(path: string): string {
  return appBasePath && path.startsWith(`${appBasePath}/`) ? path.slice(appBasePath.length) : path
}
