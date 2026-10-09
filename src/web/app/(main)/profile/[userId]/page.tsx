'use client'

import { useParams, useRouter } from 'next/navigation'
import { useState } from 'react'
import { FollowStatus } from '@shared/constants/domain'
import type { Post } from '@shared/contracts/posts'
import type { UserCard } from '@shared/contracts/follows'
import { Avatar } from '@/components/ui/Avatar'
import { Empty, Loading } from '@/components/ui/Empty'
import { ErrorView } from '@/components/ui/ErrorView'
import { useToast } from '@/components/ui/Toast'
import { useFetch } from '@/lib/api/useFetch'
import { useSession, useUser } from '@/lib/session/SessionProvider'
import { dateOnly } from '@/lib/utils/time'
import { displayName, handle } from '@/lib/utils/identity'
import { getProfile, setPrivacy } from '@/features/profile/api'
import { ProfileEditor } from '@/features/profile/components/ProfileEditor'
import { FollowButton } from '@/features/follows/components/FollowButton'
import { UserCardList } from '@/features/follows/components/UserCardList'
import { listFollowers, listFollowing } from '@/features/follows/api'
import { PostCard } from '@/features/posts/components/PostCard'
import { useCursorList } from '@/features/posts/hooks/useCursorList'
import { createComment, deletePost, listByAuthor, listComments } from '@/features/posts/api'

/**
 * one route for every profile.
 * ! is_self, can_view and follow_status are the three fields that decide the whole render, and
 * ! they arrive as data rather than as a rule the client re-derives. never reconstruct can_view
 * ! from is_public and follow_status: those two can both be true in ways that do not mean what
 * ! you would assume
 */
export default function ProfilePage() {
  const params = useParams<{ userId: string }>()
  const me = useUser()
  const router = useRouter()
  const toast = useToast()
  const { refresh } = useSession()

  const targetId = Number(params.userId)
  const [editing, setEditing] = useState(false)
  const [graph, setGraph] = useState<'none' | 'followers' | 'following'>('none')

  const { data: profile, loading, error, refetch } = useFetch(() => getProfile(targetId), [targetId])
  const posts = useCursorList<Post>((before, limit) => listByAuthor(targetId, before, limit), 20, [targetId])
  const lists = useFetch<UserCard[]>(
    () => (graph === 'followers' ? listFollowers(targetId) : graph === 'following' ? listFollowing(targetId) : Promise.resolve([])),
    [graph, targetId],
  )

  if (Number.isNaN(targetId)) return <Empty label="not available · that is not a user id" />
  if (loading) return <Loading label="reading record…" />
  if (error) return <ErrorView error={error} onRetry={refetch} />
  if (!profile) return <Empty label="not available" />

  const name = displayName(profile)

  const togglePrivacy = async (next: boolean) => {
    try {
      await setPrivacy(next)
      toast(next ? 'profile is public' : 'profile is private')
      //! refetch rather than patching locally: flipping to public does not retroactively
      //! accept pending requests and flipping to private does not drop existing followers, so
      //! the server's counts are the only correct ones
      refetch()
      await refresh()
    } catch {
      toast('that did not save')
    }
  }

  return (
    <>
      <div className="prof">
        <Avatar name={name} path={profile.avatar_path} size="lg" />
        <div className="meta">
          <h2>{name}</h2>
          <div className="h">
            {handle(profile)} · node lcn·7f3a · {profile.is_public ? 'public record' : 'private record'}
          </div>

          {/*
            ! can_view false means the server withheld the private half, so these four are null.
            ! that is the api refusing to leak rather than the client choosing to hide
          */}
          {profile.can_view ? (
            <>
              {profile.about_me && <p>{profile.about_me}</p>}
              {/*
                ? .kv is a definition list in the stylesheet, dt and dd, not divs with .k and
                ? .v. worth checking the css rather than guessing from the class name, which is
                ? the whole reason the "no class the stylesheet does not define" rule exists
              */}
              <dl className="kv" style={{ marginTop: 11 }}>
                {profile.email && (
                  <>
                    <dt>email</dt>
                    <dd>{profile.email}</dd>
                  </>
                )}
                {profile.date_of_birth && (
                  <>
                    <dt>born</dt>
                    <dd>{dateOnly(profile.date_of_birth)}</dd>
                  </>
                )}
                {profile.created_at && (
                  <>
                    <dt>joined</dt>
                    <dd>{dateOnly(profile.created_at)}</dd>
                  </>
                )}
              </dl>
            </>
          ) : (
            <p className="note-txt">
              This record is private. Follow and wait to be accepted, and the rest of it opens up.
            </p>
          )}

          <div className="stats">
            <button type="button" onClick={() => setGraph('none')}>
              <b>{posts.items.length}</b>
              <span>posts</span>
            </button>
            <button type="button" onClick={() => setGraph(graph === 'followers' ? 'none' : 'followers')}>
              <b>{profile.follower_count}</b>
              <span>followers</span>
            </button>
            <button type="button" onClick={() => setGraph(graph === 'following' ? 'none' : 'following')}>
              <b>{profile.following_count}</b>
              <span>following</span>
            </button>
            {/*? a button like its three siblings, because `.prof .stats button` is the rule */}
            {!profile.is_self && (
              <button type="button" onClick={() => router.push(`/chat/${profile.id}`)}>
                <b>·</b>
                <span>message</span>
              </button>
            )}
          </div>
        </div>

        <div className="side">
          {profile.is_self ? (
            <>
              <span className="lbl" style={{ display: 'block', marginBottom: 8 }}>
                Profile visibility
              </span>
              <div className="seg" role="group" aria-label="Profile visibility">
                <button type="button" aria-pressed={profile.is_public} onClick={() => void togglePrivacy(true)}>
                  public
                </button>
                <button type="button" aria-pressed={!profile.is_public} onClick={() => void togglePrivacy(false)}>
                  private
                </button>
              </div>
              {/* the mockup's own copy for each state, kept verbatim */}
              <p className="note-txt" style={{ marginTop: 10 }}>
                {profile.is_public
                  ? 'Anyone on this node can read your profile and follow without asking.'
                  : 'Only accepted followers can read your profile. New followers arrive as requests.'}
              </p>
              <button className="btn btn--sm" style={{ marginTop: 12 }} onClick={() => setEditing((v) => !v)}>
                {editing ? 'close editor' : 'edit record'}
              </button>
            </>
          ) : (
            <>
              <span className="lbl" style={{ display: 'block', marginBottom: 8 }}>
                Relation
              </span>
              <FollowButton
                userId={profile.id}
                status={profile.follow_status}
                onChange={() => refetch()}
                small={false}
              />
              <p className="note-txt" style={{ marginTop: 10 }}>
                {profile.follow_status === FollowStatus.Accepted
                  ? 'You follow this account, so you can read what they share with followers.'
                  : profile.follow_status === FollowStatus.Pending
                    ? 'Waiting on them. The request is sitting in their notices.'
                    : profile.is_public
                      ? 'A public account. Following takes effect immediately, no request needed.'
                      : 'A private account. Following sends a request they have to accept.'}
              </p>
            </>
          )}
        </div>
      </div>

      {editing && profile.is_self && (
        <div style={{ padding: '0 26px 18px' }}>
          <ProfileEditor
            profile={profile}
            onSaved={() => {
              setEditing(false)
              refetch()
              void refresh()
            }}
          />
        </div>
      )}

      {graph !== 'none' && (
        <>
          <div className="main__bar">
            <span className="lbl">{graph}</span>
            <div className="spacer" />
            <button className="btn btn--sm" onClick={() => setGraph('none')}>
              close
            </button>
          </div>
          {lists.loading ? (
            <Loading />
          ) : lists.error ? (
            <ErrorView error={lists.error} onRetry={lists.refetch} />
          ) : (
            <UserCardList users={lists.data ?? []} empty={`no ${graph} to show`} />
          )}
        </>
      )}

      <div className="main__bar">
        <span className="lbl">{profile.is_self ? 'Published by you' : `Published by ${name}`}</span>
        <div className="spacer" />
        {/*
          ? not presented as a total. the same visibility predicate as the feed applies here, so
          ? a stranger on a public profile sees only the public posts and this count will not
          ? match what the owner sees
        */}
        <span className="lbl">{posts.items.length} loaded</span>
      </div>

      <div className="feed">
        {posts.loading && <Loading />}
        {posts.error && <ErrorView error={posts.error} onRetry={posts.reload} />}

        {!posts.loading &&
          posts.items.map((p) => (
            <PostCard
              key={p.id}
              post={p}
              selfId={me.id}
              selfName={displayName(me)}
              selfAvatar={me.avatar_path}
              loadComments={listComments}
              submitComment={createComment}
              onDelete={async (id) => {
                await deletePost(id)
                posts.remove(id)
              }}
            />
          ))}

        {!posts.loading && posts.items.length === 0 && (
          <Empty label={profile.is_self ? 'nothing published yet' : 'nothing here you are allowed to read'} />
        )}

        {!posts.loading && !posts.exhausted && posts.items.length > 0 && (
          <button className="act more" onClick={posts.loadMore} disabled={posts.loadingMore}>
            {posts.loadingMore ? 'reading…' : 'load older entries'}
          </button>
        )}
        {!posts.loading && posts.exhausted && posts.items.length > 0 && (
          <div className="eof">
            <div className="ln" />
            <span className="lbl">end of this record</span>
          </div>
        )}
      </div>
    </>
  )
}
