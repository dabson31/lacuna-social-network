'use client'

import { useState } from 'react'
import Link from 'next/link'
import { useRouter } from 'next/navigation'
import type { ListUser } from '@shared/contracts/users'
import { Avatar } from '@/components/ui/Avatar'
import { Empty, Loading } from '@/components/ui/Empty'
import { useBadges } from '@/components/layout/BadgeProvider'
import { useFetch } from '@/lib/api/useFetch'
import { useDebounced } from '@/hooks/useDebounced'
import { displayName, handle } from '@/lib/utils/identity'
import { searchUsers } from '@/features/profile/api'

/**
 * the inbox, ported from viewMessages' list branch at line 3967.
 * ! two lists, because GET /api/conversations returns both halves and the backend has group
 * ! rooms fully built. the design only ever showed dms, so the second .rows block is the
 * ! addition the subject forces — "groups should have a common chat room" — and it reuses the
 * ! same row structure, so it needs no new css
 */
export default function InboxPage() {
  const { inbox } = useBadges()
  const [starting, setStarting] = useState(false)

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
        {/*
          ! the inbox already lists everybody an accepted follow makes reachable — the model
          ! says so, an empty last_message is a contact and not missing data. who it does NOT
          ! list is a public-profile stranger, because the query unions accepted follows with
          ! existing dm history and a stranger has neither. reaching them meant opening the
          ! directory or their profile first, which is the trip this removes
        */}
        <button className="btn btn--sm" onClick={() => setStarting((v) => !v)}>
          {starting ? 'close' : 'new conversation'}
        </button>
      </div>

      {starting && <StartConversation onClose={() => setStarting(false)} />}

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

/**
 * pick somebody to write to for the first time.
 * ? GET /api/users?q= is the only user search there is, so this is the directory's call rather
 * ? than a new endpoint. it searches names and nicknames and never email
 * ! it does not pre-filter to people you may message, and it must not pretend to: "at least one
 * ! of the two is following the other, or the recipient is public" is chat's rule and it lives
 * ! in chat's service. the thread answers 403 chat_not_allowed and the conversation page already
 * ! renders that as the red notice, which is the one honest place for it
 */
function StartConversation({ onClose }: { onClose: () => void }) {
  const router = useRouter()
  const [raw, setRaw] = useState('')
  //? debounced, or every keystroke is a request. the same 250ms the directory and the group
  //? invite picker use
  const q = useDebounced(raw, 250)
  const { data, loading } = useFetch<ListUser[]>(() => (q ? searchUsers(q, 8, 0) : Promise.resolve([])), [q])
  const found = data ?? []

  return (
    <div className="rows">
      <div className="row-item" style={{ padding: '13px 26px' }}>
        <label className="search" style={{ width: '100%', maxWidth: 360 }}>
          <span className="lbl" style={{ letterSpacing: '.1em' }}>
            to
          </span>
          <input
            autoFocus
            type="search"
            value={raw}
            onChange={(e) => setRaw(e.target.value)}
            placeholder="name or nickname"
            aria-label="Find somebody to message"
          />
        </label>
      </div>

      {found.map((u) => (
        <div className="row-item" key={u.id}>
          <button
            className="row-main"
            onClick={() => {
              onClose()
              router.push(`/chat/${u.id}`)
            }}
          >
            <span className={u.is_public ? 'dot dot--on' : 'dot dot--idle'} />
            <Avatar name={displayName(u)} path={u.avatar_path} size="sm" />
            <span className="tx">
              <span className="nm">{displayName(u)}</span>
              <span className="mt">{handle(u)} · {u.is_public ? 'public record' : 'private record'}</span>
            </span>
            <span className="caret" aria-hidden="true">
              ›
            </span>
          </button>
        </div>
      ))}

      {/*? gated on loading as well, or the empty state flashes between the keystroke and the
          response on every search that follows one that found nothing */}
      {q !== '' && !loading && found.length === 0 && <Empty label="no account matches" />}
    </div>
  )
}
