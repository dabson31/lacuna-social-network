'use client'

import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'
import { usePathname } from 'next/navigation'
import { WsFrame } from '@shared/constants/ws'
import { NOTIFICATION_LABEL } from '@shared/constants/domain'
import type { Feed, NotificationRow } from '@shared/contracts/notifications'
import type { Inbox } from '@shared/contracts/chat'
import type { Group } from '@shared/contracts/groups'
import * as api from '@/lib/api/client'
import { useSession } from '@/lib/session/SessionProvider'
import { useFrame } from '@/lib/realtime/SocketProvider'
import { useToast } from '@/components/ui/Toast'

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
  /**
   * unread group room messages, summed across rooms.
   * ! a third number rather than folded into `unread`, because the two are answered by two
   * ! different mechanisms: a dm counts rows with read_at IS NULL, a room counts rows past
   * ! group_members.last_read_message_id. anything that wants "unread messages" adds them,
   * ! and anything that wants one kind still has it
   */
  unreadGroups: number
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
  const toast = useToast()
  //? not useUser(): this provider mounts above the gate in Shell, so it has to cope with the
  //? loading and anonymous states rather than throw over them
  const me = state.status === 'authenticated' ? state.user : null
  //? a message you are already reading does not need announcing, and in an active thread a
  //? toast per frame is noise rather than a notification
  const pathname = usePathname()

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
    //! the pop-up half of "a user must be able to see the notifications in every page". the
    //! count and the Pending panel both needed you to be looking at them; this announces
    //! itself. all four required kinds come down this one frame, so one line covers follow
    //! requests, group invites, join requests and new events
    toast(`${NOTIFICATION_LABEL[row.type]} · ${row.actor_name}`)
  })

  //? a dm arriving anywhere bumps the inbox, which is cheaper and more correct than trying to
  //? patch an unread count per conversation from the frame alone
  useFrame(
    WsFrame.DmMessage,
    (m) => {
      reloadInbox()
      //? the sender receives their own message back by design — the rest commits the row and
      //? the socket pushes it — so announcing it would toast at you every time you typed
      if (m.sender_id === me?.id) return
      if (pathname === `/chat/${m.sender_id}`) return
      toast(`message · ${m.sender_name}`)
    },
    [reloadInbox],
  )

  useFrame(
    WsFrame.GroupMessage,
    (m) => {
      reloadInbox()
      if (m.sender_id === me?.id) return
      if (pathname === `/groups/${m.group_id}/chat`) return
      //? the frame carries the group id and not its title, and the titles are already here
      const room = groups.find((g) => g.id === m.group_id)
      toast(room ? `${room.title} · ${m.sender_name}` : `group message · ${m.sender_name}`)
    },
    [reloadInbox],
  )

  const value = useMemo<Badges>(() => {
    const unread = (inbox?.direct ?? []).reduce((n, c) => n + c.unread, 0)
    //! the group half of the same question, which nothing outside /chat was reading. the api
    //! has always returned it per room in inbox.groups[].unread — the counter simply stopped
    //! at direct messages, so a room with unread messages lit no badge anywhere
    const unreadGroups = (inbox?.groups ?? []).reduce((n, c) => n + c.unread, 0)
    return {
      pending: feed?.unread_count ?? 0,
      unread,
      unreadGroups,
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
