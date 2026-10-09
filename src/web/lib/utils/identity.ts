/**
 * the design's unit of identity is @handle. the database's is users.id, an integer, with a
 * nullable nickname in between.
 * ! this is the only place allowed to paper over that gap, so when the rule changes it changes
 * ! once instead of in forty components. routes and request bodies always use the numeric id
 */

export interface Named {
  nickname?: string | null
  first_name: string
  last_name: string
}

/** the name that goes in a heading or a card's primary line */
export function displayName(u: Named): string {
  return u.nickname?.trim() || `${u.first_name} ${u.last_name}`.trim()
}

/**
 * the @form the design prints in metadata lines.
 * ? only prefixed when a nickname actually exists, because "@Mika Kurosawa" with a space in
 * ? it reads as a bug, and inventing "@m.kurosawa" would match no row in the database
 */
export function handle(u: Named): string {
  return u.nickname?.trim() ? `@${u.nickname.trim()}` : displayName(u)
}

/**
 * the design's .av block is initials in a machined square. two letters, from the parts of a
 * name, falling back to the two middle dots the mockup uses
 */
export function initials(name: string): string {
  const letters = name
    .replace(/[^a-zA-Z._ ]/g, '')
    .split(/[\s._]+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((w) => w[0]!.toUpperCase())
    .join('')
  return letters || '··'
}

/** the server joins first and last into one author_name, so split it back for initials */
export function initialsOf(u: Named | string): string {
  return initials(typeof u === 'string' ? u : displayName(u))
}
