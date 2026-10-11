'use client'

import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'
import { usePathname } from 'next/navigation'
import { WsFrame } from '@shared/constants/ws'
import { NOTIFICATION_LABEL } from '@shared/constants/domain'
import type { Feed, GroupUnread, NotificationRow } from '@shared/contracts/notifications'
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
  /**
   * unresolved notifications filed under a group, one row per group.
   * ! a fourth number rather than part of `pending`, because "how many notices are waiting"
   * ! and "which group is waiting on me" are different questions. the groups list needs the
   * ! second one per row, and the nav tile needs its sum
   * ? only the three group kinds land here, so this sum is a slice of `pending` and not a
   * ? second count of the same thing. a follow request belongs to no group and never appears
   */
  groupPending: GroupUnread[]
  /** the sum of groupPending, for the Groups tile in the left rail */
  groupPendingTotal: number

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

/**
 * move one group's tally by `by`, adding the row if it is new and dropping it at zero.
 * ? a null group means the notification belongs to no group — a follow request — so there is
 * ? nothing to move, and the list comes back untouched
 */
function bumpGroup(list: GroupUnread[], groupId: number | null, by: number): GroupUnread[] {
  if (groupId === null) return list
  if (!list.some((g) => g.group_id === groupId)) {
    return by > 0 ? [...list, { group_id: groupId, unread: by }] : list
  }
  return list
    .map((g) => (g.group_id === groupId ? { ...g, unread: g.unread + by } : g))
    .filter((g) => g.unread > 0)
}

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
    setFeed((prev) => {
      if (prev === null) {
        return { notifications: [row], unread_count: 1, group_unread: bumpGroup([], row.group_id, 1) }
      }
      /**
       * ! a repeated action — a follow request sent again after being withdrawn, an invite
       * ! resent — refreshes the row the server was already holding, so this frame can carry
       * ! an id this feed already has. replacing the row is right, its entity moved. counting
       * ! it a second time is not, and neither is a second tick on the group tally
       * ? the filter below already did the replacing. this only decides whether the row is
       * ? news or an update of something already counted
       */
      const known = prev.notifications.some((n) => n.id === row.id && n.read_at === null)
      return {
        notifications: [row, ...prev.notifications.filter((n) => n.id !== row.id)],
        unread_count: known ? prev.unread_count : prev.unread_count + 1,
        //? the frame carries group_id for exactly this, so an invite or a new event lights
        //? up its own group without a refetch
        group_unread: known ? prev.group_unread : bumpGroup(prev.group_unread, row.group_id, 1),
      }
    })
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
    const groupPending = feed?.group_unread ?? []
    return {
      pending: feed?.unread_count ?? 0,
      unread,
      unreadGroups,
      groups: groups.filter((g) => g.my_status === 'member').length,
      groupPending,
      groupPendingTotal: groupPending.reduce((n, g) => n + g.unread, 0),
      notifications: feed?.notifications ?? [],
      inbox,
      myGroups: groups,
      reloadNotifications,
      reloadInbox,
      reloadGroups,
      //! the row is looked up before it is dropped, because the group tally has to come down
      //! with the global one. taking only an id and decrementing `pending` alone is how the
      //! groups list ends up still showing a number for something already dealt with
      dropNotification: (id) =>
        setFeed((prev) => {
          if (prev === null) return prev
          const gone = prev.notifications.find((n) => n.id === id)
          return {
            notifications: prev.notifications.filter((n) => n.id !== id),
            unread_count: Math.max(0, prev.unread_count - 1),
            group_unread: bumpGroup(prev.group_unread, gone?.group_id ?? null, -1),
          }
        }),
    }
  }, [feed, inbox, groups, reloadNotifications, reloadInbox, reloadGroups])

  return <BadgeContext.Provider value={value}>{children}</BadgeContext.Provider>
}

export function useBadges(): Badges {
  const ctx = useContext(BadgeContext)
  if (!ctx) throw new Error('useBadges used outside BadgeProvider')
  return ctx
}
