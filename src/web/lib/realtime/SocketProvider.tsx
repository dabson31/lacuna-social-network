'use client'

import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react'
import { WS_PATH, WsFrame } from '@shared/constants/ws'
import { assertNever, parseServerFrame, type ClientFrame, type ServerFrame } from '@shared/contracts/ws'
import * as api from '@/lib/api/client'
import { isApiError } from '@/lib/api/errors'
import { useSession } from '@/lib/session/SessionProvider'

/**
 * one socket for the whole application, shared by chat and notifications.
 * ? two sockets would be two reconnect loops, two sets of duplicate frames, and the hub keys
 * ? connections by user id so it would hold both and fan out to both
 */

export type SocketStatus = 'idle' | 'connecting' | 'open' | 'reconnecting' | 'closed'

/** one row of the design's Socket panel */
export interface FrameLogEntry {
  seq: number
  type: string
  /** the panel's short kind tag: dm, grp, ntf, or empty for a transport frame */
  kind: 'dm' | 'grp' | 'ntf' | ''
  payload: string
  outbound: boolean
}

type Listener = (frame: ServerFrame) => void

interface SocketContextValue {
  status: SocketStatus
  send: (frame: ClientFrame) => void
  /** returns its own unsubscribe, so an effect can just return it */
  subscribe: (fn: Listener) => () => void
  log: FrameLogEntry[]
  framesIn: number
  framesOut: number
  rtt: number | null
  paused: boolean
  setPaused: (v: boolean) => void
}

const SocketContext = createContext<SocketContextValue | null>(null)

const PING_MS = 25_000
const LOG_MAX = 6

/**
 * the close code the server sends when the session behind this socket is gone — a logout in
 * another tab, or the session's own ttl running out.
 * ! 1008 is policy violation, and it is the only way this tab can find out. the session lives
 * ! in one row shared by every tab of the browser, while `state` is per tab and in memory, so
 * ! the tab that did not click logout has nothing else to go on: it is idle in a chat, makes
 * ! no rest call, and the global 401 hook never fires
 */
const CLOSED_BY_SERVER = 1008

function kindOf(type: string): FrameLogEntry['kind'] {
  if (type === WsFrame.DmMessage || type === WsFrame.DmTyping) return 'dm'
  if (type === WsFrame.GroupMessage || type === WsFrame.GroupPost) return 'grp'
  if (type === WsFrame.NotificationNew || type === WsFrame.NotificationGone) return 'ntf'
  return ''
}

/** the one line summary the Socket panel prints per frame, in the mockup's own style */
function summarise(frame: ServerFrame): string {
  switch (frame.type) {
    case WsFrame.DmMessage:
      return `from ${frame.payload.sender_id} · ${frame.payload.content.length} bytes`
    case WsFrame.GroupMessage:
      return `group ${frame.payload.group_id} · from ${frame.payload.sender_id}`
    case WsFrame.GroupPost:
      return `group ${frame.payload.group_id} · post ${frame.payload.id}`
    case WsFrame.DmTyping:
      return `peer ${frame.payload.from_user_id}`
    case WsFrame.NotificationNew:
      return `${frame.payload.type} · actor ${frame.payload.actor_id}`
    case WsFrame.NotificationGone:
      return `${frame.payload.ids.length} retracted`
    case WsFrame.Pong:
      return 'keepalive'
    default:
      return assertNever(frame)
  }
}

export function SocketProvider({ children }: { children: React.ReactNode }) {
  const { state, signOut } = useSession()
  const [status, setStatus] = useState<SocketStatus>('idle')
  const [log, setLog] = useState<FrameLogEntry[]>([])
  const [framesIn, setFramesIn] = useState(0)
  const [framesOut, setFramesOut] = useState(0)
  const [rtt, setRtt] = useState<number | null>(null)
  const [paused, setPaused] = useState(false)

  const ws = useRef<WebSocket | null>(null)
  const listeners = useRef(new Set<Listener>())
  const attempt = useRef(0)
  const seq = useRef(0)
  const pingSentAt = useRef(0)
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const pingTimer = useRef<ReturnType<typeof setInterval> | null>(null)
  //! set when a close follows an auth failure. the cookie is gone, so retrying forever is a
  //! loop with no exit and a wall of failed handshakes in the server log
  const giveUp = useRef(false)
  const pausedRef = useRef(paused)
  pausedRef.current = paused
  //? held in a ref rather than taken as a dependency of connect: signOut comes off a memo
  //? keyed on the session state, so depending on it would rebuild connect every time the
  //? session changes and the effect below would tear the socket down and build a new one
  const signOutRef = useRef(signOut)
  signOutRef.current = signOut
  //! set when the provider is coming down, which happens on every logout: the shell unmounts
  //! as the router leaves (main). a close event fires AFTER the cleanup has run, so without
  //! this the handler schedules a reconnect for a provider that no longer exists — and that
  //! attempt carries no cookie, fails, and schedules the next one
  const torn = useRef(false)

  const appendLog = useCallback((type: string, payload: string, outbound: boolean) => {
    if (pausedRef.current) return
    seq.current += 1
    const entry: FrameLogEntry = { seq: seq.current, type, kind: kindOf(type), payload, outbound }
    //! newest first and trimmed to six, which is what the panel is sized for. the mockup's
    //! emit() does the same with insertAdjacentHTML and a while loop
    setLog((prev) => [entry, ...prev].slice(0, LOG_MAX))
  }, [])

  const send = useCallback(
    (frame: ClientFrame) => {
      const sock = ws.current
      if (!sock || sock.readyState !== WebSocket.OPEN) return
      sock.send(JSON.stringify(frame))
      setFramesOut((n) => n + 1)
      if (frame.type === WsFrame.Ping) pingSentAt.current = Date.now()
      appendLog(frame.type, frame.type === WsFrame.DmTyping ? `peer ${frame.payload.peer_id}` : 'keepalive', true)
    },
    [appendLog],
  )

  const connect = useCallback(() => {
    if (giveUp.current || torn.current) return
    //! no token in the query string, ever. the handshake is an ordinary GET and the browser
    //! attaches the sid cookie to it on its own, which is the whole reason the gateway can sit
    //! behind requireAuth. a token in the url ends up in access logs, history and Referer
    const url = api.API_BASE.replace(/^http/, 'ws') + WS_PATH
    setStatus(attempt.current === 0 ? 'connecting' : 'reconnecting')

    let sock: WebSocket
    try {
      sock = new WebSocket(url)
    } catch {
      scheduleReconnect()
      return
    }
    ws.current = sock

    let opened = false

    sock.onopen = () => {
      opened = true
      attempt.current = 0
      setStatus('open')
      pingTimer.current = setInterval(() => {
        //! the server pings from writePump too, so this is not redundant with it, it is the
        //! mirror. the browser api cannot send a protocol level ping, which is why TypePing
        //! exists server side at all, and the pong it answers with is where rtt comes from
        send({ type: WsFrame.Ping })
      }, PING_MS)
    }

    sock.onmessage = (ev: MessageEvent<string>) => {
      //! every inbound message goes through parseServerFrame first. it is already written, it
      //! validates the envelope, and it returns null for an unknown type rather than throwing.
      //! anything off a socket is untrusted input and gets one real check at the boundary
      const frame = parseServerFrame(ev.data)
      if (!frame) return

      setFramesIn((n) => n + 1)
      if (frame.type === WsFrame.Pong && pingSentAt.current) {
        setRtt(Date.now() - pingSentAt.current)
        pingSentAt.current = 0
      }
      appendLog(frame.type, summarise(frame), false)
      for (const fn of listeners.current) fn(frame)
    }

    sock.onclose = (ev: CloseEvent) => {
      if (pingTimer.current) clearInterval(pingTimer.current)
      pingTimer.current = null
      ws.current = null
      //? nothing to reconnect to and nobody to tell: this provider is already gone
      if (torn.current) return

      /**
       * ! the session that authenticated this socket is gone — somebody logged out, in this
       * ! tab or another one, or the ttl ran out. the socket is the only thing that can tell
       * ! this tab, because the session is one row shared by every tab while the react state
       * ! is per tab, and a tab sitting in a chat makes no rest call to trip the 401 hook
       * ! so this is not only "stop retrying": the state has to go anonymous, which is what
       * ! (main)/layout.tsx redirects on. leaving it authenticated is the bug — the tab stays
       * ! in the chat it is no longer entitled to and keeps sending typing notices
       */
      if (ev.code === CLOSED_BY_SERVER) {
        giveUp.current = true
        setStatus('closed')
        signOutRef.current()
        return
      }

      /**
       * ! a handshake refused BEFORE the upgrade never becomes a websocket, so the browser
       * ! reports 1006 and no reason — the same thing a dropped wifi looks like. the one
       * ! difference available is that a refused handshake never fired onopen
       * ? so ask the only thing that can actually answer. a 401 from `me` is a dead session and
       * ? the tab signs out; anything else, including the server being down, is left to the
       * ? backoff. `me` is exempt from the global 401 hook on purpose, which is why the status
       * ? is checked here rather than relying on it firing
       */
      if (!opened) {
        void api
          .json('/api/auth/me')
          .catch((e: unknown) => {
            if (isApiError(e) && e.status === 401) {
              giveUp.current = true
              setStatus('closed')
              signOutRef.current()
            }
          })
      }

      scheduleReconnect()
    }

    sock.onerror = () => {
      // onclose always follows, so the backoff is scheduled there and not twice
    }

    function scheduleReconnect() {
      if (giveUp.current) return
      setStatus('reconnecting')
      //! cap the backoff and jitter it. without the cap a laptop that slept for an hour
      //! hammers the server on wake, and without the jitter every socket that dropped on a
      //! restart comes back in the same tick, which is a thundering herd of your own making
      const delay = Math.min(30_000, 500 * 2 ** attempt.current) * (0.5 + Math.random() / 2)
      attempt.current += 1
      reconnectTimer.current = setTimeout(connect, delay)
    }
  }, [appendLog, send])

  useEffect(() => {
    //! only connect once the session is known to exist. connecting while loading races the
    //! cookie and the handshake comes back 401, which then looks like the socket being broken
    if (state.status !== 'authenticated') return
    giveUp.current = false
    torn.current = false
    attempt.current = 0
    connect()

    return () => {
      torn.current = true
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current)
      if (pingTimer.current) clearInterval(pingTimer.current)
      const sock = ws.current
      ws.current = null
      //? 1000 is a normal close, so the server unregisters cleanly instead of waiting for a
      //? read timeout on a connection that is already gone
      if (sock && (sock.readyState === WebSocket.OPEN || sock.readyState === WebSocket.CONNECTING)) sock.close(1000)
      setStatus('idle')
    }
  }, [state.status, connect])

  const subscribe = useCallback((fn: Listener) => {
    listeners.current.add(fn)
    return () => {
      listeners.current.delete(fn)
    }
  }, [])

  const value = useMemo<SocketContextValue>(
    () => ({ status, send, subscribe, log, framesIn, framesOut, rtt, paused, setPaused }),
    [status, send, subscribe, log, framesIn, framesOut, rtt, paused],
  )

  return <SocketContext.Provider value={value}>{children}</SocketContext.Provider>
}

export function useSocket(): SocketContextValue {
  const ctx = useContext(SocketContext)
  if (!ctx) throw new Error('useSocket used outside SocketProvider')
  return ctx
}

/** the payload that belongs to one frame type, which is what a handler for it receives */
type PayloadOf<K extends ServerFrame['type']> = Extract<ServerFrame, { type: K }>['payload']

/**
 * subscribe to one frame type with a narrowed payload.
 * ! the generic is what keeps the union intact at the call site: a dm.message handler receives
 * ! a DirectMessage and nothing else, so adding a field to a frame breaks the consumers that
 * ! read it rather than handing them undefined
 *
 * ! the one cast is here and it is unavoidable. inside the generic, K is still a type variable,
 * ! so Extract cannot resolve and typescript widens frame.payload to the union of all five
 * ! payloads while the handler expects exactly one. the runtime check on the line above is the
 * ! proof the compiler cannot reconstruct, which is precisely what a boundary cast is for
 */
export function useFrame<K extends ServerFrame['type']>(
  type: K,
  handler: (payload: PayloadOf<K>) => void,
  deps: readonly unknown[] = [],
): void {
  const { subscribe } = useSocket()
  const fn = useRef(handler)
  fn.current = handler

  useEffect(() => {
    return subscribe((frame) => {
      if (frame.type !== type) return
      fn.current(frame.payload as PayloadOf<K>)
    })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [subscribe, type, ...deps])
}
