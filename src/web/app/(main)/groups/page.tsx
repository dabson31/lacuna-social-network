'use client'

import { useRef, useState } from 'react'
import Link from 'next/link'
import { MemberStatus, NotificationType } from '@shared/constants/domain'
import type { Group } from '@shared/contracts/groups'
import { Avatar } from '@/components/ui/Avatar'
import { Empty } from '@/components/ui/Empty'
import { useToast } from '@/components/ui/Toast'
import { EmojiPicker } from '@/components/ui/EmojiPicker'
import { useBadges } from '@/components/layout/BadgeProvider'
import { acceptInvite, createGroup, declineInvite, requestJoin } from '@/features/groups/api'
import { pendingNotice } from '@/features/notifications/actions'
import { markRead } from '@/features/notifications/api'

/**
 * browse every group, ported from viewGroups at line 3847.
 * ! my_status and my_role arrive on every row, which is what makes this one request rather
 * ! than one plus N. the button comes from those two fields and never from a membership probe
 */
export default function GroupsPage() {
  const {
    myGroups,
    reloadGroups,
    inbox,
    groupPending,
    groupPendingTotal,
    unreadPosts,
    notifications,
    dropNotification,
  } = useBadges()
  const toast = useToast()
  const [open, setOpen] = useState<number | null>(null)
  const [composing, setComposing] = useState(false)
  const [busy, setBusy] = useState<number | null>(null)
  //! this form is read with `new FormData(e.currentTarget)` and has no value/onChange, so the
  //! node IS the state. the picker writes straight to it and fires an input event, which is the
  //! branch it takes when no onInsert is given
  const gTitle = useRef<HTMLInputElement>(null)
  const gDesc = useRef<HTMLTextAreaElement>(null)

  const act = async (g: Group, fn: () => Promise<unknown>, message: string) => {
    setBusy(g.id)
    try {
      await fn()
      reloadGroups()
      toast(message)
    } catch {
      toast('that did not go through')
    } finally {
      setBusy(null)
    }
  }

  /**
   * answering the invite here answers the one the notices panel is holding, so the row gets
   * marked read from here too.
   * ! without this the group keeps a pending number for an invitation already accepted, and
   * ! the panel keeps offering a join that would now answer 409
   * ? dropped locally first and marked on the server second, for the same reason
   * ? resolveNotification does it in that order: the action is the point and the read flag is
   * ? bookkeeping, so a failed mark must not look like a failed join
   */
  const resolveInviteNotice = (groupId: number) => {
    const row = pendingNotice(notifications, { type: NotificationType.GroupInvite, groupId })
    if (!row) return
    dropNotification(row.id)
    void markRead(row.id).catch(() => {
      /* the join already landed, and read-all will catch the row */
    })
  }

  const make = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    const form = new FormData(e.currentTarget)
    try {
      //! no join_policy in the body. the service defaults it to 'request', which is the flow
      //! the audit checks, and the design's form has no field for it. the column is an extra
      //! and the migration says so in its own comment
      await createGroup({
        title: String(form.get('title') ?? ''),
        description: String(form.get('description') ?? ''),
      })
      setComposing(false)
      reloadGroups()
      toast('group created · you are its creator')
    } catch {
      toast('that group could not be created')
    }
  }

  /**
   * how many messages are waiting in this group's room.
   * ! GET /api/conversations has always carried this per room, counted off
   * ! group_members.last_read_message_id. nothing on this page read it, so a group with
   * ! unread messages looked identical to a quiet one and the only place the number appeared
   * ! was /chat
   * ? undefined for a group the caller is not a member of: there is no room to be behind in,
   * ? which is also why inbox.groups only lists the ones they are in
   */
  const unreadIn = (groupId: number): number =>
    inbox?.groups.find((g) => g.group_id === groupId)?.unread ?? 0

  /**
   * how many notifications are still waiting on the reader in this group.
   * ! a different number from unreadIn, and deliberately shown as a different thing. the
   * ! subject requires notifications and private messages to be distinguishable, so an invite,
   * ! a join request and a new event count here while room messages count there
   * ? counted by the server per group, because entity_id means a different table per kind and
   * ? the client has nothing to resolve an event id to a group with
   */
  const pendingIn = (groupId: number): number =>
    groupPending.find((g) => g.group_id === groupId)?.unread ?? 0

  /** the four membership states, each with the one action that applies */
  const actionsFor = (g: Group) => {
    const posts = g.unread_posts ?? 0
    switch (g.my_status) {
      case MemberStatus.Member:
        return (
          <>
            <Link className="btn btn--sm" href={`/groups/${g.id}`}>
              open group{posts > 0 ? ` · ${posts} new` : ''}
            </Link>
            {/*
              ! the one affordance this build adds rather than ports. the design has no way to
              ! reach a group room, but the backend has them fully built and the subject is
              ! explicit that a member must be able to send and receive there. same classes as
              ! the other row actions, so it still looks like the mockup
            */}
            <Link className="btn btn--sm" href={`/groups/${g.id}/chat`}>
              group chat{unreadIn(g.id) > 0 ? ` · ${unreadIn(g.id)} new` : ''}
            </Link>
          </>
        )
      case MemberStatus.Invited:
        return (
          <>
            <button
              className="btn btn--sm"
              disabled={busy === g.id}
              onClick={() =>
                void act(
                  g,
                  async () => {
                    await acceptInvite(g.id)
                    resolveInviteNotice(g.id)
                  },
                  `joined · ${g.title}`,
                )
              }
            >
              join
            </button>
            <button
              className="btn btn--sm btn--danger"
              disabled={busy === g.id}
              onClick={() =>
                void act(
                  g,
                  async () => {
                    await declineInvite(g.id)
                    resolveInviteNotice(g.id)
                  },
                  'invitation declined',
                )
              }
            >
              decline
            </button>
          </>
        )
      case MemberStatus.Requested:
        //? there is no cancel-a-join-request route and the design has no such control either,
        //? so this renders as a disabled tag rather than a button that would 404
        return <span className="tag tag--semi">waiting on the creator</span>
      default:
        return (
          <button
            className="btn btn--sm"
            disabled={busy === g.id}
            onClick={() =>
              void act(g, async () => {
                const { status } = await requestJoin(g.id)
                //? an open group answers "member" straight away, a request-policy one answers
                //? "requested". the message comes from the response rather than a guess
                toast(status === 'member' ? `joined · ${g.title}` : 'request sent to the creator')
              }, '')
            }
          >
            request to join
          </button>
        )
    }
  }

  const roleTag = (g: Group) => {
    if (g.my_role === 'creator') return <span className="tag tag--priv">creator</span>
    if (g.my_status === MemberStatus.Member) return <span className="tag">member</span>
    if (g.my_status === MemberStatus.Invited) return <span className="tag tag--semi">invited</span>
    if (g.my_status === MemberStatus.Requested) return <span className="tag tag--semi">requested</span>
    return <span className="tag">not a member</span>
  }

  return (
    <>
      <div className="main__bar">
        <span className="lbl">
          {myGroups.length} groups
          {groupPendingTotal > 0 && ` · ${groupPendingTotal} pending`}
          {unreadPosts > 0 && ` · ${unreadPosts} new ${unreadPosts === 1 ? 'post' : 'posts'}`}
        </span>
        <div className="spacer" />
        <button className="btn btn--sm" onClick={() => setComposing((v) => !v)}>
          {composing ? 'close' : 'create a group'}
        </button>
      </div>

      {composing && (
        <form className="authform" onSubmit={make} style={{ padding: '18px 26px', maxWidth: 520 }}>
          <label className="field">
            <span className="lbl">title</span>
            <input ref={gTitle} name="title" required maxLength={120} />
            {/*? down, because this field sits near the top of the page and an upward tray would
                open above the viewport */}
            <EmojiPicker target={gTitle} placement="down" />
          </label>
          <label className="field" style={{ marginBottom: 14 }}>
            <span className="lbl">description</span>
            <textarea ref={gDesc} name="description" rows={3} required />
            <EmojiPicker target={gDesc} placement="down" />
          </label>
          <button className="btn btn--sm btn--primary" type="submit">
            create it
          </button>
        </form>
      )}

      <div className="rows">
        {myGroups.map((g) => {
          const isOpen = open === g.id
          const unread = unreadIn(g.id)
          const pending = pendingIn(g.id)
          //? three different kinds of waiting, three different words. they are counted by
          //? three different mechanisms and the subject requires at least two of them to stay
          //? distinguishable, so they are never added together on a row
          const posts = g.unread_posts ?? 0
          return (
            <div className="row-item" key={g.id}>
              <button
                className="row-main"
                aria-expanded={isOpen}
                aria-controls={`grp-${g.id}`}
                onClick={() => setOpen(isOpen ? null : g.id)}
              >
                {/*? the dot the rest of the app uses for the same thing, lit per group, by
                    either kind of waiting thing */}
                <span className={unread || pending || posts ? 'dot dot--on' : 'dot dot--idle'} />
                <Avatar name={g.title} size="sm" />
                <span className="tx">
                  <span className="nm">
                    {g.title}
                    {roleTag(g)}
                    {/*? .row-unread is the stylesheet's own class for exactly this, already
                        used by the dm and group rows on /chat */}
                    {pending > 0 && <span className="row-unread">{pending} pending</span>}
                    {posts > 0 && (
                      <span className="row-unread">
                        {posts} new {posts === 1 ? 'post' : 'posts'}
                      </span>
                    )}
                    {unread > 0 && (
                      <span className="row-unread">
                        {unread} new {unread === 1 ? 'message' : 'messages'}
                      </span>
                    )}
                  </span>
                  <span className="mt">
                    {g.member_count} {g.member_count === 1 ? 'member' : 'members'}
                  </span>
                </span>
                <span className="caret" aria-hidden="true">
                  ›
                </span>
              </button>

              <div className="row-detail" id={`grp-${g.id}`} hidden={!isOpen}>
                <div className="pad">
                  <p className="note-txt">{g.description}</p>
                  <div className="row-ops">{actionsFor(g)}</div>
                </div>
              </div>
            </div>
          )
        })}

        {myGroups.length === 0 && <Empty label="no groups on this node yet" />}
      </div>
    </>
  )
}
