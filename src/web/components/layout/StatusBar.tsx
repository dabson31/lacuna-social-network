'use client'

import { useEffect, useState } from 'react'
import { usePrefersReducedMotion } from '@/hooks/usePrefersReducedMotion'
import { useSocket } from '@/lib/realtime/SocketProvider'

/**
 * the bottom strip. rev 15 and the fk flag are true, the uptime and peer tally are the
 * design's synthetic readouts and keep its wording
 */
const LINES = [
  'bus: events.post_created → fanout 142 inboxes → 18ms',
  'mirror: segment 04 retry 6/10 — no route to host',
  'gc: 2 expired sessions purged · 1 upload orphan reclaimed',
  'ws: hub holds 3 connections · 1 room',
  'sqlite: wal checkpoint · 412 pages',
]

export function StatusBar() {
  const reduced = usePrefersReducedMotion()
  const { status } = useSocket()
  const [up, setUp] = useState(41 * 3600 + 12 * 60 + 6)
  const [line, setLine] = useState(0)

  useEffect(() => {
    const id = setInterval(() => setUp((n) => n + 1), 1000)
    return () => clearInterval(id)
  }, [])

  useEffect(() => {
    //! gated on the motion preference, like the mockup's four other intervals. css alone
    //! cannot stop a setInterval, so the javascript half has to check too
    if (reduced) return
    const id = setInterval(() => setLine((n) => (n + 1) % LINES.length), 5200)
    return () => clearInterval(id)
  }, [reduced])

  const pad = (n: number) => String(n).padStart(2, '0')
  const uptime = `${pad(Math.floor(up / 3600))}:${pad(Math.floor(up / 60) % 60)}:${pad(up % 60)}`

  return (
    <footer className="statusbar">
      <div className="cell">
        <b>rev 15</b>
      </div>
      <div className="cell hide-sm">
        sqlite · fk <b>on</b>
      </div>
      <div className="cell hide-sm">
        uptime <b>{uptime}</b>
      </div>
      <div className="cell">
        socket <b>{status}</b>
      </div>
      <div className="tick-area">
        <span>{LINES[line]}</span>
      </div>
      <div className="cell hide-sm" style={{ borderRight: 0 }}>
        one node · sqlite
      </div>
    </footer>
  )
}
