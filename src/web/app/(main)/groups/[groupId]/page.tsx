'use client'

import { useState } from 'react'
import Link from 'next/link'
import { useParams } from 'next/navigation'
import { MemberStatus, NotificationType } from '@shared/constants/domain'
import type { JoinRequest } from '@shared/contracts/groups'
import type { ListUser } from '@shared/contracts/users'
import { Avatar } from '@/components/ui/Avatar'
import { Empty, Loading } from '@/components/ui/Empty'
import { ErrorView } from '@/components/ui/ErrorView'
import { useToast } from '@/components/ui/Toast'
import { useFetch } from '@/lib/api/useFetch'
import { useUser } from '@/lib/session/SessionProvider'
import { useDebounced } from '@/hooks/useDebounced'
import { displayName } from '@/lib/utils/identity'
import { useBadges } from '@/components/layout/BadgeProvider'
import { searchUsers } from '@/features/profile/api'
import {
  acceptInvite,
  decideRequest,
  declineInvite,
  getGroup,
  invite,
  leaveGroup,
  listRequests,
  requestJoin,
} from '@/features/groups/api'
import { pendingNotice } from '@/features/notifications/actions'
import { markRead } from '@/features/notifications/api'
import { GroupPostFeed } from '@/features/groups/components/GroupPostFeed'
import { EventCard } from '@/features/groups/components/EventCard'
import { useEventFanIn } from '@/features/groups/hooks/useEventFanIn'

type Tab = 'posts' | 'events' | 'members' | 'requests'

export default function GroupPage() {
  const params = useParams<{ groupId: string }>()
  const groupId = Number(params.groupId)
  const me = useUser()
  const toast = useToast()
  const { reloadGroups, notifications, dropNotification } = useBadges()
  const [tab, setTab] = useState<Tab>('posts')

  const { data: group, loading, error, refetch } = useFetch(() => getGroup(groupId), [groupId])
  const { events, respond } = useEventFanIn()

  if (Number.isNaN(groupId)) return <Empty label="not available · that is not a group id" />
  if (loading) return <Loading label="reading the group…" />
  if (error) return <ErrorView error={error} onRetry={refetch} />
  if (!group) return <Empty label="not available" />

  const isMember = group.my_status === MemberStatus.Member
  const isCreator = group.my_role === 'creator'
  const groupEvents = events.filter((e) => e.group_id === groupId)

  const act = async (fn: () => Promise<unknown>, message: string) => {
    try {
      await fn()
      refetch()
      reloadGroups()
      toast(message)
    } catch {
      toast('that did not go through')
    }
  }

  /**
   * the same rule as on the groups list: whoever answers a notification marks it read, or the
   * group keeps a pending number for an invitation that has already been dealt with
   */
  const resolveInviteNotice = () => {
    const row = pendingNotice(notifications, { type: NotificationType.GroupInvite, groupId })
    if (!row) return
    dropNotification(row.id)
    void markRead(row.id).catch(() => {})
  }

  return (
    <>
      <div className="prof">
        <Avatar name={group.title} size="lg" />
        <div className="meta">
          <h2>{group.title}</h2>
          <div className="h">
            {group.member_count} {group.member_count === 1 ? 'member' : 'members'} · {groupEvents.length} events ·{' '}
            {group.join_policy === 'open' ? 'anyone may join' : 'joining needs the creator'}
          </div>
          <p>{group.description}</p>
        </div>

        <div className="side">
          <span className="lbl" style={{ display: 'block', marginBottom: 8 }}>
            Membership
          </span>
          <div className="row-ops">
            {isMember ? (
              <>
                <Link className="btn btn--sm" href={`/groups/${groupId}/chat`}>
                  group chat
                </Link>
                {/*? the creator cannot leave, there is nobody to hand the group to */}
                {!isCreator && (
                  <button
                    className="btn btn--sm btn--danger"
                    onClick={() => void act(() => leaveGroup(groupId), 'you left the group')}
                  >
                    leave
                  </button>
                )}
              </>
            ) : group.my_status === MemberStatus.Invited ? (
              <>
                <button
                  className="btn btn--sm"
                  onClick={() =>
                    void act(async () => {
                      await acceptInvite(groupId)
                      resolveInviteNotice()
                    }, 'joined')
                  }
                >
                  join
                </button>
                <button
                  className="btn btn--sm btn--danger"
                  onClick={() =>
                    void act(async () => {
                      await declineInvite(groupId)
                      resolveInviteNotice()
                    }, 'invitation declined')
                  }
                >
                  decline
                </button>
              </>
            ) : group.my_status === MemberStatus.Requested ? (
              <span className="tag tag--semi">waiting on the creator</span>
            ) : (
              <button
                className="btn btn--sm"
                onClick={() => void act(() => requestJoin(groupId), 'request sent')}
              >
                request to join
              </button>
            )}
          </div>
          {!isMember && (
            <p className="note-txt" style={{ marginTop: 10 }}>
              {/*
                ! a non member gets the header and the count and nothing else. members[] comes
                ! back empty, and the posts, events and messages routes all answer 403. that is
                ! the server deciding, not this component hiding
              */}
              Posts, events and the group room open up once you are a member.
            </p>
          )}
        </div>
      </div>

      <div className="main__bar">
        <div className="seg" role="tablist" aria-label="Group section">
          <button role="tab" aria-selected={tab === 'posts'} onClick={() => setTab('posts')}>
            posts
          </button>
          <button role="tab" aria-selected={tab === 'events'} onClick={() => setTab('events')}>
            events
          </button>
          <button role="tab" aria-selected={tab === 'members'} onClick={() => setTab('members')}>
            members
          </button>
          {isCreator && (
            <button role="tab" aria-selected={tab === 'requests'} onClick={() => setTab('requests')}>
              requests
            </button>
          )}
        </div>
        <div className="spacer" />
        {isMember && <InviteControl groupId={groupId} onInvited={() => toast('invitation sent')} />}
      </div>

      {!isMember ? (
        <Empty label="members only · join to read what is inside" />
      ) : tab === 'posts' ? (
        <GroupPostFeed groupId={groupId} selfName={displayName(me)} selfAvatar={me.avatar_path} />
      ) : tab === 'events' ? (
        <div style={{ padding: '18px 26px', display: 'flex', flexDirection: 'column', gap: 18, maxWidth: 560 }}>
          {groupEvents.map((e) => (
            <EventCard key={e.id} event={e} idPrefix="gev-" onRespond={respond} memberCount={group.member_count} />
          ))}
          {groupEvents.length === 0 && <Empty label="nothing scheduled" />}
          <Link className="btn btn--sm" href="/events" style={{ alignSelf: 'flex-start' }}>
            raise an event
          </Link>
        </div>
      ) : tab === 'members' ? (
        <div className="rows">
          {group.members.map((m) => (
            <div className="row-item" key={m.id}>
              <Link className="row-main" href={`/profile/${m.id}`}>
                <span className="dot dot--idle" />
                <Avatar name={`${m.first_name} ${m.last_name}`} path={m.avatar_path} size="sm" />
                <span className="tx">
                  <span className="nm">
                    {m.first_name} {m.last_name}
                    {m.role === 'creator' && <span className="tag tag--priv">creator</span>}
                  </span>
                </span>
                <span className="caret" aria-hidden="true">
                  ›
                </span>
              </Link>
            </div>
          ))}
          {group.members.length === 0 && <Empty label="no members listed" />}
        </div>
      ) : (
        <RequestQueue groupId={groupId} onDecided={() => { refetch(); reloadGroups() }} />
      )}
    </>
  )
}

/**
 * ! any member may invite, not just the creator. "They can also invite other people once they
 * ! are already part of the group" is in the subject and the service implements it, so this is
 * ! never gated on my_role
 * ? the picker reuses the directory's search, because GET /api/users?q= is the only user search
 * ? there is and inventing a "people I follow" endpoint would be a backend change
 */
function InviteControl({ groupId, onInvited }: { groupId: number; onInvited: () => void }) {
  const [open, setOpen] = useState(false)
  const [raw, setRaw] = useState('')
  const q = useDebounced(raw, 250)
  const { data } = useFetch<ListUser[]>(() => (open && q ? searchUsers(q, 8, 0) : Promise.resolve([])), [open, q])

  return (
    <>
      <button className="btn btn--sm" onClick={() => setOpen((v) => !v)}>
        {open ? 'close' : 'invite someone'}
      </button>
      {open && (
        <div className="row-detail" hidden={false} style={{ position: 'absolute', right: 26, top: 90, zIndex: 5 }}>
          <div className="pad panel" style={{ minWidth: 260 }}>
            <label className="search" style={{ marginBottom: 8 }}>
              <span className="lbl">find</span>
              <input autoFocus value={raw} onChange={(e) => setRaw(e.target.value)} placeholder="name or nickname" />
            </label>
            {(data ?? []).map((u) => (
              <button
                key={u.id}
                className="btn btn--sm"
                style={{ display: 'block', width: '100%', marginBottom: 4 }}
                onClick={async () => {
                  try {
                    await invite(groupId, u.id)
                    setOpen(false)
                    setRaw('')
                    onInvited()
                  } catch {
                    /* a 409 means they are already in or already invited, which the toast says */
                    onInvited()
                  }
                }}
              >
                {displayName(u)}
              </button>
            ))}
            {q && (data ?? []).length === 0 && <p className="note-txt">no account matches</p>}
          </div>
        </div>
      )}
    </>
  )
}

/** the creator's queue. a 403 for anybody else, which ErrorView renders as the red bar */
function RequestQueue({ groupId, onDecided }: { groupId: number; onDecided: () => void }) {
  const { data, loading, error, refetch } = useFetch<JoinRequest[]>(() => listRequests(groupId), [groupId])
  const { notifications, dropNotification } = useBadges()
  const toast = useToast()

  /**
   * ! the asking user is part of the match and not optional. one group can have several people
   * ! waiting, each with its own notification, so matching on the group alone would mark a
   * ! different request read than the one just decided
   */
  const resolveRequestNotice = (userId: number) => {
    const row = pendingNotice(notifications, {
      type: NotificationType.GroupJoinRequest,
      groupId,
      actorId: userId,
    })
    if (!row) return
    dropNotification(row.id)
    void markRead(row.id).catch(() => {})
  }

  if (loading) return <Loading />
  if (error) return <ErrorView error={error} onRetry={refetch} />

  const requests = data ?? []
  if (requests.length === 0) return <Empty label="nobody is waiting to get in" />

  return (
    <div className="rows">
      {requests.map((r) => (
        <div className="row-item" key={r.user_id}>
          <div className="row-main" style={{ cursor: 'default' }}>
            <span className="dot dot--on" />
            <Avatar name={`${r.first_name} ${r.last_name}`} path={r.avatar_path} size="sm" />
            <span className="tx">
              <span className="nm">
                {r.first_name} {r.last_name}
              </span>
              <span className="mt">asked to join</span>
            </span>
            <div className="row-ops">
              {/*! two ids: the group and the asking user. the same pair arrives on a
                  group_join_request notification as entity_id and actor_id */}
              <button
                className="btn btn--sm"
                onClick={async () => {
                  await decideRequest(groupId, r.user_id, true)
                  resolveRequestNotice(r.user_id)
                  refetch()
                  onDecided()
                  toast('admitted')
                }}
              >
                admit
              </button>
              <button
                className="btn btn--sm btn--danger"
                onClick={async () => {
                  await decideRequest(groupId, r.user_id, false)
                  resolveRequestNotice(r.user_id)
                  refetch()
                  onDecided()
                  toast('refused')
                }}
              >
                refuse
              </button>
            </div>
          </div>
        </div>
      ))}
    </div>
  )
}
