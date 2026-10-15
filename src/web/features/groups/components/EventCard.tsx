'use client'

import { useState } from 'react'
import type { Event } from '@shared/contracts/groups'
import { dayMonth, eventWhen } from '@/lib/utils/time'
import { useToast } from '@/components/ui/Toast'

/**
 * the design's .event card, ported from eventCardHTML at line 3902.
 * ? going_count, not_going_count and my_response all come with the event, so the tally row and
 * ? the pressed state of the rsvp pair need no second call
 */
export function EventCard({
  event,
  idPrefix,
  onRespond,
  /** the group's member count, when known, so the silent tally can be derived */
  memberCount,
}: {
  event: Event & { group_title?: string }
  idPrefix: string
  onRespond: (eventId: number, response: 'going' | 'not_going') => Promise<void>
  memberCount?: number
}) {
  const [open, setOpen] = useState(false)
  const [busy, setBusy] = useState(false)
  const toast = useToast()
  const { day, mon } = dayMonth(event.event_time)

  /**
   * ! the design's third tally value has no field on the api. Event carries going and not
   * ! going only, so silent is derived from the group's member count, clamped at zero because
   * ! a member who leaves after responding would otherwise make it negative
   */
  const silent =
    memberCount === undefined ? null : Math.max(0, memberCount - event.going_count - event.not_going_count)

  const pick = async (response: 'going' | 'not_going') => {
    setBusy(true)
    try {
      await onRespond(event.id, response)
      toast(`${response === 'going' ? 'going' : 'not going'} · ${event.title}`)
    } catch {
      toast('that response did not save')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="event">
      <button
        className="event__top"
        aria-expanded={open}
        aria-controls={`${idPrefix}${event.id}`}
        onClick={() => setOpen((v) => !v)}
      >
        <span className="when">
          <b>{day}</b>
          <span>{mon}</span>
        </span>
        <span className="tx">
          <span className="nm">{event.title}</span>
          <span className="mt">
            {eventWhen(event.event_time)}
            {event.group_title ? ` · ${event.group_title}` : ''}
          </span>
        </span>
        <span
          className="caret"
          aria-hidden="true"
          style={{ fontFamily: 'var(--mono)', fontSize: 10, color: 'var(--ink-ghost)' }}
        >
          ›
        </span>
      </button>

      <div className="ds" id={`${idPrefix}${event.id}`} hidden={!open}>
        {event.description}
      </div>

      <div className="rsvp" role="group" aria-label={`Response to ${event.title}`}>
        <button
          type="button"
          aria-pressed={event.my_response === 'going'}
          disabled={busy}
          onClick={() => void pick('going')}
        >
          going
        </button>
        <button
          type="button"
          aria-pressed={event.my_response === 'not_going'}
          disabled={busy}
          onClick={() => void pick('not_going')}
        >
          not going
        </button>
      </div>

      <div className="tally">
        <span className="dot dot--on" />
        <span>{event.going_count} going</span>
        <span>·</span>
        <span>{event.not_going_count} declined</span>
        {silent !== null && (
          <>
            <span>·</span>
            <span>{silent} silent</span>
          </>
        )}
      </div>
    </div>
  )
}
