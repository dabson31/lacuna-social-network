'use client'

import { useState } from 'react'
import Link from 'next/link'
import { Empty, Loading } from '@/components/ui/Empty'
import { ErrorView } from '@/components/ui/ErrorView'
import { useToast } from '@/components/ui/Toast'
import { useBadges } from '@/components/layout/BadgeProvider'
import { fieldErrors } from '@/lib/api/errors'
import { localDayStart, toRFC3339 } from '@/lib/utils/time'
import { EventCard } from '@/features/groups/components/EventCard'
import { useEventFanIn } from '@/features/groups/hooks/useEventFanIn'
import { createEvent } from '@/features/groups/api'

/**
 * the Events channel: every event across the groups the caller is a member of.
 * ! there is no GET /api/events, so this is the one genuine fan-out in the app. the hook does
 * ! it once and the right rail's Next event panel reads the same hook rather than repeating it
 */
export default function EventsPage() {
  const { myGroups } = useBadges()
  const { events, loading, error, reload, respond } = useEventFanIn()
  const toast = useToast()
  const [composing, setComposing] = useState(false)

  const mine = myGroups.filter((g) => g.my_status === 'member')
  const memberCountOf = (groupId: number) => myGroups.find((g) => g.id === groupId)?.member_count

  const raise = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    const form = new FormData(e.currentTarget)
    const groupId = Number(form.get('group_id'))
    try {
      await createEvent(groupId, {
        title: String(form.get('title') ?? ''),
        description: String(form.get('description') ?? ''),
        //! a datetime-local input gives "2026-12-25T18:00" with no seconds and no offset, and
        //! the service parses event_time as RFC 3339. without the conversion the field comes
        //! back rejected, which reads as the user typing something wrong when the input gave
        //! them no other option
        event_time: toRFC3339(String(form.get('event_time') ?? '')),
      })
      setComposing(false)
      reload()
      toast('event raised · every member was notified')
    } catch (err: unknown) {
      /*
        ! the generic "that event did not save" is what this replaces. the service answers 400
        ! with one entry per field it rejected, and throwing that away left the only possible
        ! reading of a failed submit "something, somewhere, is wrong". title, description and
        ! event_time are all required server side, so a missing one has to be nameable
        ? the `required` attributes below are the first line of defence and the browser's own
        ? popup handles them. this is the second: a field the browser cannot check, and
        ? anything the service rejects for a reason the form does not know about
      */
      const fields = fieldErrors(err)
      //? the service's own messages already name their field — "description is required",
      //? "event time must be an RFC 3339 timestamp" — so they are printed as written rather
      //? than prefixed with the key, which would read "description: description is required"
      const named = Object.values(fields ?? {}).join(' · ')
      //? one prefix for every reason the service can refuse, because the list now includes a
      //? date in the past, and "incomplete" is the wrong word for a field that was filled in
      toast(named ? `that event did not save · ${named}` : 'that event did not save')
    }
  }

  return (
    <>
      <div className="main__bar">
        <span className="lbl">{events.length} upcoming</span>
        <div className="spacer" />
        {mine.length > 0 && (
          <button className="btn btn--sm" onClick={() => setComposing((v) => !v)}>
            {composing ? 'close' : 'raise an event'}
          </button>
        )}
      </div>

      {composing && mine.length > 0 && (
        <form className="authform" onSubmit={raise} style={{ padding: '18px 26px', maxWidth: 560 }}>
          <label className="field">
            <span className="lbl">group</span>
            <select name="group_id" required>
              {mine.map((g) => (
                <option key={g.id} value={g.id}>
                  {g.title}
                </option>
              ))}
            </select>
          </label>
          <label className="field">
            <span className="lbl">title</span>
            <input name="title" required maxLength={120} />
          </label>
          {/*
            ! required, because groups.Service.CreateEvent rejects an empty description. without
            ! it the form posted, the service answered 400 and the submit looked like it simply
            ! did nothing. the audit asks for "a title, a description, a day/time", so all three
            ! are required on both sides
          */}
          <label className="field">
            <span className="lbl">description</span>
            <textarea name="description" rows={3} required />
          </label>
          {/*
            ! min is midnight today, so the picker will not offer a past day and the browser
            ! refuses one typed in by hand. the service checks the same floor, because a form
            ! attribute is a convenience and not a rule — anything posted straight at the api
            ! never passed through it
            ? min is read once per render rather than held in state. the only way it goes stale
            ? is a form left open across midnight, and the service is still behind it
          */}
          <label className="field" style={{ marginBottom: 14 }}>
            <span className="lbl">day and time</span>
            <input type="datetime-local" name="event_time" min={localDayStart()} required />
          </label>
          <button className="btn btn--sm btn--primary" type="submit">
            raise it
          </button>
        </form>
      )}

      {error && <ErrorView error={error} onRetry={reload} />}
      {loading && <Loading label="reading your groups…" />}

      {!loading && (
        <div style={{ padding: '18px 26px', display: 'flex', flexDirection: 'column', gap: 18, maxWidth: 560 }}>
          {events.map((e) => (
            <EventCard key={e.id} event={e} idPrefix="ev-" onRespond={respond} memberCount={memberCountOf(e.group_id)} />
          ))}

          {events.length === 0 && mine.length === 0 && (
            <Empty label="join a group first, events are raised inside them" />
          )}
          {events.length === 0 && mine.length > 0 && <Empty label="nothing scheduled in your groups" />}
        </div>
      )}

      {!loading && mine.length === 0 && (
        <div style={{ padding: '0 26px 18px' }}>
          <Link className="btn btn--sm" href="/groups">
            browse groups
          </Link>
        </div>
      )}
    </>
  )
}
