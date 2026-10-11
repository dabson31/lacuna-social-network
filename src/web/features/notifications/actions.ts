import { NotificationType } from '@shared/constants/domain'
import type { NotificationRow } from '@shared/contracts/notifications'
import * as api from '@/lib/api/client'
import { markRead } from './api'

/**
 * the four kinds and the four different endpoints behind them.
 * ! entity_id is what makes this work with no extra lookups, and it means something different
 * ! per kind. verified against the four publishers:
 * !   follow_request      → follows row id   (follows/service.go:46)
 * !   group_invite        → group id         (groups/service_members.go:41)
 * !   group_join_request  → group id         (groups/service_members.go:86)
 * !   event_created       → event id         (groups/service.go:213)
 * ! the join request case needs BOTH ids, the group from entity_id and the asking user from
 * ! actor_id, because the route is /groups/{gid}/requests/{uid}/accept. that is the only
 * ! two-value case and the one most likely to be written wrong
 */
export async function resolveNotification(n: NotificationRow, accept: boolean): Promise<void> {
  const verdict = accept ? 'accept' : 'decline'

  switch (n.type) {
    case NotificationType.FollowRequest:
      await api.json(`/api/follow-requests/${n.entity_id}/${verdict}`, { method: 'POST' })
      break

    case NotificationType.GroupInvite:
      await api.json(`/api/groups/${n.entity_id}/invitations/${verdict}`, { method: 'POST' })
      break

    case NotificationType.GroupJoinRequest:
      await api.json(`/api/groups/${n.entity_id}/requests/${n.actor_id}/${verdict}`, { method: 'POST' })
      break

    case NotificationType.EventCreated:
      await api.json(`/api/events/${n.entity_id}/respond`, {
        method: 'POST',
        body: { response: accept ? 'going' : 'not_going' },
      })
      break
  }

  //! the action first and the read state second, in that order. the action is the point and
  //! the read flag is bookkeeping, so a failed mark-read must never look like a failed accept
  try {
    await markRead(n.id)
  } catch {
    /* the accept already landed, and the row will be marked read on the next read-all */
  }
}

/**
 * the design's per-kind verbs, from NOTICES at line 3074.
 * ? deliberately not normalised to accept/decline: the mockup says join for an invite and
 * ? admit/refuse for a join request, and that wording is part of the design
 */
export function verbsFor(type: NotificationRow['type']): { accept: string; decline: string } {
  switch (type) {
    case NotificationType.FollowRequest:
      return { accept: 'accept', decline: 'decline' }
    case NotificationType.GroupInvite:
      return { accept: 'join', decline: 'decline' }
    case NotificationType.GroupJoinRequest:
      return { accept: 'admit', decline: 'refuse' }
    case NotificationType.EventCreated:
      return { accept: 'going', decline: 'not going' }
  }
}

/**
 * the sentence in the expanded row.
 * ? the mockup had a hand written detail per notice. the api gives the actor and the kind, so
 * ? the sentence is built from those two rather than invented
 */
export function describeNotification(n: NotificationRow): string {
  switch (n.type) {
    case NotificationType.FollowRequest:
      return `${n.actor_name} asked to follow you. Accepting lets them read your posts and start a conversation.`
    case NotificationType.GroupInvite:
      return `${n.actor_name} invited you into a group. Joining lets you read and post inside it.`
    case NotificationType.GroupJoinRequest:
      return `${n.actor_name} asked to join a group you created. You decide who gets in.`
    case NotificationType.EventCreated:
      return `${n.actor_name} raised an event in one of your groups. Answer going or not going.`
  }
}

/**
 * the unresolved notification that an action taken somewhere else has just answered.
 * ! a group notification has entry points the notices panel knows nothing about: an invite is
 * ! answered from the groups list or the group page, a join request from the group's own queue,
 * ! an rsvp from the events channel. the row stays unread through all of them, and an unread
 * ! row still counts — which is how a group goes on showing a pending number for something
 * ! already dealt with
 * ? a follow request needs none of this, it has exactly one place it can be answered
 * ! so the rule is: whoever performs the action marks the row, exactly as resolveNotification
 * ! does for the panel. this finds the row to mark
 * ? matched on the fields that identify the subject rather than on entity_id alone, because
 * ? entity_id means a different table per kind — a group for an invite, an event for an rsvp —
 * ? and a join request needs the asking user too, since one group can have several waiting
 */
export function pendingNotice(
  rows: NotificationRow[],
  match: { type: NotificationRow['type']; groupId?: number; entityId?: number; actorId?: number },
): NotificationRow | undefined {
  return rows.find(
    (n) =>
      n.read_at === null &&
      n.type === match.type &&
      (match.groupId === undefined || n.group_id === match.groupId) &&
      (match.entityId === undefined || n.entity_id === match.entityId) &&
      (match.actorId === undefined || n.actor_id === match.actorId),
  )
}

/** where a notification points, for the "open" affordance on the notices page */
export function targetHref(n: NotificationRow): string | null {
  switch (n.type) {
    case NotificationType.FollowRequest:
      return `/profile/${n.actor_id}`
    case NotificationType.GroupInvite:
    case NotificationType.GroupJoinRequest:
      return `/groups/${n.entity_id}`
    case NotificationType.EventCreated:
      return '/events'
  }
}
