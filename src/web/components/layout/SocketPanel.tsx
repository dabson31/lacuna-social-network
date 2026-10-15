'use client'

import { useToast } from '@/components/ui/Toast'
import { useSocket } from '@/lib/realtime/SocketProvider'

/**
 * the frame log, wired to the real socket instead of the mockup's scripted replay.
 * ! this stops being decoration the moment the frames are real: it is the fastest way to
 * ! answer "did the server actually send that" for every feature in the app. the sim tag
 * ! comes off, because this one is no longer synthetic
 */
export function SocketPanel() {
  const { log, status, framesIn, framesOut, paused, setPaused } = useSocket()
  const toast = useToast()

  const dotClass =
    status === 'open' ? 'dot dot--on' : status === 'reconnecting' || status === 'connecting' ? 'dot dot--warn' : 'dot dot--idle'

  return (
    <section className="sec" aria-labelledby="sockHd">
      <div className="sec__hd">
        <span className="lbl" id="sockHd">
          Socket
        </span>
        <div className="spacer" />
        <button
          className="btn btn--sm"
          aria-pressed={paused}
          title="Hold the frame log"
          onClick={() => setPaused(!paused)}
        >
          hold
        </button>
      </div>

      <div className="sock">
        {log.map((f) => (
          <button
            className="ln"
            type="button"
            key={f.seq}
            data-k={f.kind || undefined}
            onClick={() => toast(`${f.type} · ${f.payload}`)}
          >
            <span className="t">{f.outbound ? '↑' : '↓'}</span>
            <span className="ty">{f.type}</span>
            <span className="pl">{f.payload}</span>
          </button>
        ))}
      </div>

      <div className="sock__ft">
        <span className={dotClass} />
        <span>/ws multiplexed</span>
        <div className="spacer" />
        <span>
          {framesIn} ↓ / {framesOut} ↑
        </span>
      </div>
      <p className="note-txt" style={{ marginTop: 8, color: 'var(--ink-ghost)', fontSize: '10.5px' }}>
        Live frames off the one multiplexed socket. Click a frame to read it.
      </p>
    </section>
  )
}
