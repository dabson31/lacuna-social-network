'use client'

import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'
import { usePathname, useRouter } from 'next/navigation'
import type { User } from '@shared/contracts/users'
import * as api from '@/lib/api/client'
import { isApiError } from '@/lib/api/errors'

/**
 * ! three states and not a boolean. "not yet known" and "not logged in" look the same to a
 * ! boolean, which is the bug where a reader with a perfectly good cookie sees the login page
 * ! flash on every full page load because `me` has not answered yet
 */
export type SessionState =
  | { status: 'loading' }
  | { status: 'authenticated'; user: User }
  | { status: 'anonymous' }

interface SessionContextValue {
  state: SessionState
  /** login and register call this with the body they already have, so `me` is not refetched */
  signIn: (user: User) => void
  signOut: () => void
  /** after a profile patch, since PATCH answers 204 and hands nothing back */
  refresh: () => Promise<void>
}

const SessionContext = createContext<SessionContextValue | null>(null)

export function SessionProvider({ children }: { children: React.ReactNode }) {
  const [state, setState] = useState<SessionState>({ status: 'loading' })
  const router = useRouter()
  const pathname = usePathname()

  const load = useCallback(async () => {
    try {
      const user = await api.json<User>('/api/auth/me')
      setState({ status: 'authenticated', user })
    } catch (e: unknown) {
      //? a 401 from `me` on a cold load is the ordinary anonymous case, not a failure. any
      //? other error is also treated as anonymous here, because there is nothing a page can
      //? do with a half known session and the api will say so again on the next call
      if (isApiError(e)) setState({ status: 'anonymous' })
      else setState({ status: 'anonymous' })
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  /**
   * ! the global 401 hook. a 401 from any call means the cookie died, so every page on screen
   * ! is wrong and the only correct move is back to login.
   * ! the pathname guard is what stops this looping: `me` answering 401 while already on
   * ! /login would otherwise redirect to /login forever
   */
  useEffect(() => {
    api.setUnauthorizedHandler(() => {
      setState({ status: 'anonymous' })
      if (pathname !== '/login' && pathname !== '/register') router.replace('/login')
    })
    return () => api.setUnauthorizedHandler(null)
  }, [pathname, router])

  const value = useMemo<SessionContextValue>(
    () => ({
      state,
      signIn: (user) => setState({ status: 'authenticated', user }),
      signOut: () => setState({ status: 'anonymous' }),
      refresh: load,
    }),
    [state, load],
  )

  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>
}

export function useSession(): SessionContextValue {
  const ctx = useContext(SessionContext)
  if (!ctx) throw new Error('useSession used outside SessionProvider')
  return ctx
}

/**
 * for the many components inside (main) that know a user exists because the layout already
 * waited for one.
 * ? it throws rather than returning null so a component never has to null check a user the
 * ? layout has already guaranteed
 */
export function useUser(): User {
  const { state } = useSession()
  if (state.status !== 'authenticated') throw new Error('useUser used outside an authenticated route')
  return state.user
}
