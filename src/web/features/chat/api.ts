import type { DirectMessage, GroupMessage, Inbox } from '@shared/contracts/chat'
import * as api from '@/lib/api/client'

/**
 * ? one call fills the whole sidebar, dms and group rooms together, because the ui draws them
 * ? in one list and two requests for one list is a waterfall for nothing
 * ! the direct list is not just history: its query unions accepted follows in both directions
 * ! with anyone you have ever messaged, so it doubles as "who can i start a chat with". the one
 * ! case it leaves out is a public-profile stranger, who is reachable from the directory
 */
export function getInbox(): Promise<Inbox> {
  return api.json<Inbox>('/api/conversations')
}

/** before is a message id, zero means from the newest. 50 default, 100 max */
export function dmHistory(peerId: number, before: number, limit = 50): Promise<DirectMessage[]> {
  return api.json<DirectMessage[]>(`/api/messages/${peerId}` + api.query({ before: before || undefined, limit }))
}

/**
 * ! 201 with the saved row, so the client can swap an optimistic bubble for the real id and
 * ! the server's timestamp. the socket then pushes the same row, which the sender throws away
 * ! by id — the handler's own comment says to do exactly that
 */
export function sendDM(recipientId: number, content: string): Promise<DirectMessage> {
  return api.json<DirectMessage>('/api/messages', { method: 'POST', body: { recipient_id: recipientId, content } })
}

export function markDMRead(peerId: number): Promise<void> {
  return api.json<void>(`/api/messages/${peerId}/read`, { method: 'POST' })
}

export function groupHistory(groupId: number, before: number, limit = 50): Promise<GroupMessage[]> {
  return api.json<GroupMessage[]>(`/api/groups/${groupId}/messages` + api.query({ before: before || undefined, limit }))
}

export function sendGroupMessage(groupId: number, content: string): Promise<GroupMessage> {
  return api.json<GroupMessage>(`/api/groups/${groupId}/messages`, { method: 'POST', body: { content } })
}

export function markGroupRead(groupId: number): Promise<void> {
  return api.json<void>(`/api/groups/${groupId}/messages/read`, { method: 'POST' })
}
