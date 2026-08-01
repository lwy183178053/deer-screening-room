let csrfToken = ''

export function setCSRF(value: string) { csrfToken = value }

export class APIError extends Error {
  constructor(message: string, readonly status: number, readonly code: string, readonly captchaRequired: boolean) {
    super(message)
  }
}

export async function api<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers)
  if (options.body && !(options.body instanceof FormData)) headers.set('Content-Type', 'application/json')
  if (options.method && options.method !== 'GET' && options.method !== 'HEAD' && csrfToken) headers.set('X-CSRF-Token', csrfToken)
  const response = await fetch(path, { ...options, headers, credentials: 'same-origin' })
  if (!response.ok) {
    const body = await response.json().catch(() => null)
    throw new APIError(body?.error?.message || `请求失败 (${response.status})`, response.status, body?.error?.code || '', body?.error?.captcha_required === true)
  }
  if (response.status === 204) return undefined as T
  return response.json() as Promise<T>
}

export function csrf() { return csrfToken }
