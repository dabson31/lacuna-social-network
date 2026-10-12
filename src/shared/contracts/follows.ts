/**
 * follows.RequestItem — one incoming follow request.
 * ! follow_id is the follows row id and it is what accept and decline take. the actor's user
 * ! id is sitting right next to it, and a notification row calls this same number entity_id.
 * ! that is the most confusable pair of ids in the whole api
 */
export interface RequestItem {
  follow_id: number
  actor_id: number
  first_name: string
  last_name: string
  avatar_path: string | null
  created_at: string
}

/** follows.UserCard — one entry in a followers or following list */
export interface UserCard {
  id: number
  first_name: string
  last_name: string
  nickname: string | null
  avatar_path: string | null
  is_public: boolean
}

/**
 * follows.FollowResult.
 * ? the status comes from the server because the bypass rule is decided there: a public target
 * ? auto accepts, a private one goes pending. guessing it on the client is wrong on every
 * ? public account
 */
export interface FollowResult {
  status: 'pending' | 'accepted'
}
