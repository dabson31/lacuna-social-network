import { WsFrame } from '../constants/ws'

/**
 * then payloads each mirrors a Go struct's json tags one for one
 * snake_case because that is what the server sends
 */

/** chat.DirectMessage */
export interface DirectMessage {
  id: number
  sender_id: number
  recipient_id: number
  content: string
  created_at: string // RFC 3339
  read_at: string | null // null until the recipient opens the conversation
  sender_name: string
  sender_avatar: string | null
}

/** chat.GroupMessage */
export interface GroupMessage {
  id: number
  group_id: number
  sender_id: number
  content: string
  created_at: string
  sender_name: string
  sender_avatar: string | null
}

/** chat.TypingOut — the server tells you who is typing at you */
export interface TypingNotice {
  from_user_id: number
}

/** notifications.Notification */
export interface NotificationRow {
  id: number
  type: 'follow_request' | 'group_invite' | 'group_join_request' | 'event_created'
  actor_id: number
  actor_name: string
  actor_avatar: string | null
  entity_id: number | null
  /**
   * which group this notification is about, null for a kind that is about no group.
   * ! derived by the server, not stored, because entity_id names a different table per kind:
   * ! the group for an invite or a join request, the EVENT for event_created. so this is the
   * ! only field that can file a notification under a group without the client re-deriving a
   * ! rule it has no data for
   */
  group_id: number | null
  read_at: string | null
  created_at: string
}

/**
 * the sexy ass union itself 
 * `type` is the discriminant, so a `switch (frame.type)` narrows `frame.payload` to the right shape in each branch with no casts anywhere
 */
export type ServerFrame =
  | { type: typeof WsFrame.DmMessage; payload: DirectMessage }
  | { type: typeof WsFrame.GroupMessage; payload: GroupMessage }
  | { type: typeof WsFrame.DmTyping; payload: TypingNotice }
  | { type: typeof WsFrame.NotificationNew; payload: NotificationRow }
  | { type: typeof WsFrame.Pong; payload?: null }

/** The only two things the client is allowed to send. */
export type ClientFrame = { type: typeof WsFrame.DmTyping; payload: { peer_id: number } } | { type: typeof WsFrame.Ping; payload?: null }

/**
 * Put this in the `default:` of your frame switch. Because `frame` is narrowed to `never`
 * once every case is handled, adding a variant to ServerFrame without handling it is a
 * compile error — which is the entire reason this file exists rather than a bag of `any`.
 */
export function assertNever(frame: never): never {
  throw new Error(`unhandled ws frame: ${JSON.stringify(frame)}`)
}

/**
 * Anything off a socket is untrusted input, so the boundary gets one real check rather
 * than a cast. This validates the envelope only; payload shape is the server's contract.
 */
export function parseServerFrame(raw: string): ServerFrame | null {
  try {
    const value: unknown = JSON.parse(raw)
    if (typeof value !== 'object' || value === null) return null
    const { type } = value as { type?: unknown }
    if (typeof type !== 'string') return null
    const known: readonly string[] = Object.values(WsFrame)
    return known.includes(type) ? (value as ServerFrame) : null
  } catch {
    return null
  }
}
