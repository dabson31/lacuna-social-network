/**
 * the three time formats the design actually prints, and nothing else.
 * ? the api returns RFC 3339 everywhere. the mockup prints "14 min" on a post, "02:41:08" on a
 * ? notice and "yesterday" on an older one, so all three come from here
 */

const MINUTE = 60
const HOUR = 60 * MINUTE
const DAY = 24 * HOUR

/** "now" · "14 min" · "3 h" · "2 d" · then a date. the feed card's age line */
export function age(iso: string): string {
  const then = Date.parse(iso)
  if (Number.isNaN(then)) return ''
  const secs = Math.max(0, Math.floor((Date.now() - then) / 1000))
  if (secs < 45) return 'now'
  if (secs < HOUR) return `${Math.floor(secs / MINUTE)} min`
  if (secs < DAY) return `${Math.floor(secs / HOUR)} h`
  if (secs < 7 * DAY) return `${Math.floor(secs / DAY)} d`
  return new Date(then).toLocaleDateString(undefined, { day: 'numeric', month: 'short' })
}

/** "02:41:08", the notice timestamp, local time */
export function clockTime(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return [d.getHours(), d.getMinutes(), d.getSeconds()].map((n) => String(n).padStart(2, '0')).join(':')
}

/** "14:05", the chat bubble header */
export function shortTime(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

/** the event card's .when block wants a bare day number and a three letter month */
export function dayMonth(iso: string): { day: string; mon: string } {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return { day: '--', mon: '---' }
  return {
    day: String(d.getDate()),
    mon: d.toLocaleDateString(undefined, { month: 'short' }).toLowerCase(),
  }
}

/** "19 oct, 23:30", the event card's metadata line */
export function eventWhen(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  const date = d.toLocaleDateString(undefined, { day: 'numeric', month: 'short' }).toLowerCase()
  return `${date}, ${shortTime(iso)}`
}

/**
 * a datetime-local input gives "2026-12-25T18:00" with no zone, and the service parses
 * event_time as RFC 3339.
 * ! without the seconds and the offset time.Parse rejects it and the field comes back
 * ! "event time must be RFC 3339", which reads like the user typed something wrong when the
 * ! input gave them no way to type anything else
 */
export function toRFC3339(localValue: string): string {
  const d = new Date(localValue)
  return Number.isNaN(d.getTime()) ? localValue : d.toISOString()
}

/** "2026-10-09", what the date_of_birth column holds and what <input type="date"> produces */
export function dateOnly(iso: string | null): string {
  if (!iso) return ''
  return iso.slice(0, 10)
}
