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
              <span className="dot dot--idle" />
              <span className="nm off">{handle(p)}</span>
              <span className="st">{p.unread ? `${p.unread} new` : p.last_at ? 'quiet' : 'no history'}</span>
            </button>
          </li>
        ))}
      </ul>
      {peers.length === 0 && <p className="note-txt">Follow somebody, or be followed, and they appear here.</p>}
    </section>
  )
}
