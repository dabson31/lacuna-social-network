'use client'

import { useState } from 'react'
import { describeError, type ErrorFace } from '@/lib/api/describe'
import { Empty } from './Empty'

/**
 * the one error renderer in the application.
 * ! every page that can fail renders this and nothing else, which is the point: a page never
 * ! writes its own `if (status === 403)`, so a change to how 404s read is one edit in
 * ! describeError instead of one edit per feature. describeError decides WHICH of the three
 * ! shapes applies, this file only draws them
 *
 * the three shapes are all already in the stylesheet:
 *   .notice  the red bar, for a failure that leaves the page behind it valid
 *   .empty   the label block, for nothing to show
 *   .q       the monospace readout, for the per field list
 */
export function ErrorView({
  error,
  onRetry,
  /** when the surrounding element is already a panel or a column, skip the outer padding */
  inline = false,
}: {
  error: unknown
  onRetry?: () => void
  inline?: boolean
}) {
  const face = describeError(error)

  switch (face.kind) {
    case 'empty':
      return <Empty label={face.detail || face.title} />

    case 'auth':
      //? the redirect is already in flight from the session provider's 401 hook, so this is
      //? the half second of copy before the route changes, not a dead end
      return <Empty label={`${face.title} · ${face.detail}`} />

    case 'fields':
      return <FieldList face={face} inline={inline} />

    case 'notice':
      return <ErrorBar face={face} onRetry={onRetry} />
  }
}

/**
 * the degraded segment notice, with the fiction swapped for a real failure.
 * ? the design built this with a strong title, a sub line, a detail disclosure and a dismiss,
 * ? which is exactly an api error banner. repurposing it is faithful and also honest, since
 * ? "segment 04 unreachable" was always standing in for a failure this app can really have
 */
function ErrorBar({ face, onRetry }: { face: ErrorFace; onRetry?: () => void }) {
  const [open, setOpen] = useState(false)
  const [gone, setGone] = useState(false)
  if (gone) return null

  return (
    <>
      <div className="notice">
        <strong>{face.title}</strong>
        <span>{face.detail}</span>
        <div className="spacer" />
        {face.retryable && onRetry && (
          <button className="btn btn--ghost" onClick={onRetry}>
            retry
          </button>
        )}
        <button
          className="btn btn--ghost"
          aria-expanded={open}
          aria-controls="errDetail"
          onClick={() => setOpen((v) => !v)}
        >
          detail
        </button>
        <button className="btn btn--ghost" onClick={() => setGone(true)}>
          dismiss
        </button>
      </div>
      <div className="notice__more" id="errDetail" hidden={!open}>
        <b>code</b>
        {' ' + face.code}
        {face.status > 0 && (
          <>
            <br />
            <b>status</b>
            {' ' + face.status}
          </>
        )}
        {face.fields &&
          Object.entries(face.fields).map(([k, v]) => (
            <span key={k}>
              <br />
              <b>{k}</b>
              {' ' + v}
            </span>
          ))}
      </div>
    </>
  )
}

/**
 * the server named which inputs are wrong, but this is not a form.
 * ? a form renders error.fields under each input itself, with useFieldErrors below. this is
 * ? the fallback for when a field error reaches something with no inputs to put it under
 */
function FieldList({ face, inline }: { face: ErrorFace; inline: boolean }) {
  const rows = Object.entries(face.fields ?? {})
  return (
    <div style={inline ? undefined : { padding: '18px 26px' }}>
      <p className="note-txt">{face.title}</p>
      {rows.length > 0 && (
        <div className="q" style={{ marginTop: 11, fontSize: '10.5px' }}>
          {rows.map(([k, v]) => (
            <span key={k}>
              &gt; {k} {v}
              <br />
            </span>
          ))}
        </div>
      )}
    </div>
  )
}

/**
 * the form half of the same model.
 * ! a form does not render ErrorView for a 400, it prints each message under the input the
 * ! server named. that is the only reason httpx.ValidationErrors carries field keys at all,
 * ! and it is why register needs this rather than a banner
 */
export function FieldError({ fields, name }: { fields: Record<string, string> | undefined; name: string }) {
  const msg = fields?.[name]
  if (!msg) return null
  return (
    <span className="field-error" role="alert">
      {msg}
    </span>
  )
}

/**
 * a form level error, for the cases with no field to attach to: a 401 on login, a 403, a 5xx.
 * ? login's invalid_credentials deliberately carries no fields, because saying which half was
 * ? wrong confirms which emails are registered
 */
export function FormError({ error }: { error: unknown }) {
  if (!error) return null
  const face = describeError(error)
  if (face.kind === 'fields' && face.fields) return null
  return (
    <div className="notice" style={{ marginBottom: 14 }}>
      <strong>{face.title}</strong>
      <span>{face.detail}</span>
    </div>
  )
}
