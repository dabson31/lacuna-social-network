'use client'

import { useMemo } from 'react'
import { EventCard } from '@/features/groups/components/EventCard'
import { useEventFanIn } from '@/features/groups/hooks/useEventFanIn'

/**
 * the next event, in the right rail on every route.
 * ! this shares the one fan-in hook with the Events page rather than running its own. there is
 * ! no GET /api/events, so the list costs one call per group, and a naive panel would repeat
 * ! that on every single navigation
 */
export function NextEventPanel() {
  const { events, respond, loading } = useEventFanIn()

  const next = useMemo(() => {
    const now = Date.now()
    const upcoming = events.filter((e) => Date.parse(e.event_time) >= now)
    return (upcoming[0] ?? events[0]) ?? null
  }, [events])

  return (
    <section className="sec" aria-labelledby="evtHd">
      <div className="sec__hd">
        <span className="lbl" id="evtHd">
          Next event
        </span>
        <div className="spacer" />
        <span className="lbl">{next ? next.group_title : loading ? 'reading…' : 'none'}</span>
      </div>
      {next ? (
        <EventCard event={next} idPrefix="rv-" onRespond={respond} />
      ) : (
        <p className="note-txt">Nothing scheduled in your groups. Any member can raise one.</p>
      )}
    </section>
  )
}
