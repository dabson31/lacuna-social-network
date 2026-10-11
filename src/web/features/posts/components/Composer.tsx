'use client'

import { useEffect, useRef, useState } from 'react'
import { Privacy } from '@shared/constants/domain'
import type { Post } from '@shared/contracts/posts'
import type { UserCard } from '@shared/contracts/follows'
import { Avatar } from '@/components/ui/Avatar'
import { FieldError, FormError } from '@/components/ui/ErrorView'
import { useToast } from '@/components/ui/Toast'
import { fieldErrors } from '@/lib/api/errors'
import { LIMITS, runeLength } from '@/lib/utils/count'
import { displayName } from '@/lib/utils/identity'
import { submitOnEnter } from '@/lib/utils/keys'
import { listFollowers } from '@/features/follows/api'
import { ImagePicker } from '@/features/auth/components/ImagePicker'
import { createPost } from '../api'

/**
 * the design's .compose block: avatar, textarea, attachment stub, viewer chips, the privacy
 * segmented control, and transmit
 */
export function Composer({
  selfId,
  selfName,
  selfAvatar,
  onCreated,
}: {
  selfId: number
  selfName: string
  selfAvatar: string | null
  onCreated: (post: Post) => void
}) {
  const [content, setContent] = useState('')
  const [privacy, setPrivacy] = useState<Privacy>(Privacy.Public)
  const [chosen, setChosen] = useState<UserCard[]>([])
  const [candidates, setCandidates] = useState<UserCard[] | null>(null)
  const [picking, setPicking] = useState(false)
  const [error, setError] = useState<unknown>(null)
  const [busy, setBusy] = useState(false)
  const formRef = useRef<HTMLFormElement>(null)
  const toast = useToast()
  const fields = fieldErrors(error)

  /**
   * ! the chip picker is fed by the caller's FOLLOWERS and not the people they follow.
   * ! acceptedAmong queries `followee_id = author AND follower_id IN (ids)`, so a private post
   * ! names people who follow the author. getting this backwards produces a 403
   * ! forbidden_viewer that looks like a permissions bug when it is simply the wrong list
   */
  useEffect(() => {
    if (privacy !== Privacy.Private || candidates !== null) return
    void listFollowers(selfId)
      .then(setCandidates)
      .catch(() => setCandidates([]))
  }, [privacy, candidates, selfId])

  const submit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    //! the guard the disabled button used to be enough for. enter reaches the form directly,
    //! so a second one while the first is still in flight would post twice
    if (busy) return
    const formEl = e.currentTarget
    const body = new FormData(formEl)
    body.set('privacy', privacy)

    if (privacy === Privacy.Private) {
      //! one append per viewer. FormData.append with a repeated key is what produces the
      //! repeated form field the handler reads out of r.PostForm["viewer_ids"]. a single comma
      //! joined string parses as one bad int and silently selects nobody, which saves the post
      //! as private with zero viewers and no error anywhere
      for (const u of chosen) body.append('viewer_ids', String(u.id))
    }

    setBusy(true)
    setError(null)
    try {
      const created = await createPost(body)
      onCreated(created)
      setContent('')
      setChosen([])
      formEl.reset()
      toast(`transmitted · post ${created.id}`)
    } catch (err: unknown) {
      setError(err)
    } finally {
      setBusy(false)
    }
  }

  const unchosen = (candidates ?? []).filter((c) => !chosen.some((p) => p.id === c.id))

  return (
    <section className="compose" aria-label="New post">
      <FormError error={error} />
      <form ref={formRef} onSubmit={submit}>
        <div className="row">
          <Avatar name={selfName} path={selfAvatar} size="lg" />
          <div className="field">
            <textarea
              name="content"
              rows={2}
              placeholder="Transmit to the layer…"
              aria-label="Post body"
              maxLength={LIMITS.post}
              value={content}
              onChange={(e) => setContent(e.target.value)}
              onKeyDown={(e) => submitOnEnter(e, formRef.current)}
            />
            <FieldError fields={fields} name="content" />

            <div className="attached" data-open="1" style={{ marginTop: 8 }}>
              <ImagePicker name="image" label="attach image" />
            </div>
            <FieldError fields={fields} name="image" />

            {/*
              the viewer row only opens for a private post, which is the design showing the
              privacy model rather than describing it
            */}
            <div className="viewers" data-open={privacy === Privacy.Private ? '1' : '0'}>
              <span className="lbl">viewers</span>
              {chosen.map((u) => (
                <span className="chip" key={u.id}>
                  {displayName(u)}
                  <button
                    type="button"
                    aria-label={`Remove ${displayName(u)}`}
                    onClick={() => setChosen((prev) => prev.filter((p) => p.id !== u.id))}
                  >
                    ×
                  </button>
                </span>
              ))}
              <button type="button" className="chip chip--add" onClick={() => setPicking((v) => !v)}>
                + select
              </button>
            </div>

            {privacy === Privacy.Private && picking && (
              <div className="q" style={{ marginTop: 9, fontSize: '10.5px' }}>
                {candidates === null && 'reading your followers…'}
                {candidates !== null && unchosen.length === 0 && (
                  <>
                    &gt; nobody left to choose. a private post can only name accepted followers,
                    <br />
                    &gt; so somebody has to follow you first
                  </>
                )}
                {unchosen.map((u) => (
                  <button
                    key={u.id}
                    type="button"
                    className="chip chip--add"
                    style={{ margin: '2px 4px 2px 0' }}
                    onClick={() => {
                      setChosen((prev) => [...prev, u])
                      setPicking(false)
                    }}
                  >
                    {displayName(u)}
                  </button>
                ))}
              </div>
            )}

            <div className="under">
              <div className="seg" role="group" aria-label="Post privacy">
                {/*? the three values are already the api's three strings, so no mapping step */}
                <button type="button" aria-pressed={privacy === Privacy.Public} onClick={() => setPrivacy(Privacy.Public)}>
                  public
                </button>
                <button
                  type="button"
                  aria-pressed={privacy === Privacy.AlmostPrivate}
                  onClick={() => setPrivacy(Privacy.AlmostPrivate)}
                >
                  followers
                </button>
                <button
                  type="button"
                  aria-pressed={privacy === Privacy.Private}
                  onClick={() => setPrivacy(Privacy.Private)}
                >
                  selected
                </button>
              </div>
              <FieldError fields={fields} name="privacy" />
              <div className="spacer" />
              {/*? the gesture says so out loud, in the design's own .lbl, because enter now
                  sends and a reader who expects a newline would otherwise find out by
                  posting half a sentence. .under already wraps, so this costs no layout */}
              <span className="lbl">enter sends · shift+enter new line</span>
              <span className="lbl">
                {runeLength(content)} / {LIMITS.post}
              </span>
              <button className="btn btn--primary" type="submit" disabled={busy}>
                {busy ? 'writing…' : 'transmit'}
              </button>
            </div>
          </div>
        </div>
      </form>
    </section>
  )
}
