'use client'

import { useCallback, useEffect, useRef, useState } from 'react'
import Link from 'next/link'
import { useParams } from 'next/navigation'
import { WsFrame } from '@shared/constants/ws'
import { Empty, Loading } from '@/components/ui/Empty'
import { ErrorView } from '@/components/ui/ErrorView'
import { useBadges } from '@/components/layout/BadgeProvider'
import { useUser } from '@/lib/session/SessionProvider'
import { useFrame, useSocket } from '@/lib/realtime/SocketProvider'
import { displayName } from '@/lib/utils/identity'
import { ChatLog, type Bubble } from '@/features/chat/components/ChatLog'
import { ChatComposer } from '@/features/chat/components/Composer'
import { useThread } from '@/features/chat/hooks/useThread'
import { dmHistory, markDMRead, sendDM } from '@/features/chat/api'

const TYPING_CLEAR_MS = 3500
const TYPING_THROTTLE_MS = 1800

/**
 * a direct conversation.
 * ? the route param is the peer's user id, which is why numeric ids won over handles: the
 * ? nickname column is nullable, and a route keyed on something that can be null is a 404
 * ? generator
 */
export default function ConversationPage() {
  const params = useParams<{ conversationId: string }>()
  const peerId = Number(params.conversationId)
  const me = useUser()
  const { inbox, reloadInbox } = useBadges()
  const { send } = useSocket()
  const [typing, setTyping] = useState(false)

  const thread = useThread((before) => dmHistory(peerId, before), [peerId])
  const lastTypingSent = useRef(0)
  const typingTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  const peer = inbox?.direct.find((c) => c.peer_id === peerId)
  const peerName = peer ? displayName(peer) : `user ${peerId}`

  /** marking read on open, then again whenever a frame lands while this thread is on screen */
  const markRead = useCallback(() => {
    void markDMRead(peerId)
      .then(reloadInbox)
      .catch(() => {})
  }, [peerId, reloadInbox])

  useEffect(() => {
    if (!thread.loading) markRead()
  }, [thread.loading, markRead])

  //! a dm arriving for this conversation appends and clears the unread. deduped by id inside
  //! append, because POST /api/messages already returned the same row to the sender and the
  //! socket pushes it to both sides by design
  useFrame(
    WsFrame.DmMessage,
    (m) => {
      if (m.sender_id !== peerId && m.recipient_id !== peerId) return
      thread.append(m)
      setTyping(false)
      if (m.sender_id === peerId) markRead()
    },
    [peerId, thread.append, markRead],
  )

  //! there is no "stopped typing" frame, so a timer is the only thing that ever removes the
  //! line. without it the indicator sticks forever after one keystroke
  useFrame(
    WsFrame.DmTyping,
    (t) => {
      if (t.from_user_id !== peerId) return
      setTyping(true)
      if (typingTimer.current) clearTimeout(typingTimer.current)
      typingTimer.current = setTimeout(() => setTyping(false), TYPING_CLEAR_MS)
    },
    [peerId],
  )

  useEffect(() => () => void (typingTimer.current && clearTimeout(typingTimer.current)), [])

  const onTyping = () => {
    //? throttled, or every keystroke is a frame the server then has to gate and fan out
    const now = Date.now()
    if (now - lastTypingSent.current < TYPING_THROTTLE_MS) return
    lastTypingSent.current = now
    send({ type: WsFrame.DmTyping, payload: { peer_id: peerId } })
  }

  if (Number.isNaN(peerId)) return <Empty label="not available · that is not a user id" />

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
        <span className={peer?.unread ? 'dot dot--on' : 'dot dot--idle'} />
        <span className="lbl">{peerName}</span>
        <div className="spacer" />
        <Link className="btn btn--sm" href={`/profile/${peerId}`}>
          profile
        </Link>
      </div>

      {/*
        ! a 403 chat_not_allowed renders here as the red notice bar rather than an empty state,
        ! and that is deliberate on the server's side too: the recipient existing is not a
        ! secret, they are findable in the directory, it is only the permission to write to
        ! them that is missing. this is the status the audit's two-strangers check lands on
      */}
      {thread.error && <ErrorView error={thread.error} onRetry={thread.reload} />}

      {thread.loading ? (
        <Loading label="reading the conversation…" />
      ) : (
        <div className="chat">
          <ChatLog
            bubbles={bubbles}
            onReachTop={thread.loadOlder}
            loadingOlder={thread.loadingOlder}
            exhausted={thread.exhausted}
          />
          <div className="chat__typing">{typing ? `${peerName} is typing…` : ''}</div>
          <ChatComposer
            placeholder={`message ${peerName}`}
            onTyping={onTyping}
            onSend={async (content) => {
              const saved = await sendDM(peerId, content)
              thread.append(saved)
              reloadInbox()
            }}
          />
        </div>
      )}
    </>
  )
}
