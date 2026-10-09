import Link from 'next/link'

/**
 * a route that matched nothing at all, which is outside both route groups and therefore
 * outside the shell.
 * ? routes under (main) have their own not-found that keeps the navigation around it. this one
 * ? is the bare case, so it has to offer its own way out
 */
export default function NotFound() {
  return (
    <div className="shell">
      <main className="main" style={{ display: 'grid', placeItems: 'center', minHeight: '100dvh' }}>
        <div className="panel" style={{ padding: 26, maxWidth: 420, textAlign: 'center' }}>
          <h1 style={{ fontSize: 15, marginBottom: 8 }}>no such route</h1>
          <p className="note-txt">This address does not resolve to anything on this node.</p>
          <Link className="btn btn--sm" href="/feed" style={{ marginTop: 16, display: 'inline-flex' }}>
            back to the feed
          </Link>
        </div>
      </main>
    </div>
  )
}
