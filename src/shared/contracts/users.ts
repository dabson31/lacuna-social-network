import type { FollowStatus } from '../constants/domain'

/**
 * auth.User — everything about a user except the password hash.
 * ! every pointer field in the go struct is a nullable column, so it is `| null` here. getting
 * ! one of these wrong gives a crash on the one user who skipped their nickname, which is the
 * ! least reproducible bug shape there is
 */
export interface User {
  id: number
  email: string
  first_name: string
  last_name: string
  /** "YYYY-MM-DD" calendar date, not a timestamp */
  date_of_birth: string
  avatar_path: string | null
  nickname: string | null
  about_me: string | null
  is_public: boolean
  created_at: string
}

/**
 * users.Profile — what GET /api/users/{id} returns, for anyone.
 * ! the bottom four are only filled when can_view is true. that is the privacy rule arriving
 * ! as data instead of as a rule the client has to re-derive, so never reconstruct the check
 * ! from is_public and follow_status, those two can both be true in ways that do not mean
 * ! what you would assume
 */
export interface Profile {
  id: number
  first_name: string
  last_name: string
  avatar_path: string | null
  nickname: string | null
  is_public: boolean

  follower_count: number
  following_count: number

  /** how the viewer relates to this profile, the three fields that pick the buttons */
  is_self: boolean
  follow_status: FollowStatus
  can_view: boolean

  email: string | null
  date_of_birth: string | null
  about_me: string | null
  created_at: string | null
}

/**
 * users.UpdateProfileInput.
 * ! pointers on the go side, optional keys here, and for the same reason: the service has to
 * ! tell a field the client omitted from one it deliberately cleared. send only what changed
 */
export interface UpdateProfileInput {
  first_name?: string
  last_name?: string
  nickname?: string
  about_me?: string
}

/** users.ListUser — one row of browse or search, deliberately small */
export interface ListUser {
  id: number
  first_name: string
  last_name: string
  nickname: string | null
  avatar_path: string | null
  is_public: boolean
}
