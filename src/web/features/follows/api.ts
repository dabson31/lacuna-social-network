import type { FollowResult, RequestItem, UserCard } from '@shared/contracts/follows'
import * as api from '@/lib/api/client'

/**
 * ! the status comes back from the server and is never guessed on the client. the subject's
 * ! bypass rule is decided in go: a public target auto accepts and answers "accepted", a
 * ! private one answers "pending". optimistically rendering "requested" is wrong on every
 * ! public account
 */
export function follow(userId: number): Promise<FollowResult> {
  return api.json<FollowResult>(`/api/users/${userId}/follow`, { method: 'POST' })
}

/**
 * ? both unfollow and cancel-a-pending-request. one route, two meanings, and the label is the
 * ? only thing that differs: "unfollow" when accepted, "cancel" when pending
 */
export function unfollow(userId: number): Promise<void> {
  return api.json<void>(`/api/users/${userId}/follow`, { method: 'DELETE' })
}

/** both lists are gated by the same canView rule as the profile, so a stranger gets a 404 */
export function listFollowers(userId: number): Promise<UserCard[]> {
  return api.json<UserCard[]>(`/api/users/${userId}/followers`)
}

export function listFollowing(userId: number): Promise<UserCard[]> {
  return api.json<UserCard[]>(`/api/users/${userId}/following`)
}

export function incomingRequests(): Promise<RequestItem[]> {
  return api.json<RequestItem[]>('/api/follow-requests')
}

/**
 * ! the id here is the FOLLOWS ROW id, which arrives as follow_id on a RequestItem and as
 * ! entity_id on a follow_request notification. it is not the actor's user id, and the actor's
 * ! id sits right beside it in both shapes. mixing them up gives a 404 that reads as the
 * ! request having vanished
 */
export function decideRequest(followId: number, accept: boolean): Promise<void> {
  return api.json<void>(`/api/follow-requests/${followId}/${accept ? 'accept' : 'decline'}`, { method: 'POST' })
}
