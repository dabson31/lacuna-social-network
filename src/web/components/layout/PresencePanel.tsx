'use client'

import { useRouter } from 'next/navigation'
import { displayName, handle } from '@/lib/utils/identity'
import { useBadges } from './BadgeProvider'

/**
 * the design's presence list, fed from the conversation list.
 * ! the dots are synthetic and the panel says so. the hub knows who is connected through
 * ! IsOnline, but no presence frame is ever sent — chat/service.go leaves presence.online as
 * ! a todo — so the client has no way to learn it. the frame type is already reserved in
 * ! shared/constants/ws.ts for whenever that changes
 * ? the rows themselves are real: these are the people you can actually open a conversation
 * ? with, which is what the design uses the list for
 */
export function PresencePanel() {
  const { inbox } = useBadges()
  const router = useRouter()
  const peers = (inbox?.direct ?? []).slice(0, 8)

  return (
    <section className="sec" aria-labelledby="presHd">
      <div className="sec__hd">
        <span className="lbl" id="presHd">
          Presence
        </span>
        <span className="tag tag--sim">sim</span>
        <div className="spacer" />
        <span className="lbl">{peers.length} reachable</span>
      </div>
      <ul className="presence">
        {peers.map((p) => (
          <li key={p.peer_id}>
            {/*
              ! a button and not a link, because the stylesheet puts the row's whole flex
              ! layout on `.presence button`. as an anchor the dot, name and status run
              ! together with an underline through them. the mockup uses a button here too
            */}
            <button
              type="button"
              title={`Open a conversation with ${displayName(p)}`}
              onClick={() => router.push(`/chat/${p.peer_id}`)}
            >
              {/*? the dot stays idle whatever the unread count is. this column means presence
                  in this panel, the panel wears a `sim` tag because presence is synthetic here,
                  and lighting it for unread messages would make one glyph mean two things */}
              <span className="dot dot--idle" />
              <span className="nm off">{handle(p)}</span>
              {/*
                ! the unread count used to print in --ink-ghost at 8.5px, the same as "quiet"
                ! and "no history", which made the one readout in the panel that is actually
                ! news indistinguishable from the two that are not
                ? the accent, inline, is what the chrome's own two counters do. the rest of the
                ? row keeps the design's weights
              */}
              <span className="st" style={p.unread ? { color: 'var(--acid)' } : undefined}>
                {p.unread ? `${p.unread} new` : p.last_at ? 'quiet' : 'no history'}
              </span>
              {/*? the mark every other openable row in the app carries, so this reads as a row
                  that goes somewhere rather than as a line of status text */}
              <span className="caret" aria-hidden="true">
                ›
              </span>
            </button>
          </li>
        ))}
      </ul>
      {peers.length === 0 && <p className="note-txt">Follow somebody, or be followed, and they appear here.</p>}
    </section>
  )
}
