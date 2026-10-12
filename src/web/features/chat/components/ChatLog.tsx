'use client'

import { useEffect, useRef } from 'react'
import { shortTime } from '@/lib/utils/time'

/** one bubble, from msgHTML at line 4056. .msg--me puts it on the right */
export interface Bubble {
  id: number
  mine: boolean
  who: string
  at: string
  content: string
}

/**
 * the design's .chat__log.
 * ! two scroll behaviours, and they are not the same rule. on open, jump to the bottom. on a
 * ! new message, only follow if the reader was already near the bottom — yanking the view while
 * ! somebody reads scrollback is worse than not showing the new line
 */
export function ChatLog({
  bubbles,
  onReachTop,
  loadingOlder,
  exhausted,
}: {
  bubbles: Bubble[]
  onReachTop: () => void
  loadingOlder: boolean
  exhausted: boolean
}) {
  const log = useRef<HTMLDivElement>(null)
  const nearBottom = useRef(true)
  const lastCount = useRef(0)
  //! the scroll height before older messages prepend, so the position can be restored. without
  //! it the view jumps to the top on every page and the reader loses their place, which is the
  //! classic infinite-scroll-upward bug
  const anchor = useRef<number | null>(null)

  useEffect(() => {
    const el = log.current
    if (!el) return

    if (anchor.current !== null) {
      // older messages just landed: keep the same content under the reader's eye
      el.scrollTop = el.scrollHeight - anchor.current
      anchor.current = null
    } else if (lastCount.current === 0 || nearBottom.current) {
      el.scrollTop = el.scrollHeight
    }
    lastCount.current = bubbles.length
  }, [bubbles])

  const onScroll = () => {
    const el = log.current
    if (!el) return
    nearBottom.current = el.scrollHeight - el.scrollTop - el.clientHeight < 80
    if (el.scrollTop < 40 && !loadingOlder && !exhausted) {
      anchor.current = el.scrollHeight - el.scrollTop
      onReachTop()
    }
  }

  return (
    <div className="chat__log" ref={log} onScroll={onScroll}>
      {loadingOlder && <p className="note-txt">reading older…</p>}
      {exhausted && bubbles.length > 0 && (
        <div className="eof">
          <div className="ln" />
          <span className="lbl">beginning of this conversation</span>
        </div>
      )}
      {bubbles.map((b) => (
        <div className={b.mine ? 'msg msg--me' : 'msg'} key={b.id}>
          <div className="h">
            {b.mine ? 'you' : b.who} · {shortTime(b.at)}
          </div>
          <div className="bd">{b.content}</div>
        </div>
      ))}
    </div>
  )
}
