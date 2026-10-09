import type { Privacy } from '../constants/domain'

/**
 * posts.Post.
 * ? it carries author_name, author_avatar and comment_count so a feed of twenty cards is one
 * ? request. do not add a profile fetch per card to get a name the row already has
 */
export interface Post {
  id: number
  author_id: number
  content: string
  image_path: string | null
  privacy: Privacy
  created_at: string
  author_name: string
  author_avatar: string | null
  comment_count: number
  /** only meaningful on a private post, and only the author ever sees a non-zero value */
  viewer_count: number
}

/** posts.Comment — same shape as a post minus privacy, because a comment inherits the post's */
export interface Comment {
  id: number
  post_id: number
  author_id: number
  content: string
  image_path: string | null
  created_at: string
  author_name: string
  author_avatar: string | null
}
