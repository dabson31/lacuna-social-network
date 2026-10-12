import type { EventResponse, JoinPolicy, MemberRole, MemberStatus } from '../constants/domain'

/**
 * groups.Group — the row plus a member count plus how the caller relates to it.
 * ! my_status and my_role come back on every group in the browse list, which is what makes
 * ! that page one request instead of one plus N. pick the button from these two and never
 * ! from a separate membership probe
 */
export interface Group {
  id: number
  creator_id: number
  title: string
  description: string
  join_policy: JoinPolicy
  created_at: string
  member_count: number
  my_status: MemberStatus
  my_role: MemberRole
  /**
   * posts added since this caller last read the group, their own excluded, and 0 for anyone
   * who is not a member.
   * ! counted off group_members.last_read_post_id — a watermark, the same one group messages
   * ! use. so there is no per-post read state to render, only this number, and it clears with
   * ! POST /api/groups/{id}/posts/read rather than by looking at anything
   */
  unread_posts: number
}

/** groups.GroupDetail — a group plus its member list, and the list is only filled for members */
export interface GroupDetail extends Group {
  members: Member[]
}

export interface Member {
  id: number
  first_name: string
  last_name: string
  avatar_path: string | null
  role: MemberRole
}

/** groups.Invitation — one pending invite shown to the invited user */
export interface Invitation {
  group_id: number
  title: string
  description: string
  invited_by_id: number | null
  invited_by: string | null
}

/** groups.JoinRequest — one pending request shown to the creator */
export interface JoinRequest {
  user_id: number
  first_name: string
  last_name: string
  avatar_path: string | null
}

/** groups.GroupPost — a post with no privacy field, because membership is the privacy */
export interface GroupPost {
  id: number
  group_id: number
  author_id: number
  content: string
  image_path: string | null
  created_at: string
  author_name: string
  author_avatar: string | null
  comment_count: number
}

export interface GroupComment {
  id: number
  group_post_id: number
  author_id: number
  content: string
  image_path: string | null
  created_at: string
  author_name: string
  author_avatar: string | null
}

/**
 * groups.Event.
 * ? it carries both tallies and the caller's own response, so the counts and the pressed state
 * ? of the rsvp pair need no second call
 */
export interface Event {
  id: number
  group_id: number
  creator_id: number
  title: string
  description: string
  /** RFC 3339 */
  event_time: string
  created_at: string
  going_count: number
  not_going_count: number
  my_response: EventResponse
}

export interface CreateGroupInput {
  title: string
  description: string
}

export interface CreateEventInput {
  title: string
  description: string
  /** RFC 3339, for example 2026-12-25T18:00:00Z */
  event_time: string
}
