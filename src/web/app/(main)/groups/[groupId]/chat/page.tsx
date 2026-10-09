'use client'

import { useCallback, useEffect } from 'react'
import Link from 'next/link'
import { useParams } from 'next/navigation'
import { WsFrame } from '@shared/constants/ws'
import { Empty, Loading } from '@/components/ui/Empty'
import { ErrorView } from '@/components/ui/ErrorView'
import { useBadges } from '@/components/layout/BadgeProvider'
import { useUser } from '@/lib/session/SessionProvider'
import { useFrame } from '@/lib/realtime/SocketProvider'
import { ChatLog, type Bubble } from '@/features/chat/components/ChatLog'
import { ChatComposer } from '@/features/chat/components/Composer'
import { useThread } from '@/features/chat/hooks/useThread'
import { groupHistory, markGroupRead, sendGroupMessage } from '@/features/chat/api'

/**
 * the group room. the backend has had this the whole time — the routes, the group.message
 * frame, the watermark and inbox.groups — and the design had no way to reach any of it.
 * ! the subject is explicit: "Groups should have a common chat room, so if a user is a member
 * ! of the group he/she should be able to send and receive messages to this group chat". so
 * ! this route and the two links to it are the one functional addition in the build
 *
 * ? it lives under /groups/{id}/chat rather than a second [conversationId] segment because the
 * ? endpoint pair is different and the permission check is different: membership, not a follow
 */
export default function GroupChatPage() {
  const params = useParams<{ groupId: string }>()
  const groupId = Number(params.groupId)
  const me = useUser()
  const { myGroups, reloadInbox } = useBadges()

  const thread = useThread((before) => groupHistory(groupId, before), [groupId])
  const group = myGroups.find((g) => g.id === groupId)

  const markRead = useCallback(() => {
    void markGroupRead(groupId)
      .then(reloadInbox)
      .catch(() => {})
  }, [groupId, reloadInbox])

  useEffect(() => {
    if (!thread.loading) markRead()
  }, [thread.loading, markRead])

  /**
   * ! a group.message frame reaches every member including the sender, so the 201 body and the
   * ! frame are the same row arriving twice. append dedupes by id, which is why that is
   * ! harmless rather than a double bubble
   */
  useFrame(
    WsFrame.GroupMessage,
    (m) => {
      if (m.group_id !== groupId) return
      thread.append(m)
      if (m.sender_id !== me.id) markRead()
    },
    [groupId, thread.append, markRead, me.id],
  )

  if (Number.isNaN(groupId)) return <Empty label="not available · that is not a group id" />

  const bubbles: Bubble[] = thread.messages.map((m) => ({
    id: m.id,
    mine: m.sender_id === me.id,
    who: m.sender_name,
    at: m.created_at,
    content: m.content,
  }))

  return (
    <>
      <div className="main__bar">
        <Link className="btn btn--sm" href="/chat">
          ← all
        </Link>
        <span className="dot dot--on" />
        <span className="lbl">{group?.title ?? `group ${groupId}`}</span>
        <div className="spacer" />
        <Link className="btn btn--sm" href={`/groups/${groupId}`}>
          group
        </Link>
      </div>

      {/* a non member gets 403 chat_not_allowed, which ErrorView draws as the red bar in place */}
      {thread.error && <ErrorView error={thread.error} onRetry={thread.reload} />}

      {thread.loading ? (
        <Loading label="reading the room…" />
      ) : (
        <div className="chat">
          <ChatLog
            bubbles={bubbles}
            onReachTop={thread.loadOlder}
            loadingOlder={thread.loadingOlder}
            exhausted={thread.exhausted}
          />
          {/*
            ? no typing indicator in a group room. dm.typing is addressed to one peer and there
            ? is no group equivalent frame, so the slot stays empty rather than being faked
            ? the same goes for read ticks: group read state is one watermark per member,
            ? group_members.last_read_message_id, not a row per member per message
          */}
          <div className="chat__typing" />
          <ChatComposer
            placeholder={`message ${group?.title ?? 'the group'}`}
            onSend={async (content) => {
              const saved = await sendGroupMessage(groupId, content)
              thread.append(saved)
              reloadInbox()
            }}
          />
        </div>
      )}
    </>
  )
}
