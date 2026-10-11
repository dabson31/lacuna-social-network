import type { NotificationRow } from './ws'

/**
 * notifications.Feed — the inbox payload.
 * ! the unread count rides along so the bell can draw a badge without a second call, and it is
 * ! deliberately a different counter from the dm unread count. the subject requires the two to
 * ! be distinguishable, so they stay two numbers in two places
 */
export interface Feed {
  notifications: NotificationRow[]
  unread_count: number
  /**
   * the same unread count split per group.
   * ! counted by the server rather than derived from `notifications`, because that array is
   * ! one page of the inbox. deriving per-group numbers from it would undercount as soon as
   * ! somebody has more unread notifications than the page holds
   */
  group_unread: GroupUnread[]
}

/** notifications.GroupUnread — one group's share of the unread count */
export interface GroupUnread {
  group_id: number
  unread: number
}

export type { NotificationRow } from './ws'
