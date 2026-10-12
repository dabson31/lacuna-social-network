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
 * ! behind a disclosure an auditor would have to go looking for
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
      //! NULL as distinct while every '' is equal — otherwise exactly one account could ever
      //! skip its nickname
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
    <div className="panel authcard" style={{ maxWidth: 440 }}>
      <div className="board__hd">
        <i />
        <span className="lbl">New record</span>
        <span className="spacer" />
        <span className="lbl">lcn·7f3a</span>
      </div>

      <div className="authcard__body authform">
        <p className="authcard__mark">
          <i />
          LACUNA
        </p>
        <p className="note-txt" style={{ marginTop: 8 }}>
          This node keeps your record. Nothing here is replicated anywhere else.
        </p>

        <div style={{ marginTop: 18 }}>
          <FormError error={error} />
        </div>

        {/*
          !! method="post" for the same reason as the login form: if the page fails to hydrate,
          !! the browser submits natively, and a form with no method defaults to GET — which
          !! would put an email and a password in the address bar and in history
        */}
        <form method="post" onSubmit={submit} style={{ marginTop: 14 }}>
          <label className="field">
            <span className="lbl">email</span>
            <input type="email" name="email" autoComplete="email" placeholder="you@node" required autoFocus />
            <FieldError fields={fields} name="email" />
          </label>

          <label className="field">
            <span className="lbl">password</span>
            {/* the service's rule: at least 8 characters, at most 72 bytes, which is bcrypt's
                own hard limit rather than an arbitrary choice */}
            <input
              type="password"
              name="password"
              autoComplete="new-password"
              placeholder="at least 8 characters"
              required
              minLength={8}
            />
            <FieldError fields={fields} name="password" />
          </label>

          <div className="pair">
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

          <label className="field">
            <span className="lbl">date of birth</span>
            {/*? type=date produces exactly the YYYY-MM-DD the column holds and the service
                parses with time.DateOnly, so no formatting step is needed anywhere */}
            <input type="date" name="date_of_birth" required />
            <FieldError fields={fields} name="date_of_birth" />
          </label>

          <div className="group-hd">
            <span className="lbl">optional</span>
          </div>

          <label className="field">
            <span className="lbl">nickname</span>
            <input name="nickname" placeholder="shown instead of your name" maxLength={LIMITS.nickname} />
            <FieldError fields={fields} name="nickname" />
          </label>

          <label className="field">
            <span className="lbl">
              about me
              <span className="count">
                {runeLength(about)} / {LIMITS.aboutMe}
              </span>
            </span>
            {/*! counted in runes and not in string length, because the server counts with
                utf8.RuneCountInString. with .length an emoji costs two and this counter would
                refuse text the server would happily take */}
            <textarea name="about_me" rows={3} value={about} onChange={(e) => setAbout(e.target.value)} />
            <FieldError fields={fields} name="about_me" />
          </label>

          <div className="field">
            <span className="lbl">avatar</span>
            <ImagePicker name="avatar" label="choose image" />
            <FieldError fields={fields} name="avatar" />
            <FieldError fields={fields} name="image" />
          </div>

          <button className="btn btn--primary submit" type="submit" disabled={busy}>
            {busy ? 'writing…' : 'create record'}
          </button>
        </form>

        <div className="authcard__rule" />
        {/*? default privacy is private, which the backend plan chose deliberately so the
            request-and-accept flow is the normal path rather than the edge case */}
        <p className="note-txt">
          New accounts start private: follow requests arrive as notices, and you can switch to a
          public profile at any time.
        </p>
        <p className="note-txt" style={{ marginTop: 10 }}>
          Already have a record? <Link href="/login">Open a session</Link>.
        </p>
      </div>

      <span className="tick" aria-hidden="true" />
    </div>
  )
}
