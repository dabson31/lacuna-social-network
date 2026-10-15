import type { Feed } from '@shared/contracts/notifications'
import * as api from '@/lib/api/client'

/** notifications uses offset paging, unlike the feed and chat which use a cursor */
export function listNotifications(limit = 20, offset = 0): Promise<Feed> {
  return api.json<Feed>('/api/notifications' + api.query({ limit, offset }))
}

export function markRead(id: number): Promise<void> {
  return api.json<void>(`/api/notifications/${id}/read`, { method: 'POST' })
}

export function markAllRead(): Promise<void> {
  return api.json<void>('/api/notifications/read-all', { method: 'POST' })
}
