import { ApiError, isApiError } from './errors'

/**
 * the one place that decides what an error looks like on screen.
 * ! this exists so no page ever writes an `if (status === 403)` of its own. a page calls
 * ! describeError and renders whatever comes back, which means a change to how 404s read is
 * ! one edit here instead of one edit per feature. the taxonomy is the api's, straight off the
 * ! status table in plan/frontend/route-map.md
 *
 * ? it returns data and never jsx on purpose, so the mapping is testable and the rendering
 * ? stays in components/ui/ErrorView. same split as handlers and services on the go side
 */

/**
 * where an error belongs in the layout, which is a design question before it is a technical one
 *
 *   notice — the red .notice bar between the chrome and the body. the request failed but the
 *            page behind it is still valid, so it reports in place and does not replace anything
 *   empty  — the .empty block inside the main column. there is nothing to show, and the shell
 *            with its navigation stays exactly where it is
 *   fields — the server named the inputs that are wrong. a form renders these under each one,
 *            and anything that is not a form falls back to listing them in a notice
 *   auth   — the session is gone. the redirect is already in flight, this is the half second
 *            of copy before the route changes
 */
export type ErrorFaceKind = 'notice' | 'empty' | 'fields' | 'auth'

export interface ErrorFace {
  kind: ErrorFaceKind
  /** short, lowercase, in the design's voice. goes in the <strong> of the notice or the .lbl */
  title: string
  /** the server's own message where there is one, since it is written to be shown */
  detail: string
  /** the machine readable code, printed in the detail disclosure. stable, unlike the message */
  code: string
  status: number
  /** whether offering a retry makes sense. a 403 will not change if you ask again */
  retryable: boolean
  fields?: Record<string, string>
}

export function describeError(e: unknown): ErrorFace {
  if (!isApiError(e)) {
    // a render crash or a programming bug, which is the one case with no server message
    return {
      kind: 'notice',
      title: 'something broke on this page',
      detail: e instanceof Error ? e.message : 'an unexpected error occurred',
      code: 'client_error',
      status: 0,
      retryable: true,
    }
  }

  const base = { code: e.code, status: e.status, detail: e.message, fields: e.fields }

  // the network never answered, which NetworkError reports as status 0
  if (e.status === 0) {
    return {
      ...base,
      kind: 'notice',
      title: 'server unreachable',
      detail: 'no answer from the api. it may be down, or the connection dropped.',
      retryable: true,
    }
  }

  switch (e.status) {
    // malformed input. with fields it is a form's problem, without them it is a bad request
    // the user cannot fix, so it reports as a notice
    case 400:
      return { ...base, kind: e.fields ? 'fields' : 'notice', title: 'that request was not valid', retryable: false }

    //! two very different 401s, and the code is what tells them apart. "unauthenticated" means
    //! the cookie is gone and every page on screen is stale. "invalid_credentials" is the login
    //! form being answered — nothing ended, the password was simply wrong. mapping both to
    //! "your session ended" tells somebody who just mistyped that they were logged out of a
    //! session they never had
    case 401:
      if (e.code === 'invalid_credentials') {
        return { ...base, kind: 'notice', title: 'that did not match', retryable: false }
      }
      return { ...base, kind: 'auth', title: 'your session ended', detail: 'log in again to continue.', retryable: false }

    //! 403 is the only status this api uses to say "you are logged in and still may not". it
    //! means the thing exists and is not yours, so it reports in place rather than replacing
    //! the page, and retrying is pointless
    case 403:
      return { ...base, kind: 'notice', title: 'not permitted', retryable: false }

    //! 404 means not found OR not visible to you, deliberately, so the api never confirms that
    //! a private thing exists. the copy must never say "deleted" — that is a guess, and it is
    //! wrong every time a stranger opens an almost_private post
    case 404:
      return { ...base, kind: 'empty', title: 'not available', detail: e.message || 'this is not here, or not yours to see.', retryable: false }

    case 409:
      return { ...base, kind: e.fields ? 'fields' : 'notice', title: 'that already exists', retryable: false }

    case 413:
      return { ...base, kind: 'fields', title: 'that file is too large', fields: e.fields ?? { image: 'file is too large' }, retryable: false }

    case 415:
      return { ...base, kind: 'fields', title: 'that file type is not allowed', fields: e.fields ?? { image: 'must be a JPEG, PNG or GIF' }, retryable: false }

    default:
      // 5xx and anything unforeseen. the server already logged the real cause, so the client
      // says the honest minimum and offers to try again
      return {
        ...base,
        kind: 'notice',
        title: e.status >= 500 ? 'the server failed on that' : 'that did not work',
        retryable: e.status >= 500,
      }
  }
}

/** a 404 built by hand, for a route that resolved but whose subject does not exist */
export function notFoundFace(what: string): ErrorFace {
  return describeError(new ApiError(404, { code: 'not_found', message: `this ${what} is not here, or not yours to see.` }))
}
