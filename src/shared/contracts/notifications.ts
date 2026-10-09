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
}

export type { NotificationRow } from './ws'
