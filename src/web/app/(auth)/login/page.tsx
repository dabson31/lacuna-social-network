'use client'

import { Suspense, useState } from 'react'
import Link from 'next/link'
import { useRouter, useSearchParams } from 'next/navigation'
import { SessionProvider, useSession } from '@/lib/session/SessionProvider'
import { FormError } from '@/components/ui/ErrorView'
import { login } from '@/features/auth/api'

export default function LoginPage() {
  //? the auth pages carry their own provider: they sit outside (main), so they need the 401
  //? hook and the signIn setter without the shell around them
  //? Suspense because useSearchParams opts the route into client rendering, and without a
  //? boundary the whole page falls back instead of just the card
  return (
    <SessionProvider>
      <Suspense fallback={null}>
        <LoginCard />
      </Suspense>
    </SessionProvider>
  )
}

function LoginCard() {
  const router = useRouter()
  const params = useSearchParams()
  const { signIn } = useSession()
  const [error, setError] = useState<unknown>(null)
  const [busy, setBusy] = useState(false)

  const submit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    const form = new FormData(e.currentTarget)
    setBusy(true)
    setError(null)
    try {
      const user = await login(String(form.get('email') ?? ''), String(form.get('password') ?? ''))
      //? the response body is the user and the cookie is already set, so push it straight into
      //? the session rather than making `me` answer the same question again
      signIn(user)
      //? middleware stashed where they were headed before it bounced them here
      const next = params.get('next')
      router.replace(next?.startsWith('/') ? next : '/feed')
    } catch (err: unknown) {
      setError(err)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="panel authcard">
      {/* the index board's own header strip: accent square, label, spacer, node id */}
      <div className="board__hd">
        <i />
        <span className="lbl">Access</span>
        <span className="spacer" />
        <span className="lbl">lcn·7f3a</span>
      </div>

      <div className="authcard__body authform">
        <p className="authcard__mark">
          <i />
          LACUNA
        </p>
        <p className="note-txt" style={{ marginTop: 8 }}>
          Open a session on this node. It holds its own copy of everything you can see.
        </p>

        {/*
          ! a 401 here carries no fields, deliberately. the service uses one error for a wrong
          ! email and a wrong password so the response never confirms which addresses are
          ! registered, and it compares against a dummy hash so both paths take the same time.
          ! that vagueness is a feature and must not be "improved"
        */}
        <div style={{ marginTop: 18 }}>
          <FormError error={error} />
        </div>

        {/*
          !! method="post" matters even though onSubmit always preventDefaults it. if the page
          !! ever fails to hydrate — a bad deploy, a chunk 404, a thrown effect — react never
          !! attaches the handler and the browser submits this form natively. a form with no
          !! method defaults to GET, which would put the email and password in the query
          !! string, the address bar, and browser history. this one line makes the worst case
          !! a useless POST instead of a credential leak
        */}
        <form method="post" onSubmit={submit} style={{ marginTop: 14 }}>
          <label className="field">
            <span className="lbl">email</span>
            <input
              type="email"
              name="email"
              autoComplete="email"
              placeholder="you@node"
              required
              autoFocus
            />
          </label>

          <label className="field">
            <span className="lbl">password</span>
            <input type="password" name="password" autoComplete="current-password" required />
          </label>

          <button className="btn btn--primary submit" type="submit" disabled={busy}>
            {busy ? 'opening…' : 'open session'}
          </button>
        </form>

        <div className="authcard__rule" />
        <p className="note-txt">
          No account on this node? <Link href="/register">Request one</Link>.
        </p>
      </div>

      {/* the machined corner the design puts on a panel */}
      <span className="tick" aria-hidden="true" />
    </div>
  )
}
