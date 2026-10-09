import type { Comment, Post } from '@shared/contracts/posts'
import * as api from '@/lib/api/client'

/** before is a post id and zero means "from the newest". limit is clamped to 1..50 server side */
export function listFeed(before: number, limit: number): Promise<Post[]> {
  return api.json<Post[]>('/api/posts' + api.query({ before: before || undefined, limit }))
}

/**
 * ! this route hangs off a user but lives in the posts module, because the thing returned is
 * ! posts and the visibility rule deciding what comes back is the posts module's rule.
 * ! router.go has a note saying exactly that
 */
export function listByAuthor(authorId: number, before: number, limit: number): Promise<Post[]> {
  return api.json<Post[]>(`/api/users/${authorId}/posts` + api.query({ before: before || undefined, limit }))
}

export function getPost(id: number): Promise<Post> {
  return api.json<Post>(`/api/posts/${id}`)
}

export function createPost(body: FormData): Promise<Post> {
  return api.form<Post>('/api/posts', body)
}

/** the author's own post. anybody else's reads as 404, the same as an invisible one */
export function deletePost(id: number): Promise<void> {
  return api.json<void>(`/api/posts/${id}`, { method: 'DELETE' })
}

/**
 * ? no limit and no offset: this returns every comment on the post. so the design's
 * ? "load N more" is a reveal rather than a fetch, which is the rare case where the design
 * ? wanted pagination and the api made it free
 */
export function listComments(postId: number): Promise<Comment[]> {
  return api.json<Comment[]>(`/api/posts/${postId}/comments`)
}

export function createComment(postId: number, body: FormData): Promise<Comment> {
  return api.form<Comment>(`/api/posts/${postId}/comments`, body)
}
