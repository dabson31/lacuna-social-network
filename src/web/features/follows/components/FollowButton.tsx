'use client'

import { useState } from 'react'
import { FollowStatus } from '@shared/constants/domain'
import { useToast } from '@/components/ui/Toast'
import { isApiError } from '@/lib/api/errors'
import { follow, unfollow } from '../api'

/**
 * one button, four states, three of them from the server.
 * ? DELETE is both unfollow and cancel-a-pending-request, so the only difference between those
 * ? two is the label. the subject's bypass rule means a public target skips pending entirely,
 * ? which is why the next state is read off the response instead of assumed
 */
export function FollowButton({
  userId,
  status,
  onChange,
  small = true,
}: {
  userId: number
  status: FollowStatus
  onChange: (next: FollowStatus) => void
  small?: boolean
}) {
  const [busy, setBusy] = useState(false)
  const toast = useToast()
  const cls = small ? 'btn btn--sm' : 'btn'

  const act = async () => {
    setBusy(true)
    try {
      if (status === FollowStatus.None) {
        const { status: next } = await follow(userId)
        onChange(next)
        toast(next === 'accepted' ? 'now following' : 'request sent, waiting on them')
      } else {
        await unfollow(userId)
        onChange(FollowStatus.None)
        toast(status === FollowStatus.Pending ? 'request withdrawn' : 'unfollowed')
      }
    } catch (err: unknown) {
      /**
       * ! 409 already_following is the one failure here that is not really a failure: a relation
       * ! the caller could not see already exists. it happens in a followers or following list,
       * ! where the only thing available to derive a status from is the viewer's own accepted
       * ! follows — an outgoing request appears in neither of the api's two lists
       * ? the state is set to pending rather than left alone, because DELETE is both unfollow
       * ? and cancel-a-request: the action behind the button is right either way, and only the
       * ? word on it can be off, in the rarer case where the existing relation was accepted
       */
      if (isApiError(err) && err.status === 409) {
        onChange(FollowStatus.Pending)
        toast('you have already asked this account')
      } else {
        toast('that did not go through')
      }
    } finally {
      setBusy(false)
    }
  }

  const label =
    status === FollowStatus.Accepted ? 'unfollow' : status === FollowStatus.Pending ? 'cancel request' : 'follow'

  return (
    <button className={cls} disabled={busy} onClick={() => void act()}>
      {label}
    </button>
  )
}
