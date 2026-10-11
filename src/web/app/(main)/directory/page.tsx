'use client'

import { useState } from 'react'
import Link from 'next/link'
import type { ListUser } from '@shared/contracts/users'
import type { UserCard } from '@shared/contracts/follows'
import { FollowStatus } from '@shared/constants/domain'
import { Avatar } from '@/components/ui/Avatar'
import { Empty, Loading } from '@/components/ui/Empty'
import { ErrorView } from '@/components/ui/ErrorView'
import { useToast } from '@/components/ui/Toast'
import { useDebounced } from '@/hooks/useDebounced'
import { useFetch } from '@/lib/api/useFetch'
import { useUser } from '@/lib/session/SessionProvider'
import { displayName, handle } from '@/lib/utils/identity'
import { searchUsers } from '@/features/profile/api'
import { FollowButton } from '@/features/follows/components/FollowButton'
import { listFollowers, listFollowing } from '@/features/follows/api'

const PAGE = 20

/**
 * every account this node has a record of, ported from viewDirectory at line 3760.
 * ? offset paging here rather than a cursor: the list is ordered by name and does not grow at
 * ? the top, so an offset is stable in a way it is not for the feed
 */
export default function DirectoryPage() {
  const me = useUser()
  const toast = useToast()
  const [raw, setRaw] = useState('')
  const [offset, setOffset] = useState(0)
  const [open, setOpen] = useState<number | null>(null)
  //! debounced, or every keystroke is a request. the mockup had to rebuild its list and then
  //! restore the caret by hand on each one, which react makes unnecessary, but the throttling
  //! is still needed
  const q = useDebounced(raw, 250)

  const { data, loading, error, refetch } = useFetch(() => searchUsers(q, PAGE, offset), [q, offset])

  /**
   * ! ListUser carries no relationship field — id, names, nickname, avatar, is_public and
   * ! nothing else. so the design's four state relation tag cannot come from the browse
   * ! response. these two lists are fetched once for the whole page and intersected, which is
   * ! two calls rather than one per row, and they are the same lists the composer's viewer
   * ! picker needs anyway
   */
  const followers = useFetch<UserCard[]>(() => listFollowers(me.id), [me.id])
  const following = useFetch<UserCard[]>(() => listFollowing(me.id), [me.id])

  const relationOf = (id: number): { label: string; status: FollowStatus } => {
    const iFollow = (following.data ?? []).some((u) => u.id === id)
    const followsMe = (followers.data ?? []).some((u) => u.id === id)
    if (iFollow && followsMe) return { label: 'mutual', status: FollowStatus.Accepted }
    if (iFollow) return { label: 'following', status: FollowStatus.Accepted }
    if (followsMe) return { label: 'follows you', status: FollowStatus.None }
    //? a pending request is not visible in either list, so it reads as no relation until the
    //? follow button's own response says otherwise
    return { label: 'no relation', status: FollowStatus.None }
  }

  const users: ListUser[] = data ?? []

  return (
    <>
      <div className="main__bar">
        <span className="lbl">
          {users.length} {users.length === 1 ? 'account' : 'accounts'}
          {offset > 0 ? ` · from ${offset + 1}` : ''}
        </span>
        <div className="spacer" />
        <label className="search">
          <span className="lbl" style={{ letterSpacing: '.1em' }}>
            find
          </span>
          <input
            type="search"
            value={raw}
            onChange={(e) => {
              setRaw(e.target.value)
              setOffset(0)
            }}
            placeholder="name or nickname"
            aria-label="Search accounts"
          />
          <span className="k">/</span>
        </label>
      </div>

      {error && <ErrorView error={error} onRetry={refetch} />}
      {loading && <Loading label="reading the directory…" />}

      {!loading && (
        <div className="rows">
          {users.map((u) => {
            const rel = relationOf(u.id)
            const isOpen = open === u.id
            return (
              <div className="row-item" key={u.id}>
                <button
                  className="row-main"
                  aria-expanded={isOpen}
                  aria-controls={`dir-${u.id}`}
                  onClick={() => setOpen(isOpen ? null : u.id)}
                >
                  <span className={u.is_public ? 'dot dot--on' : 'dot dot--idle'} />
                  <Avatar name={displayName(u)} path={u.avatar_path} size="sm" />
                  <span className="tx">
                    <span className="nm">
                      {displayName(u)}
                      <span className="tag">{rel.label}</span>
                    </span>
                    <span className="mt">
                      {handle(u)} · {u.is_public ? 'public record' : 'private record'}
                    </span>
                  </span>
                  <span className="caret" aria-hidden="true">
                    ›
                  </span>
                </button>

                <div className="row-detail" id={`dir-${u.id}`} hidden={!isOpen}>
                  <div className="pad">
                    <p className="note-txt">
                      {u.is_public
                        ? 'A public record. You can read it and follow without asking.'
                        : 'A private record. Following sends a request they have to accept.'}
                    </p>
                    <div className="row-ops">
                      <FollowButton
                        userId={u.id}
                        status={rel.status}
                        onChange={() => {
                          followers.refetch()
                          following.refetch()
                        }}
                      />
                      {/*
                        ? this is the entry point for messaging a public-profile stranger. they
                        ? are not in /api/conversations until a first message exists, because
                        ? that query unions accepted follows with existing dm history
                      */}
                      <Link className="btn btn--sm" href={`/chat/${u.id}`}>
                        message
                      </Link>
                      <Link className="btn btn--sm" href={`/profile/${u.id}`}>
                        open record
                      </Link>
                      <button
                        className="btn btn--sm"
                        onClick={() => {
                          void navigator.clipboard?.writeText(handle(u))
                          toast(`copied · ${handle(u)}`)
                        }}
                      >
                        copy handle
                      </button>
                    </div>
                  </div>
                </div>
              </div>
            )
          })}

          {users.length === 0 && <Empty label="no account matches" />}
        </div>
      )}

      {!loading && (offset > 0 || users.length === PAGE) && (
        <div className="main__bar">
          <button className="btn btn--sm" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - PAGE))}>
            ← previous
          </button>
          <div className="spacer" />
          <button className="btn btn--sm" disabled={users.length < PAGE} onClick={() => setOffset(offset + PAGE)}>
            next →
          </button>
        </div>
      )}
    </>
  )
}
