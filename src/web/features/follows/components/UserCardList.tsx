'use client'

import Link from 'next/link'
import type { UserCard } from '@shared/contracts/follows'
import { Avatar } from '@/components/ui/Avatar'
import { Empty } from '@/components/ui/Empty'
import { displayName, handle } from '@/lib/utils/identity'

/** the design's .rows list, reused for a followers or following list */
export function UserCardList({ users, empty }: { users: UserCard[]; empty: string }) {
  if (users.length === 0) return <Empty label={empty} />
  return (
    <div className="rows">
      {users.map((u) => (
        <div className="row-item" key={u.id}>
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
            <span className="caret" aria-hidden="true">
              ›
            </span>
          </Link>
        </div>
      ))}
    </div>
  )
}
