/**
 * the one error shape this api ever returns, and the one parser that reads it.
 * ! httpx.go says it outright: ErrorBody is the ONLY error shape the api will ever return, so
 * ! the frontend writes one error parser against this and never needs another. that guarantee
 * ! is worth using properly instead of sprinkling try/catch with string matching
 */

/** mirrors httpx.ErrorBody */
export interface ErrorBody {
  code: string
  message: string
  fields?: Record<string, string>
}

export class ApiError extends Error {
  readonly status: number
  readonly code: string
  /**
   * ! fields is the whole reason this carries more than a message: the register form needs
   * ! "already registered" under the email input and not in a toast. WriteValidation fills it
   * ! server side with one key per form field, and this is where it survives the trip
   */
  readonly fields?: Record<string, string>

  constructor(status: number, body: ErrorBody) {
    super(body.message || 'request failed')
    this.name = 'ApiError'
    this.status = status
    this.code = body.code || 'unknown'
    this.fields = body.fields
  }
}

/**
 * the network never answered at all, which is a different thing from the server answering
 * with an error. a 0 status keeps it inside the one error type so no caller has to handle
 * two shapes
 */
export class NetworkError extends ApiError {
  constructor(cause?: unknown) {
    super(0, {
      code: 'unreachable',
      message: 'the server did not answer, it may be down or the connection dropped',
    })
    this.name = 'NetworkError'
    if (cause instanceof Error) this.cause = cause
  }
}

export function isApiError(e: unknown): e is ApiError {
  return e instanceof ApiError
}

/**
 * ? a field error worth printing under an input, as opposed to one that belongs in a banner.
 * ? 400 validation, 409 conflicts and 415 media types all arrive with fields filled in
 */
export function fieldErrors(e: unknown): Record<string, string> | undefined {
  return isApiError(e) ? e.fields : undefined
}
