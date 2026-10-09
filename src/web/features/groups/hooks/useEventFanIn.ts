'use client'

import { useCallback, useEffect, useState } from 'react'
import type { Event } from '@shared/contracts/groups'
import { ApiError } from '@/lib/api/errors'
import { useBadges } from '@/components/layout/BadgeProvider'
import { listEvents, respondToEvent } from '../api'

/** an event plus which group raised it, since the design prints the group on every card */
export interface FannedEvent extends Event {
  group_title: string
}

/**
 * the one real fan-out in this app.
 * ! the design's Events channel is "events raised inside your groups", flattened. there is no
 * ! GET /api/events, so it is the group list and then one call per group where the caller is a
 * ! member. that is fine at this scale, a user is in maybe six groups, and what would make it
 * ! not fine is doing it per render, which is why it lives in one hook that both the page and
 * ! the right rail panel share
 */
export function useEventFanIn() {
  const { myGroups } = useBadges()
  const [events, setEvents] = useState<FannedEvent[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<ApiError | null>(null)

  // the ids are the real dependency, not the array identity, which changes on every reload
  const memberIds = myGroups
    .filter((g) => g.my_status === 'member')
    .map((g) => g.id)
    .join(',')

  const load = useCallback(async () => {
    const mine = myGroups.filter((g) => g.my_status === 'member')
    if (mine.length === 0) {
      setEvents([])
      setLoading(false)
      return
    }
    setLoading(true)
    setError(null)
    try {
      const pages = await Promise.all(
        mine.map(async (g) => (await listEvents(g.id)).map((e) => ({ ...e, group_title: g.title }))),
      )
      //? sorted here and not in css, because the design prints a day/month block per card and
      //? the panel wants whichever one is next
      const flat = pages.flat().sort((a, b) => a.event_time.localeCompare(b.event_time))
      setEvents(flat)
    } catch (e: unknown) {
      setError(e instanceof ApiError ? e : new ApiError(0, { code: 'client_error', message: String(e) }))
    } finally {
      setLoading(false)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [memberIds])

  useEffect(() => {
    void load()
  }, [load])

  /**
   * ? the counts are patched locally after an rsvp because the endpoint answers 204 and the
   * ? arithmetic is exact: one response moves one vote between two tallies
   */
  const respond = useCallback(async (eventId: number, next: 'going' | 'not_going') => {
    await respondToEvent(eventId, next)
    setEvents((prev) =>
      prev.map((e) => {
        if (e.id !== eventId) return e
        const going = e.going_count - (e.my_response === 'going' ? 1 : 0) + (next === 'going' ? 1 : 0)
        const notGoing = e.not_going_count - (e.my_response === 'not_going' ? 1 : 0) + (next === 'not_going' ? 1 : 0)
        return { ...e, going_count: going, not_going_count: notGoing, my_response: next }
      }),
    )
  }, [])

  return { events, loading, error, reload: load, respond }
}
