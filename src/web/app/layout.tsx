import type { Metadata } from 'next'
import '@/styles/lacuna.css'
//? the one stylesheet that is not the design, loaded after it. it covers the two things the
//? mockup never needed — real uploaded images, and a form outside the composer — and takes
//? every value from a rule or token lacuna.css already defines
import '@/styles/additions.css'

export const metadata: Metadata = {
  title: 'LACUNA — Node Terminal',
  description: 'A social layer that keeps its own copy of everything you can see.',
}

/**
 * ! dark only by intent. the stylesheet says so in its own comment — "a system light
 * ! preference can't break the composition" — so there is deliberately no
 * ! prefers-color-scheme branch anywhere
 */
export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" data-theme="dark">
      <head>
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link rel="preconnect" href="https://fonts.gstatic.com" crossOrigin="" />
        <link
          href="https://fonts.googleapis.com/css2?family=IBM+Plex+Mono:wght@300;400;500;600&family=Inter:wght@400;500;600&display=swap"
          rel="stylesheet"
        />
        {/*
          ! this has to be inline and it has to run before the body paints. moved into an
          ! effect it runs after first paint and a reader who picked umber sees a flash of
          ! graphite, which is the exact bug the mockup's own inline script exists to avoid
        */}
        <script
          dangerouslySetInnerHTML={{
            __html: `try{var s=localStorage.getItem('lacuna.scheme');if(s==='umber')document.documentElement.dataset.scheme='umber'}catch(e){}`,
          }}
        />
      </head>
      <body>
        {/* the analog layer. decorative, and outside .shell exactly as the mockup has it */}
        <div className="crt" aria-hidden="true" />
        <div className="grain" aria-hidden="true" />
        <div className="sweep" aria-hidden="true" />
        {children}
      </body>
    </html>
  )
}
