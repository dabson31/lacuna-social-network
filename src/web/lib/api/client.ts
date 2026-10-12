import { ApiError, NetworkError, type ErrorBody } from './errors'

/**
 * the only fetch in the application. every feature's api.ts goes through here, so the things
 * that must never be forgotten are impossible to forget
 */

export const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080'

/** an uploaded file is stored as a bare name and served from /uploads/{name} */
export function uploadUrl(name: string): string {
  return `${API_BASE}/uploads/${encodeURIComponent(name)}`
}

/**
 * ! the 401 hook. a 401 from any call means the cookie expired or the session row was
 * ! destroyed, at which point every page on screen is wrong. the session provider registers
 * ! itself here at mount so the redirect happens in one place instead of in forty callers
 */
type UnauthorizedHandler = () => void
let onUnauthorized: UnauthorizedHandler | null = null
export function setUnauthorizedHandler(fn: UnauthorizedHandler | null): void {
  onUnauthorized = fn
}

/** the two endpoints whose 401 is an answer rather than a dead session */
function isAuthProbe(path: string): boolean {
  return path === '/api/auth/login' || path === '/api/auth/me'
}

async function request<T>(path: string, init: RequestInit): Promise<T> {
  let res: Response
  try {
    res = await fetch(`${API_BASE}${path}`, {
      ...init,
      //! this single line is what the entire session depends on. the cookie is HttpOnly and
      //! scoped to :8080 while the page is on :3000, so without it the browser attaches
      //! nothing and every authenticated route answers 401 while the request looks perfectly
      //! correct in the network tab. it is set here, once, where no feature can drop it
      credentials: 'include',
    })
  } catch (cause) {
    // fetch only rejects when the request never completed, a dns failure, a refused
    // connection, an abort. an http error status resolves normally and is handled below
    throw new NetworkError(cause)
  }

  if (!res.ok) {
    const body = await readErrorBody(res)
    //! a 401 normally means the cookie died, so the handler clears the session and redirects.
    //! the two auth endpoints are the exception: login answering 401 is it doing its job, and
    //! `me` answering 401 on a cold load is just "nobody is logged in". firing the global
    //! handler for those turns an ordinary wrong password into a session-expired redirect
    if (res.status === 401 && onUnauthorized && !isAuthProbe(path)) onUnauthorized()
    throw new ApiError(res.status, body)
  }

  //! 204 is the normal success for roughly fifteen endpoints: logout, both patches, unfollow,
  //! every mark-read, rsvp and most group actions. json() on an empty body throws
  //! "Unexpected end of JSON input", which then surfaces as an error message on a request
  //! that actually worked
  if (res.status === 204 || res.headers.get('content-length') === '0') {
    return undefined as T
  }
  return (await res.json()) as T
}

/**
 * an error body should be the standard envelope, but a 502 from something in front of the
 * server is html, and a parse failure must not replace the real status with a json error
 */
async function readErrorBody(res: Response): Promise<ErrorBody> {
  try {
    const parsed: unknown = await res.json()
    if (parsed && typeof parsed === 'object' && 'error' in parsed) {
      const { error } = parsed as { error: ErrorBody }
      if (error && typeof error.code === 'string') return error
    }
  } catch {
    // fall through to the generic body below
  }
  return { code: `http_${res.status}`, message: res.statusText || 'request failed' }
}

export interface JsonOptions {
  method?: 'GET' | 'POST' | 'PATCH' | 'DELETE'
  body?: unknown
  signal?: AbortSignal
}

/** a json request. DecodeJSON runs DisallowUnknownFields, so send exactly the documented keys */
export function json<T>(path: string, opts: JsonOptions = {}): Promise<T> {
  const { method = 'GET', body, signal } = opts
  return request<T>(path, {
    method,
    signal,
    headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
}

/**
 * a multipart request, for anything carrying an image.
 * ! never set Content-Type here. the browser generates it with a boundary token as part of
 * ! the header, and a hand written "multipart/form-data" has no boundary, so
 * ! ParseMultipartForm fails and the handler answers 400 "expected a multipart/form-data
 * ! body", which is the most misleading error message in the whole api
 */
export function form<T>(path: string, body: FormData, method: 'POST' | 'PATCH' = 'POST'): Promise<T> {
  return request<T>(path, { method, body })
}

/** a query string builder that drops empty values rather than sending limit=&before= */
export function query(params: Record<string, string | number | undefined | null>): string {
  const qs = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) {
    if (v === undefined || v === null || v === '') continue
    qs.set(k, String(v))
  }
  const s = qs.toString()
  return s ? `?${s}` : ''
}
