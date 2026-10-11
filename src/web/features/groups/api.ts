import type {
  CreateEventInput,
  CreateGroupInput,
  Event,
  Group,
  GroupComment,
  GroupDetail,
  GroupPost,
  Invitation,
  JoinRequest,
} from '@shared/contracts/groups'
import * as api from '@/lib/api/client'

/** grouped the way the routes group in mountGroups, so the file reads like the router */

// ---- lifecycle ----

/**
 * every group, with the caller's own my_status and my_role on each one.
 * ? that is what the subject asks for: "a section where you can browse through all groups".
 * ? not paginated, which is fine at this scale and would be a backend change if it were not
 */
export function browseGroups(): Promise<Group[]> {
  return api.json<Group[]>('/api/groups')
}

export function getGroup(id: number): Promise<GroupDetail> {
  return api.json<GroupDetail>(`/api/groups/${id}`)
}

/**
 * ! no join_policy in the body. the service defaults it to 'request', which is the flow the
 * ! audit checks, and the design's create form has no field for it. the column exists as an
 * ! extra and the migration says so
 */
export function createGroup(input: CreateGroupInput): Promise<Group> {
  return api.json<Group>('/api/groups', { method: 'POST', body: input })
}

// ---- by invite ----

/** any member may invite, not just the creator. the subject says so and the service allows it */
export function invite(groupId: number, userId: number): Promise<void> {
  return api.json<void>(`/api/groups/${groupId}/invite`, { method: 'POST', body: { user_id: userId } })
}

export function acceptInvite(groupId: number): Promise<void> {
  return api.json<void>(`/api/groups/${groupId}/invitations/accept`, { method: 'POST' })
}

export function declineInvite(groupId: number): Promise<void> {
  return api.json<void>(`/api/groups/${groupId}/invitations/decline`, { method: 'POST' })
}

export function listInvitations(): Promise<Invitation[]> {
  return api.json<Invitation[]>('/api/invitations')
}

// ---- by request ----

/** answers {"status": "requested"} normally, or "member" when the group's policy is open */
export function requestJoin(groupId: number): Promise<{ status: string }> {
  return api.json<{ status: string }>(`/api/groups/${groupId}/join`, { method: 'POST' })
}

/** creator only, a 403 for anybody else */
export function listRequests(groupId: number): Promise<JoinRequest[]> {
  return api.json<JoinRequest[]>(`/api/groups/${groupId}/requests`)
}

/**
 * ! two ids: the group and the asking user. the same pair arrives on a group_join_request
 * ! notification as entity_id and actor_id
 */
export function decideRequest(groupId: number, userId: number, accept: boolean): Promise<void> {
  return api.json<void>(`/api/groups/${groupId}/requests/${userId}/${accept ? 'accept' : 'decline'}`, {
    method: 'POST',
  })
}

export function leaveGroup(groupId: number): Promise<void> {
  return api.json<void>(`/api/groups/${groupId}/membership`, { method: 'DELETE' })
}

// ---- content, members only ----

export function listGroupPosts(groupId: number): Promise<GroupPost[]> {
  return api.json<GroupPost[]>(`/api/groups/${groupId}/posts`)
}

export function createGroupPost(groupId: number, body: FormData): Promise<GroupPost> {
  return api.form<GroupPost>(`/api/groups/${groupId}/posts`, body)
}

/**
 * clears this group's unread post count by moving the caller's watermark to the newest post.
 * ! the same shape as marking a chat room read, and for the same reason: unread posts are one
 * ! number per member, not a read flag per post per member
 */
export function markGroupPostsRead(groupId: number): Promise<void> {
  return api.json<void>(`/api/groups/${groupId}/posts/read`, { method: 'POST' })
}

export function listGroupComments(groupId: number, postId: number): Promise<GroupComment[]> {
  return api.json<GroupComment[]>(`/api/groups/${groupId}/posts/${postId}/comments`)
}

export function createGroupComment(groupId: number, postId: number, body: FormData): Promise<GroupComment> {
  return api.form<GroupComment>(`/api/groups/${groupId}/posts/${postId}/comments`, body)
}

// ---- events ----

export function listEvents(groupId: number): Promise<Event[]> {
  return api.json<Event[]>(`/api/groups/${groupId}/events`)
}

export function createEvent(groupId: number, input: CreateEventInput): Promise<Event> {
  return api.json<Event>(`/api/groups/${groupId}/events`, { method: 'POST', body: input })
}

/**
 * ! this route hangs off events and not off groups, because an rsvp is about the event and the
 * ! caller and the group is already implied by the event row. it is an upsert, so changing your
 * ! mind is the same call
 */
export function respondToEvent(eventId: number, response: 'going' | 'not_going'): Promise<void> {
  return api.json<void>(`/api/events/${eventId}/respond`, { method: 'POST', body: { response } })
}
