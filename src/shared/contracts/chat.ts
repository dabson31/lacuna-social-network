/**
 * chat.Conversation — one row of the dm list.
 * ! last_message is '' and last_at is null for someone you can chat with but never have. that
 * ! is intentional: the list doubles as "who can i even start a chat with", so an empty
 * ! preview is a normal row to render and not missing data to filter out
 */
export interface Conversation {
  peer_id: number
  first_name: string
  last_name: string
  avatar_path: string | null
  last_message: string
  last_at: string | null
  unread: number
}

/** chat.GroupConversation — the same row for a group room */
export interface GroupConversation {
  group_id: number
  title: string
  last_message: string
  last_sender: string
  last_at: string | null
  unread: number
}

/**
 * chat.Inbox — what GET /api/conversations gives back.
 * ? dms and groups come together because the ui draws them in one sidebar, and two requests
 * ? to fill one list is a waterfall for no reason
 */
export interface Inbox {
  direct: Conversation[]
  groups: GroupConversation[]
}

//? the two message shapes already live in ws.ts, because a row fetched from history and a
//? frame pushed over the socket are deliberately the exact same shape. re-export rather than
//? redeclare, or they drift
export type { DirectMessage, GroupMessage } from './ws'
