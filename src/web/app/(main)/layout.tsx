'use client'

import { SessionProvider, useSession } from '@/lib/session/SessionProvider'
import { SocketProvider } from '@/lib/realtime/SocketProvider'
import { ToastProvider } from '@/components/ui/Toast'
import { BadgeProvider } from '@/components/layout/BadgeProvider'
import { Chrome } from '@/components/layout/Chrome'
import { RailLeft } from '@/components/layout/RailLeft'
import { RailRight } from '@/components/layout/RailRight'
import { StatusBar } from '@/components/layout/StatusBar'
import { MainHead } from '@/components/layout/MainHead'
import { Loading } from '@/components/ui/Empty'

/**
 * the shell, once, around every authenticated route.
 * ! the chrome's logout control and the right rail's Pending panel both live here and nowhere
 * ! else. the subject wants logout "available at all times" and notifications visible "in
 * ! every page", and a layout is the only way to make either structural rather than something
 * ! you remember to add to each new route
 *
 * ? four providers and that is the whole list: session, socket, badges, toast. the arch doc
 * ? ruled out a global store and nothing since has argued otherwise
 */
export default function MainLayout({ children }: { children: React.ReactNode }) {
  return (
    <SessionProvider>
      <ToastProvider>
        <SocketProvider>
          <BadgeProvider>
            <Shell>{children}</Shell>
          </BadgeProvider>
        </SocketProvider>
      </ToastProvider>
    </SessionProvider>
  )
}

function Shell({ children }: { children: React.ReactNode }) {
  const { state } = useSession()

  return (
    <div className="shell">
      <Chrome />
      <div className="body">
        <RailLeft />
        <main className="main">
          <MainHead />
          {/*
            ! three session states, and loading is its own branch. collapsing it into anonymous
            ! is the bug where a reader with a perfectly good cookie watches the page flash a
            ! redirect on every hard refresh, because `me` has not answered yet
          */}
          {state.status === 'loading' ? <Loading label="reading session…" /> : children}
        </main>
        <RailRight />
      </div>
      <StatusBar />
    </div>
  )
}
