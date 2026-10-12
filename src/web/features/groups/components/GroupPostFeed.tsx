'use client'

import { useCallback, useEffect, useRef, useState } from 'react'
import { NavButton } from '@/components/ui/NavButton'
import type { GroupPost } from '@shared/contracts/groups'
import { Avatar } from '@/components/ui/Avatar'
import { Empty, Loading } from '@/components/ui/Empty'
import { ErrorView } from '@/components/ui/ErrorView'
import { useToast } from '@/components/ui/Toast'
import { useBadges } from '@/components/layout/BadgeProvider'
import { useFrame } from '@/lib/realtime/SocketProvider'
import { WsFrame } from '@shared/constants/ws'
import { LIMITS, runeLength } from '@/lib/utils/count'
import { submitOnEnter } from '@/lib/utils/keys'
import { age } from '@/lib/utils/time'
import { Attachment } from '@/features/posts/components/Attachment'
import { Thread } from '@/features/posts/components/Thread'
import { ImagePicker } from '@/features/auth/components/ImagePicker'
import { EmojiPicker } from '@/components/ui/EmojiPicker'
import { createGroupComment, createGroupPost, listGroupComments, listGroupPosts, markGroupPostsRead } from '../api'

/**
 * group posts, members only.
 * ? GroupPost has the same fields as Post minus privacy, because membership IS the privacy
 * ? here — there is nothing per-post to decide. so the card is the design's .post block
 * ? without the tag and the edge mark
 * ? neither the posts nor their comments are paginated, both endpoints return whole lists
 */
export function GroupPostFeed({
  groupId,
  selfName,
  selfAvatar,
}: {
  groupId: number
  selfName: string
  selfAvatar: string | null
}) {
  const [posts, setPosts] = useState<GroupPost[] | null>(null)
  const [error, setError] = useState<unknown>(null)
  const [open, setOpen] = useState<number | null>(null)
  const [content, setContent] = useState('')
  const [busy, setBusy] = useState(false)
  const { reloadGroups } = useBadges()
  //? enter submits through the form rather than through a click, so the handler needs a
  //? handle on it. same shape as the feed composer, which is the point
  const formRef = useRef<HTMLFormElement>(null)
  const bodyRef = useRef<HTMLTextAreaElement>(null)
  const toast = useToast()

  /**
   * clearing the number, which is a separate thing from drawing the posts.
   * ! it has to run again for every post that arrives while this list is on screen, or the
   * ! count climbs for a group the reader is looking straight at — the watermark only moves
   * ! when something tells it to
   */
  const markRead = useCallback(() => {
    markGroupPostsRead(groupId)
      .then(reloadGroups)
      .catch(() => {
        /* the posts are read either way, the number simply clears on the next load */
      })
  }, [groupId, reloadGroups])

  const load = () => {
    listGroupPosts(groupId)
      .then((list) => {
        setPosts(list)
        /**
         * ! read means on screen, so the watermark moves here and not when the group page
         * ! mounts: a non-member never gets this far, and neither does anyone who opened the
         * ! events or members tab instead
         * ? the group list is reloaded after it, because the number this just cleared is
         * ? drawn from Group.unread_posts and the shell is still holding the old one
         */
        markRead()
      })
      .catch(setError)
  }

  useEffect(load, [groupId])

  /**
   * a post from somebody else, arriving while this list is open.
   * ? deduped by id because the author's own 201 already put their post in the list and the
   * ? socket pushes the same row to them as well, by design — the same rule the chat thread
   * ? uses for its own messages
   */
  useFrame(
    WsFrame.GroupPost,
    (p) => {
      if (p.group_id !== groupId) return
      setPosts((prev) => (prev === null ? [p] : prev.some((x) => x.id === p.id) ? prev : [p, ...prev]))
      markRead()
    },
    [groupId, markRead],
  )

  const submit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    if (busy) return
    const formEl = e.currentTarget
    const body = new FormData(formEl)
    if (runeLength(content.trim()) === 0 && !(body.get('image') as File | null)?.size) return

    setBusy(true)
    try {
      const created = await createGroupPost(groupId, body)
      //? the socket pushes this same row back to the author, so the frame handler above
      //? dedupes it by id. prepending here as well is what makes it appear without waiting
      //? for the round trip through the hub
      setPosts((prev) => (prev ?? []).some((x) => x.id === created.id) ? (prev ?? []) : [created, ...(prev ?? [])])
      setContent('')
      formEl.reset()
      toast('posted to the group')
    } catch (err: unknown) {
      setError(err)
    } finally {
      setBusy(false)
    }
  }

  if (error && posts === null) return <ErrorView error={error} onRetry={load} />

  return (
    <>
      <section className="compose" aria-label="New group post">
        <form ref={formRef} onSubmit={submit}>
          <div className="row">
            <Avatar name={selfName} path={selfAvatar} size="lg" />
            <div className="field">
              <textarea
                ref={bodyRef}
                name="content"
                rows={2}
                placeholder="Post to this group…"
                aria-label="Group post body"
                maxLength={LIMITS.post}
                value={content}
                onChange={(e) => setContent(e.target.value)}
                onKeyDown={(e) => submitOnEnter(e, formRef.current)}
              />
              <div className="attached" data-open="1" style={{ marginTop: 8 }}>
                <ImagePicker name="image" label="attach image" />
                <EmojiPicker target={bodyRef} onInsert={setContent} />
              </div>
              <div className="under">
                <span className="lbl">visible to members only</span>
                <div className="spacer" />
                <span className="lbl">enter sends · shift+enter new line</span>
                <span className="lbl">
                  {runeLength(content)} / {LIMITS.post}
                </span>
                <button className="btn btn--primary" type="submit" disabled={busy}>
                  {busy ? 'writing…' : 'post'}
                </button>
              </div>
            </div>
          </div>
        </form>
      </section>

      <div className="feed">
        {posts === null && <Loading />}

        {(posts ?? []).map((p) => (
          <article className="post" key={p.id} data-id={p.id}>
            <div className="post__hd">
              <Avatar name={p.author_name} path={p.author_avatar} />
              <div className="post__who">
                <div className="nm">
                  <NavButton href={`/profile/${p.author_id}`}>{p.author_name}</NavButton>
                </div>
                <div className="mt">
                  <span>post {p.id}</span>
                  <span>·</span>
                  <span>{age(p.created_at)}</span>
                </div>
              </div>
              <div className="spacer" />
              <span className="tag tag--semi">members</span>
            </div>

            <div className="post__body">
              {p.content && <p>{p.content}</p>}
              {p.image_path && <Attachment path={p.image_path} />}
            </div>

            <div className="post__ft">
              <button
                className="act"
                aria-expanded={open === p.id}
                aria-controls={`gthread-${p.id}`}
                onClick={() => setOpen(open === p.id ? null : p.id)}
              >
                reply <b>{p.comment_count}</b>
              </button>
            </div>

            <div className="thread" id={`gthread-${p.id}`} hidden={open !== p.id}>
              {open === p.id && (
                <Thread
                  postId={p.id}
                  selfName={selfName}
                  selfAvatar={selfAvatar}
                  /*
                    ? the group comment shapes match posts.Comment field for field apart from
                    ? the post key, so the design's thread component serves both. mapped here
                    ? rather than widening the component's type
                  */
                  load={async () =>
                    (await listGroupComments(groupId, p.id)).map((c) => ({ ...c, post_id: c.group_post_id }))
                  }
                  submit={async (body) => {
                    const c = await createGroupComment(groupId, p.id, body)
                    return { ...c, post_id: c.group_post_id }
                  }}
                />
              )}
            </div>
          </article>
        ))}

        {posts !== null && posts.length === 0 && <Empty label="nothing posted in this group yet" />}
      </div>
    </>
  )
}
