'use client'

import Link from 'next/link'
import { useParams, useRouter } from 'next/navigation'
import { Empty, Loading } from '@/components/ui/Empty'
import { ErrorView } from '@/components/ui/ErrorView'
import { useFetch } from '@/lib/api/useFetch'
import { useUser } from '@/lib/session/SessionProvider'
import { displayName } from '@/lib/utils/identity'
import { eventWhen } from '@/lib/utils/time'
import { PostCard } from '@/features/posts/components/PostCard'
import { createComment, deletePost, getPost, listComments } from '@/features/posts/api'

/**
 * one post, on its own page, with its replies open.
 * ! GET /api/posts/{id} has existed since phase 5 and nothing called it: the feed card's only
 * ! link out was the author's name, so the one thing a reader could not open from the feed was
 * ! the post. getPost was already written and sitting unused in features/posts/api.ts
 *
 * ! a 404 here means not found OR not visible to you, which is the documented behaviour of
 * ! every read in this api and the reason the copy never says "deleted". the visibility
 * ! predicate is the feed's own, run in sql, so a private post somebody shares the id of
 * ! answers exactly as if it had never existed
 * ? the whole page is the card plus a bar. the card already holds the thread, the privacy tag,
 * ? the withdraw action and the reply box, and a second renderer for the same post would be
 * ? two things to keep in step for nothing
 */
export default function PostPage() {
  const params = useParams<{ postId: string }>()
  const postId = Number(params.postId)
  const me = useUser()
  const router = useRouter()

  const { data: post, loading, error, refetch } = useFetch(() => getPost(postId), [postId])

  if (Number.isNaN(postId)) return <Empty label="not available · that is not a post id" />
  if (loading) return <Loading label="reading the entry…" />
  if (error) return <ErrorView error={error} onRetry={refetch} />
  if (!post) return <Empty label="not available" />

  return (
    <>
      <div className="main__bar">
        <Link className="btn btn--sm" href="/feed">
          ← feed
        </Link>
        <span className="lbl">post {post.id}</span>
        {/*
          ? the card's own line says "8 min", which is the right answer in a feed you are
          ? scrolling and the wrong one on the page you opened to read the thing properly.
          ? eventWhen is the design's existing "19 oct, 23:30" — borrowed from the event card
          ? rather than inventing a fourth time format for one line
        */}
        <span className="lbl">{eventWhen(post.created_at)}</span>
        <div className="spacer" />
        <Link className="btn btn--sm" href={`/profile/${post.author_id}`}>
          {post.author_name}
        </Link>
      </div>

      <div className="feed">
        <PostCard
          post={post}
          selfId={me.id}
          selfName={displayName(me)}
          selfAvatar={me.avatar_path}
          loadComments={listComments}
          submitComment={createComment}
          /*
            ? withdrawing the thing this page is about leaves nothing to render, so it goes
            ? back to the feed rather than sitting on a card that no longer exists
          */
          onDelete={async (id) => {
            await deletePost(id)
            router.replace('/feed')
          }}
          standalone
        />
      </div>
    </>
  )
}
