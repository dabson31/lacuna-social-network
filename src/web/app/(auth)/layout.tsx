import type { Metadata } from 'next'

export const metadata: Metadata = {
  title: 'LACUNA — Access',
}

/**
 * a bare centred card, no chrome and no rails.
 * ! this is what the route group is for: the two halves of the app genuinely want different
 * ! layouts, and putting login inside the shell would mean a navigation bar for somebody with
 * ! nothing to navigate to
 */
export default function AuthLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="shell">
      <main
        className="main"
        style={{ display: 'grid', placeItems: 'center', minHeight: '100dvh', padding: '26px 16px' }}
      >
        {children}
      </main>
    </div>
  )
}
