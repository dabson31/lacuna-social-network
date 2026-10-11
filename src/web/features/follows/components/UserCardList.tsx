'use client'

import type { ReactNode } from 'react'
import Link from 'next/link'
import type { UserCard } from '@shared/contracts/follows'
import { Avatar } from '@/components/ui/Avatar'
import { Empty } from '@/components/ui/Empty'
import { displayName, handle } from '@/lib/utils/identity'

/**
 * the design's .rows list, reused for a followers or following list.
 * ? `action` is a render prop and not a status plus a callback, because UserCard carries no
 * ? relationship field — id, names, nickname, avatar, is_public and nothing else. whoever draws
 * ? the list is the only one who can know how the viewer relates to each row, so the button is
 * ? built there and this component stays layout
 */
export function UserCardList({
  users,
  empty,
  action,
}: {
  users: UserCard[]
  empty: string
  action?: (u: UserCard) => ReactNode
}) {
  if (users.length === 0) return <Empty label={empty} />
  return (
    <div className="rows">
      {users.map((u) => {
        const ops = action?.(u)
        return (
          //? .row-item--act is the one class here the design does not define, and it only lays
          //? the row and its action out side by side. additions.css says what and why
          <div className={ops ? 'row-item row-item--act' : 'row-item'} key={u.id}>
            <Link className="row-main" href={`/profile/${u.id}`}>
              <span className={u.is_public ? 'dot dot--on' : 'dot dot--idle'} />
              <Avatar name={displayName(u)} path={u.avatar_path} size="sm" />
              <span className="tx">
                <span className="nm">
                  {displayName(u)}
                  <span className="tag">{u.is_public ? 'public' : 'private'}</span>
                </span>
                <span className="mt">{handle(u)}</span>
              </span>
              {/*? the caret belongs to the row that opens the profile, so it stays inside the
                  link and the action sits outside it — a button inside an anchor is not
                  something a browser will lay out, let alone announce */}
              <span className="caret" aria-hidden="true">
                ›
              </span>
            </Link>
            {ops && <div className="row-ops">{ops}</div>}
          </div>
        )
      })}
    </div>
  )
}
