'use client'

import Link from 'next/link'
import { Avatar } from '@/components/ui/Avatar'
import { Empty, Loading } from '@/components/ui/Empty'
import { useBadges } from '@/components/layout/BadgeProvider'
import { displayName } from '@/lib/utils/identity'

/**
 * the inbox, ported from viewMessages' list branch at line 3967.
 * ! two lists, because GET /api/conversations returns both halves and the backend has group
 * ! rooms fully built. the design only ever showed dms, so the second .rows block is the
 * ! addition the subject forces — "groups should have a common chat room" — and it reuses the
 * ! same row structure, so it needs no new css
 */
export default function InboxPage() {
  const { inbox } = useBadges()

  if (!inbox) return <Loading label="reading conversations…" />

  const unread = inbox.direct.reduce((n, c) => n + c.unread, 0)
  const unreadGroups = inbox.groups.reduce((n, c) => n + c.unread, 0)

  return (
    <>
      <div className="main__bar">
        <span className="lbl">
          {inbox.direct.length} {inbox.direct.length === 1 ? 'conversation' : 'conversations'}
        </span>
        <div className="spacer" />
        <span className="lbl">{unread || 'no'} unread</span>
      </div>

      <div className="rows">
        {inbox.direct.map((c) => (
          <div className="row-item" key={c.peer_id}>
            <Link className="row-main" href={`/chat/${c.peer_id}`}>
              <span className={c.unread ? 'dot dot--on' : 'dot dot--idle'} />
              <Avatar name={displayName(c)} path={c.avatar_path} size="sm" />
              <span className="tx">
                <span className="nm">
                  {displayName(c)}
                  {c.unread > 0 && <span className="row-unread">{c.unread} new</span>}
                </span>
                {/*
                  ! an empty last_message is a normal row and not missing data. the model says
                  ! so: it is somebody you can chat with but never have, which is what makes
                  ! this list double as a contact list
                */}
                <span className="ex">{c.last_message || 'no messages yet'}</span>
              </span>
              <span className="caret" aria-hidden="true">
                ›
              </span>
            </Link>
          </div>
        ))}

        {inbox.direct.length === 0 && (
          <Empty label="nobody to talk to yet · follow somebody, or be followed" />
        )}
      </div>

      <div className="main__bar">
        <span className="lbl">group rooms</span>
        <div className="spacer" />
        {/*? the same readout the dm bar above carries, so each half reports its own unread */}
        <span className="lbl">
          {inbox.groups.length} · {unreadGroups || 'no'} unread
        </span>
      </div>

      <div className="rows">
        {inbox.groups.map((g) => (
          <div className="row-item" key={g.group_id}>
            <Link className="row-main" href={`/groups/${g.group_id}/chat`}>
              <span className={g.unread ? 'dot dot--on' : 'dot dot--idle'} />
              <Avatar name={g.title} size="sm" />
              <span className="tx">
                <span className="nm">
                  {g.title}
                  {g.unread > 0 && <span className="row-unread">{g.unread} new</span>}
                </span>
                <span className="ex">
                  {g.last_message ? `${g.last_sender}: ${g.last_message}` : 'no messages yet'}
                </span>
              </span>
              <span className="caret" aria-hidden="true">
                ›
              </span>
            </Link>
          </div>
        ))}

        {inbox.groups.length === 0 && <Empty label="no group rooms · join a group to get one" />}
      </div>
    </>
  )
}
