import type { User } from '@shared/contracts/users'
import * as api from '@/lib/api/client'

/**
 * ! register is multipart and login is json. that is not an inconsistency: register carries an
 * ! optional avatar file and login never will, and DecodeJSON on the login side runs
 * ! DisallowUnknownFields so a stray extra key is a 400 rather than a silently ignored field
 */
export function register(body: FormData): Promise<User> {
  return api.form<User>('/api/auth/register', body)
}

export function login(email: string, password: string): Promise<User> {
  return api.json<User>('/api/auth/login', { method: 'POST', body: { email, password } })
}

export function logout(): Promise<void> {
  return api.json<void>('/api/auth/logout', { method: 'POST' })
}

export function me(): Promise<User> {
  return api.json<User>('/api/auth/me')
}
