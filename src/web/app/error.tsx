'use client'

import Link from 'next/link'
import { describeError } from '@/lib/api/describe'

/**
 * the root error boundary, for a crash that escaped a page.
 * ! it goes through the same describeError as every other failure, so a render crash reads the
 * ! same way an api failure does and there is still only one error taxonomy in the app
 */
export default function RootError({ error, reset }: { error: Error; reset: () => void }) {
  const face = describeError(error)

  return (
    <div className="shell">
      <main className="main" style={{ display: 'grid', placeItems: 'center', minHeight: '100dvh' }}>
        <div className="panel" style={{ padding: 26, maxWidth: 460 }}>
          <h1 style={{ fontSize: 15, marginBottom: 8 }}>{face.title}</h1>
          <p className="note-txt">{face.detail}</p>
          <div className="q" style={{ margin: '11px 0 0', fontSize: '10.5px' }}>
            &gt; code {face.code}
          </div>
          <div className="row-ops" style={{ marginTop: 16 }}>
            <button className="btn btn--sm" onClick={reset}>
              try again
            </button>
            <Link className="btn btn--sm" href="/feed">
              back to the feed
            </Link>
          </div>
        </div>
      </main>
    </div>
  )
}
