'use client'

import { useEffect } from 'react'
// next/router is the pages-router hook and it throws "NextRouter was not mounted" inside the app router, which is a runtime failure rather
import { useRouter } from 'next/navigation'
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

  const router = useRouter()

  //! nobody un-authenticated stays inside main
  //! this retarded ass implemention fucks somethign and makes it infinitely load
  //! a browser holding a cookie whose session row is gone
  //! without this, it would try reading the session forever
  //
  //? the main issue was that it would render before logging you out
  useEffect(() => {
    if (state.status === 'anonymous') router.replace('/login')
  }, [state.status, router])

  return (
    <div className="shell">
      <Chrome />
      <div className="body">
        <RailLeft />
        <main className="main">
          <MainHead />

          {/*
            !
            ! {state.status === 'loading' ? <Loading label="reading session…" /> : children}
              because signOut() is synchronous and
          */}

          {/*
            ! children render onyl when there is a user, which is exactly what useUser()
            ? somebody with a good cookie must not vbe redirected   
          */}
          {state.status === 'authenticated' ? children : <Loading label={state.status === 'loading' ? 'reading session…' : 'closing session…'} />}
        </main>
        <RailRight />
      </div>
      <StatusBar />
    </div>
  )
}
