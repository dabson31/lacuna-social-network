'use client'

import { useMemo, useState } from 'react'
import { useRouter } from 'next/navigation'
import type { Post } from '@shared/contracts/posts'
import { Empty, Loading } from '@/components/ui/Empty'
import { ErrorView } from '@/components/ui/ErrorView'
import { useUser } from '@/lib/session/SessionProvider'
import { Composer } from '@/features/posts/components/Composer'
import { PostCard } from '@/features/posts/components/PostCard'
import { useCursorList } from '@/features/posts/hooks/useCursorList'
import { createComment, deletePost, listComments, listFeed } from '@/features/posts/api'

/**
 * the four scopes from the mockup's SCOPES.
 * ! GET /api/posts takes before and limit and nothing else, so these filter what is loaded
 * ! rather than what is queried. three of the four are honest about the data:
 * !   all        no filter
 * !   mine       author_id === me, exact
 * !   following  everything that is not mine, which is nearly right — a public post from a
 * !              stranger also lands in the feed and this will call it "following"
 * !   groups     always empty, and that is not a bug. group posts live in group_posts, a
 * !              separate table that /api/posts never touches, so the design's feed and a
 * !              group's feed are two different lists on the server
 * ? so the groups tab links out to /groups instead of filtering to nothing
 */
const SCOPES = [
  ['all', 'all'],
  ['following', 'following'],
  ['mine', 'mine'],
] as const

type Scope = (typeof SCOPES)[number][0]

export default function FeedPage() {
  const me = useUser()
  const router = useRouter()
  const [scope, setScope] = useState<Scope>('all')
  const [query, setQuery] = useState('')

  const feed = useCursorList<Post>((before, limit) => listFeed(before, limit), 20, [])

  const visible = useMemo(() => {
    const q = query.trim().toLowerCase()
    return feed.items.filter((p) => {
      if (scope === 'mine' && p.author_id !== me.id) return false
      if (scope === 'following' && p.author_id === me.id) return false
      if (!q) return true
      return (p.content + ' ' + p.author_name).toLowerCase().includes(q)
    })
  }, [feed.items, scope, query, me.id])

  return (
    <>
      <div className="main__bar">
        <div className="seg" role="tablist" aria-label="Feed scope">
          {SCOPES.map(([v, label]) => (
            <button
              key={v}
              type="button"
              role="tab"
              aria-selected={scope === v}
              onClick={() => setScope(v)}
            >
              {label}
            </button>
          ))}
          {/*
            ! a button, because `.seg button` is what the stylesheet styles. the groups scope
            ! navigates instead of filtering, since group posts live in a separate table that
            ! /api/posts never touches, so filtering for them would always show nothing
          */}
          <button type="button" role="tab" aria-selected={false} onClick={() => router.push('/groups')}>
            groups
          </button>
        </div>

        {/*
          the mockup's own honesty, and it matters more with pagination rather than less: a
          filter applies to what is loaded, not to the whole feed
        */}
        <span className="lbl">
          {visible.length} {visible.length === 1 ? 'entry' : 'entries'}
          {visible.length === feed.items.length ? '' : ` · filtered from ${feed.items.length}`}
        </span>
        <div className="spacer" />
        <label className="search">
          <span className="lbl" style={{ letterSpacing: '.1em' }}>
            find
          </span>
          <input
            type="search"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="user or text"
            aria-label="Filter the loaded feed"
          />
          <span className="k">/</span>
        </label>
      </div>

      <Composer
        selfId={me.id}
        selfName={me.nickname ?? `${me.first_name} ${me.last_name}`}
        selfAvatar={me.avatar_path}
        onCreated={feed.prepend}
      />

      {feed.error && <ErrorView error={feed.error} onRetry={feed.reload} />}

      <div className="feed">
        {feed.loading && <Loading label="reading the layer…" />}

        {!feed.loading &&
          visible.map((p) => (
            <PostCard
              key={p.id}
              post={p}
              selfId={me.id}
              selfName={me.nickname ?? `${me.first_name} ${me.last_name}`}
              selfAvatar={me.avatar_path}
              loadComments={listComments}
              submitComment={createComment}
              onDelete={async (id) => {
                await deletePost(id)
                feed.remove(id)
              }}
            />
          ))}

        {!feed.loading && visible.length === 0 && <Empty label="no entries match this filter" />}

        {/* the design's .act.more for another page, and .eof once the api runs short */}
        {!feed.loading && !feed.exhausted && (
          <button className="act more" onClick={feed.loadMore} disabled={feed.loadingMore}>
            {feed.loadingMore ? 'reading…' : 'load older entries'}
          </button>
        )}
        {!feed.loading && feed.exhausted && feed.items.length > 0 && (
          <div className="eof">
            <div className="ln" />
            <span className="lbl">end of what this node holds for you</span>
          </div>
        )}
      </div>
    </>
  )
}
