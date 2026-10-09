import type { ListUser, Profile, UpdateProfileInput } from '@shared/contracts/users'
import * as api from '@/lib/api/client'

/** answers for any id. is_self, can_view and follow_status decide which of three renders applies */
export function getProfile(id: number): Promise<Profile> {
  return api.json<Profile>(`/api/users/${id}`)
}

/**
 * ! 204 with no body, so there is no updated profile to merge — refetch after it resolves.
 * ! and send only the keys that changed: UpdateProfileInput uses pointers precisely so the
 * ! service can tell an omitted field from a deliberately cleared one, and sending all four
 * ! every time throws that distinction away
 */
export function updateMe(patch: UpdateProfileInput): Promise<void> {
  return api.json<void>('/api/users/me', { method: 'PATCH', body: patch })
}

export function setPrivacy(isPublic: boolean): Promise<void> {
  return api.json<void>('/api/users/me/privacy', { method: 'PATCH', body: { is_public: isPublic } })
}

/**
 * ? an empty q matches everyone, because the store builds LIKE '%%', and the caller is excluded
 * ? by the query itself. so browse and search are the same call with no client side filtering
 */
export function searchUsers(q: string, limit: number, offset: number): Promise<ListUser[]> {
  return api.json<ListUser[]>('/api/users' + api.query({ q, limit, offset }))
}
