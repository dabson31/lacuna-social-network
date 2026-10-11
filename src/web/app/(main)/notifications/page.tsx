'use client'

import { useState } from 'react'
import Link from 'next/link'
import { NOTIFICATION_LABEL } from '@shared/constants/domain'
import type { NotificationRow } from '@shared/contracts/notifications'
import { Avatar } from '@/components/ui/Avatar'
import { Empty } from '@/components/ui/Empty'
import { useToast } from '@/components/ui/Toast'
import { useBadges } from '@/components/layout/BadgeProvider'
import { clockTime } from '@/lib/utils/time'
import { markAllRead, markRead } from '@/features/notifications/api'
import { describeNotification, isStale, resolveNotification, targetHref, verbsFor } from '@/features/notifications/actions'

/**
 * the Notices channel, ported from viewNotices at line 4071.
 * ! the whole page runs off GET /api/notifications alone. every row carries type, actor_id and
 * ! entity_id, and between them that is enough to act on any of the four kinds with no second
 * ! lookup — the mapping lives in features/notifications/actions.ts
 *
 * ? the list comes from the same provider the right rail's Pending panel reads, so resolving
 * ? something here updates the panel and the tile badge at the same time
 */
export default function NoticesPage() {
  const { notifications, pending, dropNotification, reloadNotifications, reloadGroups } = useBadges()
  const toast = useToast()
  const [open, setOpen] = useState<number | null>(null)
  const [busy, setBusy] = useState<number | null>(null)

  const act = async (n: NotificationRow, accept: boolean) => {
    setBusy(n.id)
    try {
      await resolveNotification(n, accept)
      dropNotification(n.id)
      toast(`${NOTIFICATION_LABEL[n.type]} ${accept ? 'accepted' : 'declined'} · ${n.actor_name}`)
      if (n.type === 'group_invite' || n.type === 'group_join_request') reloadGroups()
    } catch (err: unknown) {
      //! a stale row is retired instead of being left in the queue. it points at something
      //! that is already gone, so the only thing another click can do is fail again
      if (isStale(err)) {
        dropNotification(n.id)
        void markRead(n.id).catch(() => {})
        toast(`${NOTIFICATION_LABEL[n.type]} · already dealt with`)
      } else {
        toast('that did not go through')
        reloadNotifications()
      }
    } finally {
      setBusy(null)
    }
  }

  const dismissAll = async () => {
    try {
      await markAllRead()
      reloadNotifications()
      toast('queue cleared')
    } catch {
      toast('could not clear the queue')
    }
  }

  return (
    <>
      <div className="main__bar">
        <span className="lbl">
          {pending} pending · {notifications.length} total
        </span>
        <div className="spacer" />
        {pending > 0 && (
          <button className="btn btn--sm" onClick={() => void dismissAll()}>
            dismiss all
          </button>
        )}
      </div>

      <div className="rows">
        {notifications.map((n) => {
          const done = n.read_at !== null
          const isOpen = open === n.id || (!done && open === null)
          const verbs = verbsFor(n.type)
          const href = targetHref(n)

          return (
            <div className="row-item" key={n.id} style={done ? { opacity: 0.5 } : undefined}>
              <button
                className="row-main"
                aria-expanded={isOpen}
                aria-controls={`ntc-${n.id}`}
                onClick={() => setOpen(open === n.id ? -1 : n.id)}
              >
                <span className={done ? 'dot dot--idle' : 'dot dot--on'} />
                <Avatar name={n.actor_name} path={n.actor_avatar} size="sm" />
                <span className="tx">
                  <span className="nm">{n.actor_name}</span>
                  <span className="mt">
                    {NOTIFICATION_LABEL[n.type]} · {clockTime(n.created_at)}
                    {done ? ' · resolved' : ''}
                  </span>
                </span>
                <span className="caret" aria-hidden="true">
                  ›
                </span>
              </button>

              <div className="row-detail" id={`ntc-${n.id}`} hidden={!isOpen}>
                <div className="pad">
                  <p className="note-txt">{describeNotification(n)}</p>
                  {!done && (
                    <div className="row-ops">
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
                      {href && (
                        <Link className="btn btn--sm" href={href}>
                          open
                        </Link>
                      )}
                    </div>
                  )}
                </div>
              </div>
            </div>
          )
        })}

        {notifications.length === 0 && <Empty label="nothing has happened yet" />}
      </div>
    </>
  )
}
