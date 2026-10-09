'use client'

import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'
import { WsFrame } from '@shared/constants/ws'
import type { Feed, NotificationRow } from '@shared/contracts/notifications'
import type { Inbox } from '@shared/contracts/chat'
import type { Group } from '@shared/contracts/groups'
import * as api from '@/lib/api/client'
import { useSession } from '@/lib/session/SessionProvider'
import { useFrame } from '@/lib/realtime/SocketProvider'

/**
 * the counters the shell shows on every route, in one place.
 * ! this is what makes "a user must be able to see the notifications in every page" structural
 * ! rather than something you remember to add. the provider sits in (main)/layout.tsx, so every
 * ! page inherits it and no page can be shipped without it
 *
 * ! notifications and private messages are kept as two separate numbers from two separate
 * ! sources, because the subject requires them to be distinguishable. merging them into one
 * ! bell is the mistake this file exists to prevent
 */
interface Badges {
  /** unresolved notifications. notifications.Feed.unread_count */
  pending: number
  /** unread direct messages, summed across conversations. a different counter entirely */
  unread: number
  /** groups the caller is a member of, for the tile */
  groups: number

  notifications: NotificationRow[]
  inbox: Inbox | null
  myGroups: Group[]

  reloadNotifications: () => void
  reloadInbox: () => void
  reloadGroups: () => void
  /** after an accept or decline, so the panel and the page agree without a full refetch */
  dropNotification: (id: number) => void
}

const BadgeContext = createContext<Badges | null>(null)

export function BadgeProvider({ children }: { children: React.ReactNode }) {
  const { state } = useSession()
  const authed = state.status === 'authenticated'

  const [feed, setFeed] = useState<Feed | null>(null)
  const [inbox, setInbox] = useState<Inbox | null>(null)
  const [groups, setGroups] = useState<Group[]>([])

  const reloadNotifications = useCallback(() => {
    if (!authed) return
    void api
      .json<Feed>('/api/notifications' + api.query({ limit: 50 }))
      .then(setFeed)
      .catch(() => {
        /* the shell must render even when a panel cannot fill, so this stays silent */
      })
  }, [authed])

  const reloadInbox = useCallback(() => {
    if (!authed) return
    void api
      .json<Inbox>('/api/conversations')
      .then(setInbox)
      .catch(() => {})
  }, [authed])

  const reloadGroups = useCallback(() => {
    if (!authed) return
    void api
      .json<Group[]>('/api/groups')
      .then(setGroups)
      .catch(() => {})
  }, [authed])

  useEffect(() => {
    reloadNotifications()
    reloadInbox()
    reloadGroups()
  }, [reloadNotifications, reloadInbox, reloadGroups])

  /**
   * ! the live half. the row is already written and already counted by the time this frame
   * ! arrives, so prepending it here is the doorbell and not the record. a reader who was
   * ! offline still finds it on the next load, which is why Record writes first and pushes
   * ! second
   */
  useFrame(WsFrame.NotificationNew, (row) => {
    setFeed((prev) =>
      prev === null
        ? { notifications: [row], unread_count: 1 }
        : {
            notifications: [row, ...prev.notifications.filter((n) => n.id !== row.id)],
            unread_count: prev.unread_count + 1,
          },
    )
  })

  //? a dm arriving anywhere bumps the inbox, which is cheaper and more correct than trying to
  //? patch an unread count per conversation from the frame alone
  useFrame(WsFrame.DmMessage, () => reloadInbox(), [reloadInbox])
  useFrame(WsFrame.GroupMessage, () => reloadInbox(), [reloadInbox])

  const value = useMemo<Badges>(() => {
    const unread = (inbox?.direct ?? []).reduce((n, c) => n + c.unread, 0)
    return {
      pending: feed?.unread_count ?? 0,
      unread,
      groups: groups.filter((g) => g.my_status === 'member').length,
      notifications: feed?.notifications ?? [],
      inbox,
      myGroups: groups,
      reloadNotifications,
      reloadInbox,
      reloadGroups,
      dropNotification: (id) =>
        setFeed((prev) =>
          prev === null
            ? prev
            : {
                notifications: prev.notifications.filter((n) => n.id !== id),
                unread_count: Math.max(0, prev.unread_count - 1),
              },
        ),
    }
  }, [feed, inbox, groups, reloadNotifications, reloadInbox, reloadGroups])

  return <BadgeContext.Provider value={value}>{children}</BadgeContext.Provider>
}

export function useBadges(): Badges {
  const ctx = useContext(BadgeContext)
  if (!ctx) throw new Error('useBadges used outside BadgeProvider')
  return ctx
}
