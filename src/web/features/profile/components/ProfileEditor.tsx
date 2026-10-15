'use client'

import { useRef, useState } from 'react'
import type { Profile, UpdateProfileInput } from '@shared/contracts/users'
import { FieldError, FormError } from '@/components/ui/ErrorView'
import { useToast } from '@/components/ui/Toast'
import { EmojiPicker } from '@/components/ui/EmojiPicker'
import { fieldErrors } from '@/lib/api/errors'
import { LIMITS, runeLength } from '@/lib/utils/count'
import { updateMe } from '../api'

/**
 * editing your own record. the four fields PATCH /api/users/me accepts.
 * ! only the changed keys are sent. a dirty check also avoids the service's 400 "no fields to
 * ! update", which would otherwise scold somebody who opened the editor and pressed save
 */
export function ProfileEditor({ profile, onSaved }: { profile: Profile; onSaved: () => void }) {
  const [first, setFirst] = useState(profile.first_name)
  const [last, setLast] = useState(profile.last_name)
  const [nickname, setNickname] = useState(profile.nickname ?? '')
  const [about, setAbout] = useState(profile.about_me ?? '')
  const [error, setError] = useState<unknown>(null)
  const [busy, setBusy] = useState(false)
  const nickRef = useRef<HTMLInputElement>(null)
  const aboutRef = useRef<HTMLTextAreaElement>(null)
  const toast = useToast()
  const fields = fieldErrors(error)

  const patch: UpdateProfileInput = {}
  if (first !== profile.first_name) patch.first_name = first
  if (last !== profile.last_name) patch.last_name = last
  if (nickname !== (profile.nickname ?? '')) patch.nickname = nickname
  if (about !== (profile.about_me ?? '')) patch.about_me = about
  const dirty = Object.keys(patch).length > 0

  const save = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!dirty) return
    setBusy(true)
    setError(null)
    try {
      await updateMe(patch)
      toast('record updated')
      onSaved()
    } catch (err: unknown) {
      setError(err)
    } finally {
      setBusy(false)
    }
  }

  return (
    <form className="authform" onSubmit={save} style={{ marginTop: 14 }}>
      <FormError error={error} />
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12, marginBottom: 12 }}>
        <label className="field">
          <span className="lbl">first name</span>
          <input value={first} maxLength={LIMITS.name} onChange={(e) => setFirst(e.target.value)} />
          <FieldError fields={fields} name="first_name" />
        </label>
        <label className="field">
          <span className="lbl">last name</span>
          <input value={last} maxLength={LIMITS.name} onChange={(e) => setLast(e.target.value)} />
          <FieldError fields={fields} name="last_name" />
        </label>
      </div>

      <label className="field">
        <span className="lbl">nickname</span>
        <input ref={nickRef} value={nickname} maxLength={LIMITS.nickname} onChange={(e) => setNickname(e.target.value)} />
        <FieldError fields={fields} name="nickname" />
        {/*! the two fields a person writes in their own voice get the tray. first and last name
            do not: the api renders them as `first_name || ' ' || last_name` in every member
            list, notification and chat header, and that is identity rather than expression */}
        <EmojiPicker target={nickRef} onInsert={setNickname} placement="down" />
      </label>

      <label className="field">
        <span className="lbl">
          about me
          <span style={{ float: 'right', color: 'var(--ink-ghost)' }}>
            {runeLength(about)} / {LIMITS.aboutMe}
          </span>
        </span>
        <textarea ref={aboutRef} rows={3} value={about} onChange={(e) => setAbout(e.target.value)} />
        <FieldError fields={fields} name="about_me" />
        <EmojiPicker target={aboutRef} onInsert={setAbout} />
      </label>

      <button className="btn btn--sm btn--primary" type="submit" disabled={busy || !dirty}>
        {busy ? 'writing…' : dirty ? 'save' : 'nothing changed'}
      </button>
    </form>
  )
}
