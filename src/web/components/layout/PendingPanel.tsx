'use client'

import { useState } from 'react'
import { NOTIFICATION_LABEL } from '@shared/constants/domain'
import type { NotificationRow } from '@shared/contracts/notifications'
import { Avatar } from '@/components/ui/Avatar'
import { useToast } from '@/components/ui/Toast'
import { resolveNotification, verbsFor, describeNotification } from '@/features/notifications/actions'
import { useBadges } from './BadgeProvider'

/**
 * the Pending panel, in the right rail, on every route.
 * ! this is the half of "notifications on every page" that is actionable rather than just a
 * ! count. the four kinds each take a different endpoint, and resolveNotification owns that
 * ! mapping so this component is only layout
 */
export function PendingPanel() {
  const { notifications, pending, dropNotification, reloadNotifications, reloadGroups } = useBadges()
  const toast = useToast()
  const [open, setOpen] = useState<number | null>(null)
  const [busy, setBusy] = useState<number | null>(null)

  //? the panel shows what is still unresolved. a read notification has been dealt with and
  //? stays in the Notices channel, which is what the design's empty copy says too
  const shown = notifications.filter((n) => n.read_at === null)

  const act = async (n: NotificationRow, accept: boolean) => {
    setBusy(n.id)
    try {
      await resolveNotification(n, accept)
      dropNotification(n.id)
      toast(`${NOTIFICATION_LABEL[n.type]} ${accept ? 'accepted' : 'declined'} · ${n.actor_name}`)
      //? a group kind changes membership, so the group tile and any open group page are stale
      if (n.type === 'group_invite' || n.type === 'group_join_request') reloadGroups()
    } catch {
      toast('that did not go through')
      reloadNotifications()
    } finally {
      setBusy(null)
    }
  }

  return (
    <section className="sec" aria-labelledby="pendHd">
      <div className="sec__hd">
        <span className="lbl" id="pendHd">
          Pending
        </span>
        <div className="spacer" />
        <span className="ct">{pending}</span>
      </div>

      <div>
        {shown.map((n) => {
          const verbs = verbsFor(n.type)
          const isOpen = open === n.id
          return (
            <div className="req" key={n.id}>
              <button
                className="req__main"
                aria-expanded={isOpen}
                aria-controls={`rq-${n.id}`}
                onClick={() => setOpen(isOpen ? null : n.id)}
              >
                <Avatar name={n.actor_name} path={n.actor_avatar} size="sm" />
                <span className="tx">
                  <span className="nm">{n.actor_name}</span>
                  <span className="ty">{NOTIFICATION_LABEL[n.type]}</span>
                </span>
                <span className="caret" aria-hidden="true">
                  ›
                </span>
              </button>
              <div className="req__body" id={`rq-${n.id}`} hidden={!isOpen}>
                <p className="ex">{describeNotification(n)}</p>
                <div className="req__ops">
                  <button className="btn btn--sm" disabled={busy === n.id} onClick={() => void act(n, true)}>
                    {verbs.accept}
                  </button>
                  <button
                    className="btn btn--sm btn--danger"
                    disabled={busy === n.id}
                    onClick={() => void act(n, false)}
                  >
                    {verbs.decline}
                  </button>
                </div>
              </div>
            </div>
          )
        })}
      </div>

      {/* the design's own empty copy, kept verbatim */}
      <p className="note-txt" hidden={shown.length > 0}>
        Queue clear. Resolved items stay in the notices channel.
      </p>
    </section>
  )
}
