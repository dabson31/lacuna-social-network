'use client'

import { useCallback, useEffect, useRef, useState } from 'react'
import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { usePrefersReducedMotion } from '@/hooks/usePrefersReducedMotion'
import { useBadges } from './BadgeProvider'
import { CHANNELS, channelFor } from './channels'
import { Glyph } from './Glyph'

/**
 * the index bay: a tiled board in the left rail that turns to face you.
 * ! the rotation is entirely css, driven by data-open on .bay and the --rx / --ry custom
 * ! properties on .board. this component only sets those, which is why the port keeps the
 * ! design exactly: none of the geometry is reimplemented here
 */

const COUNT_LABEL: Record<string, string> = {
  feed: 'entries',
  directory: 'accounts',
  groups: 'groups',
  events: 'upcoming',
  messages: 'unread',
  notices: 'pending',
}

type Scheme = 'graphite' | 'umber'

export function RailLeft() {
  const pathname = usePathname()
  const reduced = usePrefersReducedMotion()
  const badges = useBadges()
  const current = channelFor(pathname)

  const bay = useRef<HTMLDivElement>(null)
  const board = useRef<HTMLDivElement>(null)
  const nav = useRef<HTMLUListElement>(null)

  const [open, setOpen] = useState(false)
  /** which channel the readout under the tiles is describing, hover wins over the route */
  const [shown, setShown] = useState<string>(current.id)
  const [scheme, setScheme] = useState<Scheme>('graphite')
  const [focusIx, setFocusIx] = useState(0)

  //? the inline script in the root layout has already applied the stored scheme before paint,
  //? so this only reads back what the document is already wearing
  useEffect(() => {
    setScheme(document.documentElement.dataset.scheme === 'umber' ? 'umber' : 'graphite')
  }, [])

  useEffect(() => {
    if (!open) setShown(current.id)
  }, [open, current.id])

  const countFor = useCallback(
    (id: string): number | '' => {
      switch (id) {
        case 'notices':
          return badges.pending || ''
        case 'messages':
          //! both kinds. a group room is a conversation on this tile's own list, so a tile
          //! labelled "unread" that only counted dms sat blank while rooms had unread messages
          return badges.unread + badges.unreadGroups || ''
        case 'groups':
          return badges.groups || ''
        default:
          return ''
      }
    },
    [badges],
  )

  const tiles = useCallback(
    () => Array.from(nav.current?.querySelectorAll<HTMLButtonElement>('.tile') ?? []),
    [],
  )

  /** how many columns the tile grid currently has, read off the computed style as the mockup does */
  const cols = useCallback(() => {
    if (!nav.current) return 2
    return getComputedStyle(nav.current).gridTemplateColumns.split(' ').length
  }, [])

  /**
   * the board leans toward whichever tile you are on. ported as written, including the
   * wide-layout branch, because the numbers are tuned to the design and not derived
   */
  const lean = useCallback(
    (i: number) => {
      if (reduced || !open || !board.current || !nav.current) return
      const c = cols()
      const n = tiles().length
      const col = i % c
      const row = Math.floor(i / c)
      const rows = Math.ceil(n / c)
      const wide = c <= 2
      const ry = wide ? 7 + (col - (c - 1) / 2) * 6 : (col - (c - 1) / 2) * 1.6
      const rx = wide ? 1 - (row - (rows - 1) / 2) * 2.2 : 3 - (row - (rows - 1) / 2) * 2
      board.current.style.setProperty('--ry', ry.toFixed(2))
      board.current.style.setProperty('--rx', rx.toFixed(2))
    },
    [reduced, open, cols, tiles],
  )

  const close = useCallback(() => {
    setOpen(false)
    board.current?.style.removeProperty('--ry')
    board.current?.style.removeProperty('--rx')
    setShown(current.id)
  }, [current.id])

  const applyScheme = useCallback((next: Scheme) => {
    if (next === 'umber') document.documentElement.dataset.scheme = 'umber'
    else delete document.documentElement.dataset.scheme
    setScheme(next)
    //? one of the few genuinely per-viewer conveniences in this app, which is exactly what
    //? local storage is for. wrapped because a private window can throw on access
    try {
      localStorage.setItem('lacuna.scheme', next)
    } catch {
      /* no persistence available, the choice still applies for this page */
    }
  }, [])

  /**
   * the keyboard grid.
   * ! the arrow arithmetic is the mockup's, copied rather than rederived. it is a wrapping 2d
   * ! walk that already handles a ragged last row, and getting it subtly wrong is the kind of
   * ! thing nobody notices until somebody tabs through it in an audit
   */
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const active = document.activeElement
      if (!bay.current?.contains(active) || !(active instanceof HTMLElement) || !active.classList.contains('tile')) {
        return
      }
      const t = tiles()
      const n = t.length
      const c = cols()
      let f = focusIx

      if (e.key === 'Escape') {
        e.preventDefault()
        active.blur()
        close()
        return
      }
      if (/^[1-7]$/.test(e.key)) {
        e.preventDefault()
        t[Number(e.key) - 1]?.click()
        return
      }
      if (e.key === 'ArrowRight') f = (f + 1) % n
      else if (e.key === 'ArrowLeft') f = (f + n - 1) % n
      else if (e.key === 'ArrowDown') f = f + c < n ? f + c : f % c
      else if (e.key === 'ArrowUp') f = f - c >= 0 ? f - c : Math.min(n - 1, (f % c) + c * Math.floor((n - 1 - (f % c)) / c))
      else if (e.key === 'Home') f = 0
      else if (e.key === 'End') f = n - 1
      else return

      e.preventDefault()
      t[f]?.focus()
      setFocusIx(f)
      lean(f)
    }
    addEventListener('keydown', onKey)
    return () => removeEventListener('keydown', onKey)
  }, [focusIx, tiles, cols, close, lean])

  const schemeDesc =
    scheme === 'umber'
      ? 'Demo switch. Umber is showing. Press to return the whole interface to graphite.'
      : 'Demo switch. Graphite is showing. Press to repaint the whole interface in umber.'

  const infoChannel = CHANNELS.find((c) => c.id === shown)
  const infoCount = infoChannel ? countFor(infoChannel.id) : ''

  return (
    <nav className="rail-l" aria-label="Primary">
      <div
        className="bay"
        id="bay"
        ref={bay}
        data-open={open ? '1' : '0'}
        onPointerEnter={(e) => {
          if (e.pointerType !== 'touch') setOpen(true)
        }}
        onPointerLeave={(e) => {
          if (e.pointerType !== 'touch') close()
        }}
        onFocus={() => setOpen(true)}
        onBlur={(e) => {
          if (!e.currentTarget.contains(e.relatedTarget as Node | null)) close()
        }}
      >
        <div className="bay__stage">
          <div className="board" id="board" ref={board}>
            <div className="board__hd">
              <i />
              <span className="lbl">Index</span>
              <span className="spacer" />
              <span className="lbl">layer 02 / {current.name.toLowerCase()}</span>
            </div>

            <ul className="nav" ref={nav}>
              {CHANNELS.map((c, i) => {
                const n = countFor(c.id)
                return (
                  <li key={c.id}>
                    <Link
                      className="tile"
                      href={c.href}
                      aria-current={c.id === current.id ? 'page' : undefined}
                      onPointerOver={() => {
                        setShown(c.id)
                        lean(i)
                      }}
                      onFocus={() => {
                        setShown(c.id)
                        setFocusIx(i)
                        lean(i)
                      }}
                    >
                      <Glyph id={c.id} />
                      <span className="nm">{c.name}</span>
                      <span className="ct">{n}</span>
                      <span className="ix">{c.ix}</span>
                    </Link>
                  </li>
                )
              })}

              {/* the eighth slot. deliberately not in CHANNELS: it is not a destination, which
                  is also why the 1-7 shortcut never reaches it */}
              <li>
                <button
                  type="button"
                  className="tile tile--scheme"
                  aria-pressed={scheme === 'umber'}
                  aria-label="Colour scheme demo: switch between graphite and umber"
                  onPointerOver={() => setShown('scheme')}
                  onFocus={() => setShown('scheme')}
                  onClick={() => applyScheme(scheme === 'umber' ? 'graphite' : 'umber')}
                >
                  <span className="sw" aria-hidden="true">
                    <i data-s="graphite" data-on={scheme === 'graphite' ? '1' : '0'} />
                    <i data-s="umber" data-on={scheme === 'umber' ? '1' : '0'} />
                  </span>
                  <span className="nm">Scheme</span>
                  <span className="ct">{scheme}</span>
                </button>
              </li>
            </ul>

            <div className="info" aria-live="polite">
              <div className="info__in">
                <div className="info__hd">
                  <b>{shown === 'scheme' ? 'Scheme' : (infoChannel?.name ?? '')}</b>
                  <span className="n">
                    {shown === 'scheme' ? (
                      <>
                        {scheme}
                        <small>active</small>
                      </>
                    ) : infoCount === '' ? null : (
                      <>
                        {infoCount}
                        <small>{COUNT_LABEL[shown] ?? ''}</small>
                      </>
                    )}
                  </span>
                </div>
                <p>{shown === 'scheme' ? schemeDesc : (infoChannel?.sub ?? '')}</p>
                <div className="info__ft">
                  <figure className="lain" aria-hidden="true">
                    {/* extracted out of the mockup's 64kb inline data uri into a real asset.
                        same bytes, same dimensions, served from /public instead of the html */}
                    <img src="/lain.gif" alt="" width={166} height={248} />
                  </figure>
                  <span className="hint">
                    <b>↑↓←→</b> move
                    <br />
                    <b>enter</b> open · <b>1–7</b> jump
                    <br />
                    <b>esc</b> release
                  </span>
                </div>
              </div>
            </div>
            <span className="tick" aria-hidden="true" />
          </div>
        </div>
      </div>

      <div className="foot">
        <p>schema rev 15</p>
        <p>sqlite · fk on</p>
        <p>closed network</p>
        <p style={{ color: 'var(--ink-faint)' }}>no exit node</p>
      </div>
    </nav>
  )
}
