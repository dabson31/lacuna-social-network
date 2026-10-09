'use client'

import { useEffect, useRef, useState } from 'react'
import { usePrefersReducedMotion } from '@/hooks/usePrefersReducedMotion'
import { useToast } from '@/components/ui/Toast'
import { useSocket } from '@/lib/realtime/SocketProvider'
import { LOOP, LOOP_MS, type LoopFrame } from './loop-frames'

/**
 * the panels the design carries as atmosphere.
 * ! these keep the .tag--sim marker the design gives them, because wiring them would mean
 * ! inventing a distributed system that does not exist: this is one process over one sqlite
 * ! file and there are no segments, no relays and no mirrors. deleting them changes the
 * ! design, un-labelling them would be a lie, so they stay and they stay marked
 * ? the one exception is the frames row in Link, which counts real frames off the real socket
 */

export function LinkPanel() {
  const { framesIn, framesOut } = useSocket()
  const [open, setOpen] = useState<string | null>(null)
  const row = (id: string) => ({
    'aria-expanded': open === id,
    'aria-controls': `lk-${id}`,
    onClick: () => setOpen(open === id ? null : id),
  })

  return (
    <section className="sec" aria-labelledby="linkHd">
      <div className="sec__hd">
        <span className="lbl" id="linkHd">
          Link
        </span>
        <div className="spacer" />
        <span className="tag tag--sim">sim</span>
      </div>
      <div className="link-list">
        <div className="link-row">
          <button className="link-row__main" {...row('route')}>
            <span className="dot dot--on" />
            <span className="k">route</span>
            <span className="v">2 hops</span>
            <span className="caret" aria-hidden="true">›</span>
          </button>
          <div className="link-row__body" id="lk-route" hidden={open !== 'route'}>
            <ul>
              <li><span className="dot dot--on" />lcn·7f3a<em>self</em></li>
              <li><span className="dot dot--rly" />relay·02<em>11 ms</em></li>
              <li><span className="dot dot--warn" />seg·04<em>no route</em></li>
            </ul>
          </div>
        </div>

        <div className="link-row">
          <button className="link-row__main" {...row('seg')}>
            <span className="dot dot--warn" />
            <span className="k">segments</span>
            <span className="v warn">5 / 6</span>
            <span className="caret" aria-hidden="true">›</span>
          </button>
          <div className="link-row__body" id="lk-seg" hidden={open !== 'seg'}>
            <ul>
              <li><span className="dot dot--on" />seg 01<em>42 nodes</em></li>
              <li><span className="dot dot--on" />seg 02<em>37 nodes</em></li>
              <li><span className="dot dot--on" />seg 03<em>29 nodes</em></li>
              <li><span className="dot dot--warn" />seg 04<em>14 silent</em></li>
              <li><span className="dot dot--on" />seg 05<em>21 nodes</em></li>
              <li><span className="dot dot--rly" />seg 06<em>19 relayed</em></li>
            </ul>
          </div>
        </div>

        {/* the one real row in this panel */}
        <div className="link-row">
          <button className="link-row__main" {...row('frames')}>
            <span className="dot dot--rly" />
            <span className="k">frames</span>
            <span className="v">
              {framesIn} ↓ / {framesOut} ↑
            </span>
            <span className="caret" aria-hidden="true">›</span>
          </button>
          <div className="link-row__body" id="lk-frames" hidden={open !== 'frames'}>
            <p>Counted since this page loaded, off the live socket. Inbound are frames the server pushed, outbound are typing notices and keepalives.</p>
          </div>
        </div>

        <div className="link-row">
          <button className="link-row__main" {...row('queue')}>
            <span className="dot dot--idle" />
            <span className="k">queue</span>
            <span className="v">0 held</span>
            <span className="caret" aria-hidden="true">›</span>
          </button>
          <div className="link-row__body" id="lk-queue" hidden={open !== 'queue'}>
            <p>Outbound frames waiting on an unreachable peer. Anything addressed to segment 04 lands here until the route recovers.</p>
          </div>
        </div>
      </div>
    </section>
  )
}

/** 44 bars of pure css, driven by a sine and some noise. synthetic, as the design says */
export function CarrierPanel() {
  const reduced = usePrefersReducedMotion()
  const bars = useRef<HTMLDivElement>(null)
  const [read, setRead] = useState({ in: '41.2', out: '8.7' })

  useEffect(() => {
    const host = bars.current
    if (!host) return
    //? built imperatively because it is 44 presentational elements with a height each, and
    //? re-rendering them through react four times a second would be pure waste
    host.replaceChildren(...Array.from({ length: 44 }, () => document.createElement('i')))
    const cells = Array.from(host.children) as HTMLElement[]

    const paint = () => {
      let sum = 0
      cells.forEach((el, i) => {
        const v = Math.abs(Math.sin(Date.now() / 2600 + i / 3.1)) * 0.75 + Math.random() * 0.25
        sum += v
        el.style.height = `${14 + v * 86}%`
        el.className = v > 0.82 ? 'hi' : v > 0.45 ? 'on' : ''
      })
      const avg = sum / cells.length
      setRead({ in: (30 + avg * 26).toFixed(1), out: (4 + avg * 9).toFixed(1) })
    }

    paint()
    if (reduced) return
    const id = setInterval(paint, 420)
    return () => clearInterval(id)
  }, [reduced])

  return (
    <section className="sec" aria-labelledby="carHd">
      <div className="sec__hd">
        <span className="lbl" id="carHd">
          Carrier
        </span>
        <div className="spacer" />
        <span className="tag tag--sim">sim</span>
      </div>
      <div className="carrier">
        <div className="bars" ref={bars} aria-hidden="true" />
        <div className="read">
          <span>in {read.in} kb/s</span>
          <span>out {read.out}</span>
        </div>
      </div>
    </section>
  )
}

/**
 * the integrity readout.
 * ? schema rev 15 and fk on are both true, the rest is the design's fiction. /api/health only
 * ? answers whether the database is reachable, which is the one row that could be made real
 */
const INTEGRITY = [
  { k: 'schema', v: 'rev 15 · clean', cls: 'ok', note: 'All 15 migrations applied, each with a matching down. Verified at boot.' },
  { k: 'fk check', v: '0 violations', cls: 'ok', note: 'PRAGMA foreign_key_check returned nothing on the last pass.' },
  { k: 'uploads', v: '1.4 gb / 8 gb', cls: 'ok', note: 'Files on disk under data/uploads. Paths only in the database.' },
  { k: 'sessions', v: '3 active', cls: 'ok', note: 'Three server-side sessions open. Two browsers plus this one.' },
  { k: 'last mirror', v: 'failed 02:17', cls: 'warn', note: 'Segment 04 refused the handshake. Six retries, no route to host.' },
  { k: 'digest', v: '7f3a·9c4e·a118', cls: '', note: 'Content digest of this node. Click the row to copy it.', copy: true },
]

export function IntegrityPanel() {
  const [open, setOpen] = useState<number | null>(null)
  const toast = useToast()

  return (
    <section className="sec" aria-labelledby="intgHd">
      <div className="sec__hd">
        <span className="lbl" id="intgHd">
          Integrity
        </span>
        <div className="spacer" />
        <span className="tag tag--sim">sim</span>
      </div>
      <div className="intg">
        {INTEGRITY.map((r, i) => (
          <div key={r.k}>
            <button
              className="intg__row"
              aria-expanded={open === i}
              aria-controls={`ig-${i}`}
              onClick={() => {
                setOpen(open === i ? null : i)
                if (r.copy) {
                  void navigator.clipboard?.writeText(r.v).then(() => toast(`digest · ${r.v}`))
                }
              }}
            >
              <span className="k">{r.k}</span>
              <span className={`v ${r.cls}`}>{r.v}</span>
              <span className="caret" aria-hidden="true">›</span>
            </button>
            <p className="intg__note" id={`ig-${i}`} hidden={open !== i}>
              {r.note}
            </p>
          </div>
        ))}
      </div>
    </section>
  )
}

/**
 * the ascii frame player, a static asset rendered unchanged.
 * ? the mockup builds each cell as a span through innerHTML. here each row is one element and
 * ? the tinted runs inside it are spans, which is the same output with far fewer nodes
 */
function FrameRow({ row, red }: { row: string; red: string }) {
  const out: React.ReactNode[] = []
  let i = 0
  while (i < row.length) {
    // walk runs of identical styling rather than emitting one span per character
    const styleOf = (x: number) => (red[x] === '1' ? 'r' : row[x] === '@' || row[x] === '%' ? 'hi' : '')
    const cls = styleOf(i)
    let j = i
    while (j < row.length && styleOf(j) === cls) j++
    const text = row.slice(i, j)
    out.push(cls ? <span className={cls} key={i}>{text}</span> : text)
    i = j
  }
  return <>{out}{'\n'}</>
}

export function LoopPanel() {
  const reduced = usePrefersReducedMotion()
  const [ix, setIx] = useState(0)
  const [held, setHeld] = useState(false)

  useEffect(() => {
    if (reduced || held || LOOP.length < 2) return
    const id = setInterval(() => setIx((n) => (n + 1) % LOOP.length), LOOP_MS)
    return () => clearInterval(id)
  }, [reduced, held])

  const frame: LoopFrame = LOOP[ix] ?? { rows: [], red: [] }
  const fps = LOOP.length > 1 ? `${Math.round(1000 / LOOP_MS)} fps` : 'still'

  return (
    <section className="sec" aria-labelledby="loopHd">
      <div className="sec__hd">
        <span className="lbl" id="loopHd">
          Loop
        </span>
        <span className="tag tag--sim">ascii</span>
        <div className="spacer" />
        <button className="btn btn--sm" aria-pressed={held} title="Pause the loop" onClick={() => setHeld(!held)}>
          hold
        </button>
      </div>
      <div className="loop" aria-hidden="true">
        <pre>
          <code>
            {frame.rows.map((row, y) => (
              <FrameRow key={y} row={row} red={frame.red[y] ?? ''} />
            ))}
          </code>
        </pre>
      </div>
      <div className="loop__ft" style={{ border: '1px solid var(--line)', borderTop: 0 }}>
        <span className="dot dot--on" />
        <span>50×24 · {fps}</span>
        <div className="spacer" />
        <span>
          frame {ix + 1} / {LOOP.length}
        </span>
      </div>
    </section>
  )
}

export function ArchiveNote() {
  return (
    <section className="sec">
      <div className="sec__hd">
        <span className="lbl">Archive note</span>
      </div>
      <p className="note-txt">
        This node holds its own copy of everything you can see. If it stops answering, nothing here is retrievable from
        anywhere else.
      </p>
      <p className="lbl" style={{ margin: '10px 0 0', color: 'var(--ink-ghost)' }}>
        present day · present time
      </p>
    </section>
  )
}
