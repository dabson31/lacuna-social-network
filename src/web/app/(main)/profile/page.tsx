'use client'

import { useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { Loading } from '@/components/ui/Empty'
import { useSession } from '@/lib/session/SessionProvider'

/**
 * the Profile tile points here rather than at an id, because the tile is rendered before the
 * session has necessarily answered.
 * ? so this is a one hop redirect to /profile/{me}, which keeps the channel table free of a
 * ? value only the session knows
 */
export default function MyProfile() {
  const { state } = useSession()
  const router = useRouter()

  useEffect(() => {
    if (state.status === 'authenticated') router.replace(`/profile/${state.user.id}`)
    if (state.status === 'anonymous') router.replace('/login')
  }, [state, router])

  return <Loading label="opening your record…" />
}
