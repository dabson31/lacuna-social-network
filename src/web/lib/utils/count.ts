/**
 * content limits are counted in runes on the server, never in bytes.
 * ! chat/service.go spells it out: counted in runes so an emoji costs one character instead
 * ! of four. a byte limit would cut a message of 500 emojis off halfway through a character
 * ! and leave broken utf8 in the db. so a client side counter has to agree, or it rejects
 * ! messages the server would happily take
 */
export function runeLength(s: string): number {
  return [...s].length
}

/** the server's own caps, so an input can show a counter that matches what will be accepted */
export const LIMITS = {
  post: 5000,
  comment: 2000,
  message: 2000,
  aboutMe: 500,
  nickname: 30,
  name: 50,
} as const
