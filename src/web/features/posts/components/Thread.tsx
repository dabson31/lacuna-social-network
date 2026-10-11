'use client'

import { useEffect, useState } from 'react'
import type { Comment } from '@shared/contracts/posts'
import { Avatar } from '@/components/ui/Avatar'
import { ErrorView } from '@/components/ui/ErrorView'
import { useToast } from '@/components/ui/Toast'
import { LIMITS, runeLength } from '@/lib/utils/count'
import { age } from '@/lib/utils/time'
import { ImagePicker } from '@/features/auth/components/ImagePicker'
import { Attachment } from './Attachment'

const PAGE = 3

/**
 * the comment thread, ported from the .thread region of postHTML.
 * ! the comments endpoint has no limit or offset, it hands back the whole list. so the design's
 * ! "load N more" is a reveal and not a fetch — hold them all, show a slice, grow the slice
 * ? the fetch happens when the thread is first expanded and not with the feed, because twenty
 * ? cards would otherwise be twenty-one requests on load
 */
export function Thread({
  postId,
  selfName,
  selfAvatar,
  load,
  submit,
  onCountChange,
}: {
  postId: number
  selfName: string
  selfAvatar: string | null
  load: () => Promise<Comment[]>
  submit: (body: FormData) => Promise<Comment>
  onCountChange?: (n: number) => void
}) {
  const [comments, setComments] = useState<Comment[] | null>(null)
  const [shown, setShown] = useState(PAGE)
  const [error, setError] = useState<unknown>(null)
  const [draft, setDraft] = useState('')
  const [busy, setBusy] = useState(false)
  const toast = useToast()

  useEffect(() => {
    let live = true
    load()
      .then((list) => {
        if (!live) return
        setComments(list)
        //? the newest are at the bottom, so reveal from the end: a thread opens showing the
        //? most recent replies with the older ones behind the button
        setShown(Math.min(PAGE, list.length) || PAGE)
      })
      .catch((e: unknown) => live && setError(e))
    return () => {
      live = false
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [postId])

  const send = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    const formEl = e.currentTarget
    const body = new FormData(formEl)
    //? the service accepts text or an image, so an empty draft with a file attached is valid
    if (runeLength(draft.trim()) === 0 && !(body.get('image') as File | null)?.size) return

    setBusy(true)
    try {
      const created = await submit(body)
      setComments((prev) => [...(prev ?? []), created])
      setShown((n) => n + 1)
      onCountChange?.((comments?.length ?? 0) + 1)
      setDraft('')
      formEl.reset()
      toast(`reply stored · post ${postId}`)
    } catch (err: unknown) {
      setError(err)
    } finally {
      setBusy(false)
    }
  }

  if (error && comments === null) return <ErrorView error={error} inline />

  const all = comments ?? []
  const visible = all.slice(Math.max(0, all.length - shown))
  const more = all.length - visible.length

  return (
    <>
      {more > 0 && (
        <>
          {/*
            ! the label says what the click does. it used to read "load N more" where N was
            ! every remaining reply, while the click revealed three — so on a post with twenty
            ! replies the button said "load 17 more" and then said "load 14 more"
          */}
          <button className="act more" onClick={() => setShown((n) => n + PAGE)}>
            load {Math.min(PAGE, more)} more
          </button>
          {/*
            ? and the way out of clicking it six times. the comments endpoint has no limit or
            ? offset — it already handed back every reply — so this is a reveal and not a
            ? fetch, which is why "all" costs nothing and needs no new call
          */}
          {more > PAGE && (
            <button className="act more" onClick={() => setShown(all.length)}>
              load all {more}
            </button>
          )}
        </>
      )}

      {visible.map((c) => (
        <div className="c" key={c.id}>
          <Avatar name={c.author_name} path={c.author_avatar} size="sm" />
          <div className="tx">
            <div className="h">
              {c.author_name} · {age(c.created_at)}
            </div>
            <p>{c.content}</p>
            {c.image_path && <Attachment path={c.image_path} />}
          </div>
        </div>
      ))}

      {comments === null && <p className="note-txt">reading replies…</p>}

      <form className="reply-box" onSubmit={send}>
        <Avatar name={selfName} path={selfAvatar} size="sm" />
        <input
          type="text"
          name="content"
          placeholder={`reply to post ${postId}`}
          aria-label={`Reply to post ${postId}`}
          maxLength={LIMITS.comment}
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
        />
        <ImagePicker name="image" label="image" />
        <button className="btn btn--sm" type="submit" disabled={busy}>
          send
        </button>
      </form>
    </>
  )
}
