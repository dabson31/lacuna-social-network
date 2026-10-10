'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'
import { useRouter } from 'next/navigation'
import * as api from '@/lib/api/client'
import { useSession } from '@/lib/session/SessionProvider'
import { useSocket } from '@/lib/realtime/SocketProvider'
import { useToast } from '@/components/ui/Toast'
import { handle } from '@/lib/utils/identity'

/**
 * ! logout lives here and nowhere else. the subject wants it "available at all times", and a
 * ! layout is the only way to make that structural instead of a thing you remember to add to
 * ! each new route. same reasoning as the notification badges
 *
 * ? node, layer, peers and the segment readouts stay as the mockup wrote them. this is a
 * ? single process over one sqlite file, there are no segments, and the design's own header
 * ? says every live readout here is synthetic
 */
export function Chrome() {
  const { state, signOut } = useSession()
  const { status, rtt } = useSocket()
  const router = useRouter()
  const toast = useToast()
  const [clock, setClock] = useState('--:--:--')

  //? the clock is the one genuinely live readout in the mockups chrome, so it stays live
  //? it starts as the placeholder so the server render and the first client render agree
  useEffect(() => {
    const tick = () => {
      const d = new Date()
      setClock([d.getUTCHours(), d.getUTCMinutes(), d.getUTCSeconds()].map((n) => String(n).padStart(2, '0')).join(':'))
    }
    tick()
    const id = setInterval(tick, 1000)
    return () => clearInterval(id)
  }, [])

  const user = state.status === 'authenticated' ? state.user : null

  const onLogout = async () => {
    try {
      //! not just a client side forget. the handler deletes the sessions row and then clears
      //! the cookie, which is what makes two browsers independent: closing one session must
      //! not touch the other, and both must survive a restart
      await api.json('/api/auth/logout', { method: 'POST' })
    } catch {
      // the cookie is being dropped either way, a failed logout must not trap anyone here
    }
    signOut()
    toast('session closed')
    router.replace('/login')
  }

  const sockLabel = status === 'open' ? 'open' : status === 'reconnecting' ? 'retry' : status === 'connecting' ? 'opening' : 'closed'

  return (
    <header className="chrome">
      <div className="mark">
        <i />
        LACUNA
      </div>
      <div className="sep" />
      <span>
        node <b>LCN·7F3A</b>
      </span>
      <span className="hide-sm">
        layer <b>02</b>
      </span>
      <div className="sep hide-sm" />
      <span className="hide-sm">
        peers <b>148</b>
      </span>
      <div className="spacer" />
      <span className="hide-sm">
        socket <b style={{ color: status === 'open' ? 'var(--acid)' : 'var(--red)' }}>{sockLabel}</b> · <b>{rtt === null ? '--' : `${rtt}ms`}</b>
      </span>
      <div className="sep hide-sm" />
      <span className="mono">{clock}&nbsp;UTC</span>
      <div className="sep" />
      {user ? (
        <>
          <Link className="link" href={`/profile/${user.id}`} title="Open your profile">
            {handle(user)}
          </Link>
          <div className="sep" />
          <button className="btn btn--ghost" onClick={onLogout}>
            log out
          </button>
        </>
      ) : (
        <Link className="link" href="/login">
          log in
        </Link>
      )}
    </header>
  )
}
