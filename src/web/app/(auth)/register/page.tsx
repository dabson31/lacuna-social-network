'use client'

import { useState } from 'react'
import Link from 'next/link'
import { useRouter } from 'next/navigation'
import { SessionProvider, useSession } from '@/lib/session/SessionProvider'
import { FieldError, FormError } from '@/components/ui/ErrorView'
import { fieldErrors } from '@/lib/api/errors'
import { LIMITS, runeLength } from '@/lib/utils/count'
import { ImagePicker } from '@/features/auth/components/ImagePicker'
import { register } from '@/features/auth/api'

export default function RegisterPage() {
  return (
    <SessionProvider>
      <RegisterCard />
    </SessionProvider>
  )
}

/**
 * the seven fields the subject names, with the three optional ones visibly present.
 * ! "the Avatar/Image, Nickname and About Me should be present in the form but the user can
 * ! skip the filling of those fields" — so they are rendered and marked optional, never hidden
 * ! behind a disclosure that an auditor would have to find
 */
function RegisterCard() {
  const router = useRouter()
  const { signIn } = useSession()
  const [error, setError] = useState<unknown>(null)
  const [busy, setBusy] = useState(false)
  const [about, setAbout] = useState('')
  const fields = fieldErrors(error)

  const submit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    setBusy(true)
    setError(null)
    try {
      //! FormData straight off the form element, so every named input is included and the
      //! optional ones arrive as empty strings. the service turns an empty nickname into NULL
      //! rather than '', which matters because the column is UNIQUE and sqlite treats every
      //! NULL as distinct while every '' is equal — otherwise exactly one user could ever
      //! skip their nickname
      const user = await register(new FormData(e.currentTarget))
      signIn(user)
      router.replace('/feed')
    } catch (err: unknown) {
      setError(err)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="panel authform" style={{ padding: 26, width: '100%', maxWidth: 480 }}>
      <div className="mark" style={{ marginBottom: 4 }}>
        <i />
        LACUNA
      </div>
      <p className="lbl" style={{ marginBottom: 18 }}>
        new record · this node only
      </p>

      <FormError error={error} />

      <form onSubmit={submit}>
        <label className="field">
          <span className="lbl">email</span>
          <input type="email" name="email" autoComplete="email" required />
          <FieldError fields={fields} name="email" />
        </label>

        <label className="field">
          <span className="lbl">password</span>
          <input type="password" name="password" autoComplete="new-password" required minLength={8} />
          {/* the service's own rule: at least 8 characters, at most 72 bytes, which is bcrypt's
              hard limit rather than an arbitrary choice */}
          <FieldError fields={fields} name="password" />
        </label>

        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12, marginBottom: 12 }}>
          <label className="field">
            <span className="lbl">first name</span>
            <input name="first_name" autoComplete="given-name" required maxLength={LIMITS.name} />
            <FieldError fields={fields} name="first_name" />
          </label>
          <label className="field">
            <span className="lbl">last name</span>
            <input name="last_name" autoComplete="family-name" required maxLength={LIMITS.name} />
            <FieldError fields={fields} name="last_name" />
          </label>
        </div>

        <label className="field" style={{ marginBottom: 18 }}>
          <span className="lbl">date of birth</span>
          {/*? type=date produces exactly the YYYY-MM-DD the column holds and the service
              parses with time.DateOnly, so no formatting step is needed anywhere */}
          <input type="date" name="date_of_birth" required />
          <FieldError fields={fields} name="date_of_birth" />
        </label>

        <div className="sep" style={{ margin: '0 0 14px' }} />
        <p className="lbl" style={{ marginBottom: 12 }}>
          optional
        </p>

        <label className="field">
          <span className="lbl">nickname</span>
          <input name="nickname" maxLength={LIMITS.nickname} />
          <FieldError fields={fields} name="nickname" />
        </label>

        <label className="field">
          <span className="lbl">
            about me
            <span style={{ float: 'right', color: 'var(--ink-ghost)' }}>
              {runeLength(about)} / {LIMITS.aboutMe}
            </span>
          </span>
          {/*! counted in runes and not in string length, because the server counts with
              utf8.RuneCountInString. with .length an emoji costs two and the counter would
              reject text the server would happily take */}
          <textarea name="about_me" rows={3} value={about} onChange={(e) => setAbout(e.target.value)} />
          <FieldError fields={fields} name="about_me" />
        </label>

        <div style={{ marginBottom: 18 }}>
          <span className="lbl" style={{ display: 'block', marginBottom: 8 }}>
            avatar
          </span>
          <ImagePicker name="avatar" label="choose image" />
          <FieldError fields={fields} name="avatar" />
          <FieldError fields={fields} name="image" />
        </div>

        <button className="btn btn--primary" type="submit" disabled={busy} style={{ width: '100%' }}>
          {busy ? 'writing…' : 'create record'}
        </button>
      </form>

      <p className="note-txt" style={{ marginTop: 18 }}>
        {/*? default privacy is private, which the backend plan chose deliberately so the
            request-and-accept flow is the normal path rather than the edge case */}
        New accounts start private. Follow requests arrive as notices, and you can switch to a
        public profile at any time.
      </p>
      <p className="note-txt" style={{ marginTop: 10 }}>
        Already have a record? <Link className="link" href="/login">Open a session</Link>.
      </p>
    </div>
  )
}
