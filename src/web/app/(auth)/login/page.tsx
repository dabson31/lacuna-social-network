'use client'

import { useState } from 'react'
import Link from 'next/link'
import { useRouter } from 'next/navigation'
import { SessionProvider, useSession } from '@/lib/session/SessionProvider'
import { FormError } from '@/components/ui/ErrorView'
import { login } from '@/features/auth/api'

export default function LoginPage() {
  //? the auth pages carry their own provider: they are outside (main), so they need the 401
  //? hook and the signIn setter without the shell around them
  return (
    <SessionProvider>
      <LoginCard />
    </SessionProvider>
  )
}

function LoginCard() {
  const router = useRouter()
  const { signIn } = useSession()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<unknown>(null)
  const [busy, setBusy] = useState(false)

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setBusy(true)
    setError(null)
    try {
      const user = await login(email, password)
      //? the response body is the user and the cookie is already set, so push it straight into
      //? the session rather than making `me` answer the same question again
      signIn(user)
      router.replace('/feed')
    } catch (err: unknown) {
      setError(err)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="panel authform" style={{ padding: 26, width: '100%', maxWidth: 380 }}>
      <div className="mark" style={{ marginBottom: 4 }}>
        <i />
        LACUNA
      </div>
      <p className="lbl" style={{ marginBottom: 18 }}>
        node lcn·7f3a · layer 02
      </p>

      {/*
        ! a 401 here carries no fields, deliberately. the service uses one error for a wrong
        ! email and a wrong password so the response never confirms which addresses are
        ! registered, and it compares against a dummy hash so both paths take the same time.
        ! that vagueness is a feature and must not be "improved"
      */}
      <FormError error={error} />

      <form onSubmit={submit}>
        <label className="field">
          <span className="lbl">email</span>
          <input
            type="email"
            autoComplete="email"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
          />
        </label>

        <label className="field" style={{ marginBottom: 18 }}>
          <span className="lbl">password</span>
          <input
            type="password"
            autoComplete="current-password"
            required
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </label>

        <button className="btn btn--primary" type="submit" disabled={busy} style={{ width: '100%' }}>
          {busy ? 'opening…' : 'open session'}
        </button>
      </form>

      <p className="note-txt" style={{ marginTop: 18 }}>
        No account on this node? <Link className="link" href="/register">Request one</Link>.
      </p>
    </div>
  )
}
