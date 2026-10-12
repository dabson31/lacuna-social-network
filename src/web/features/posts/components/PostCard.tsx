'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { NavButton } from '@/components/ui/NavButton'
import { Privacy } from '@shared/constants/domain'
import type { Comment, Post } from '@shared/contracts/posts'
import { Avatar } from '@/components/ui/Avatar'
import { useToast } from '@/components/ui/Toast'
import { age } from '@/lib/utils/time'
import { Attachment } from './Attachment'
import { Thread } from './Thread'

/**
 * the design's feed card, ported from postHTML at line 3469. same elements, same classes,
 * same order.
 * ? Post carries author_name, author_avatar, comment_count and viewer_count so a feed of
 * ? twenty cards is one request. no per-card profile fetch for a field the row already has
 */

/** the privacy edge mark and tag, from privacyTag at line 3464 */
function PrivacyTag({ post }: { post: Post }) {
  if (post.privacy === Privacy.AlmostPrivate) return <span className="tag tag--semi">followers</span>
  if (post.privacy === Privacy.Private) return <span className="tag tag--priv">selected · {post.viewer_count}</span>
  return <span className="tag tag--pub">public</span>
}

export function PostCard({
  post,
  selfId,
  selfName,
  selfAvatar,
  loadComments,
  submitComment,
  onDelete,
  standalone = false,
}: {
  post: Post
  selfId: number
  selfName: string
  selfAvatar: string | null
  loadComments: (postId: number) => Promise<Comment[]>
  submitComment: (postId: number, body: FormData) => Promise<Comment>
  onDelete?: (postId: number) => Promise<void>
  /**
   * this card IS the page, at /posts/{id}.
   * ? one prop rather than two, because the two things it changes are the same decision: the
   * ? thread opens without being asked, since reading the replies is why you came, and the
   * ? "open" action goes, since it would navigate to the page you are already on
   */
  standalone?: boolean
}) {
  const [open, setOpen] = useState(standalone)
  const [meta, setMeta] = useState(false)
  const [mirrored, setMirrored] = useState(false)
  const [count, setCount] = useState(post.comment_count)
  const toast = useToast()
  const router = useRouter()
  const isSelf = post.author_id === selfId

  /**
   * the card itself opens the post.
   * ! lacuna.css has always lit `.post:hover` with the same wash `.row-main:hover` uses, so the
   * ! design treated the card as a live surface — it just had no page to send you to, and the
   * ! only thing a reader could actually open from the feed was the author's name
   * ! the whole card cannot be a <Link>: it holds eight buttons and a reply form, and an <a>
   * ! wrapping a <button> is invalid html that browsers re-parent, which breaks the card
   * ? so it is a click on the article with the things that already do something excluded. the
   * ? footer keeps its own `open` control, which is what makes this reachable from a keyboard:
   * ? the article is deliberately NOT given tabIndex or role=button, because a focusable thing
   * ? containing nine other focusable things is worse for a screen reader than no shortcut
   */
  const openFromCard = (e: React.MouseEvent<HTMLElement>) => {
    if (standalone) return
    const el = e.target as HTMLElement | null

    //? anything that already does something keeps doing it, and nothing else
    if (el?.closest('button, a, input, textarea, select, label, [role="button"]')) return

    //! the two panels that open INSIDE the card are reading surfaces, not the card. you
    //! expanded the replies to read them here, so a stray click in them must not yank the
    //! page out from under you
    if (el?.closest('.thread, .post__meta')) return

    //? a drag that ended up selecting text is a read, not a click
    if (window.getSelection()?.toString()) return

    router.push(`/posts/${post.id}`)
  }

  const withdraw = async () => {
    if (!onDelete) return
    try {
      await onDelete(post.id)
      toast(`withdrawn · post ${post.id} removed from this node`)
    } catch {
      toast('that post could not be withdrawn')
    }
  }

  return (
    <article
      className="post"
      data-priv={post.privacy}
      //? the hook additions.css hangs the pointer on. absent on the post's own page, where
      //? there is nowhere left to go
      data-openable={standalone ? undefined : '1'}
      data-id={post.id}
      onClick={openFromCard}
    >
      <div className="post__hd">
        <Avatar name={post.author_name} path={post.author_avatar} />
        <div className="post__who">
          <div className="nm">
            {/*? a button, because `.post__who .nm button` is the rule the design wrote */}
            <NavButton href={`/profile/${post.author_id}`}>{post.author_name}</NavButton>
          </div>
          <div className="mt">
            <span>post {post.id}</span>
            <span>·</span>
            <span>{age(post.created_at)}</span>
          </div>
        </div>
        <div className="spacer" />
        <PrivacyTag post={post} />
        <button
          className="btn btn--ghost"
          aria-expanded={meta}
          aria-label={`Detail for post ${post.id}`}
          onClick={() => setMeta((v) => !v)}
        >
          ⋯
        </button>
      </div>

      {meta && (
        <div className="q post__meta" style={{ margin: '11px 0 0 41px', fontSize: '10.5px' }}>
          &gt; id&nbsp;&nbsp;&nbsp;&nbsp; {post.id}
          <br />
          &gt; author {post.author_name}
          <br />
          &gt; privacy {post.privacy}
          <br />
          {/*? the design's "origin this node · not replicated" is literally true here: one
              process, one sqlite file, nothing is replicated anywhere */}
          &gt; origin this node · not replicated
        </div>
      )}

      <div className="post__body">
        {post.content && <p>{post.content}</p>}
        {post.image_path && <Attachment path={post.image_path} />}
      </div>

      <div className="post__ft">
        <button
          className="act"
          aria-expanded={open}
          aria-controls={`thread-${post.id}`}
          onClick={() => setOpen((v) => !v)}
        >
          reply <b>{count}</b>
        </button>

        {post.privacy === Privacy.Private ? (
          <button
            className="act act--on"
            onClick={() => toast(`visible to ${post.viewer_count} selected accounts · privacy private`)}
          >
            viewers <b>{post.viewer_count}</b>
          </button>
        ) : (
          /*
            ! mirror has no endpoint, no table and no column — it is the design's own
            ! distributed-node concept and there is no server side to it. it stays because it is
            ! the design's signature interaction, it stays local, and it claims nothing about
            ! the server beyond what the toast says
          */
          <button
            className={mirrored ? 'act act--on' : 'act'}
            aria-pressed={mirrored}
            onClick={() => {
              setMirrored((v) => !v)
              toast(mirrored ? `mirror withdrawn · post ${post.id}` : `mirrored locally · post ${post.id}`)
            }}
          >
            mirror <b>{mirrored ? 1 : 0}</b>
          </button>
        )}

        {/*
          ! the post's own page. the card had exactly one way out of it before — the author's
          ! name — so the only thing you could open from the feed was the person who wrote it
          ? a NavButton and not a Link: `.act` is styled on the element in places and a <button>
          ? is what the footer's other three actions are, so an <a> here would be the one
          ? underlined blue item in the row
        */}
        {!standalone && (
          <NavButton className="act" href={`/posts/${post.id}`} title={`Open post ${post.id}`}>
            open
          </NavButton>
        )}

        <button
          className="act"
          onClick={() => {
            void navigator.clipboard?.writeText(`lacuna://post/${post.id}`)
            toast(`post id · lacuna://post/${post.id}`)
          }}
        >
          copy id
        </button>
        <div className="spacer" />
        {isSelf && onDelete && (
          <button className="act" onClick={() => void withdraw()}>
            withdraw
          </button>
        )}
      </div>

      <div className="thread" id={`thread-${post.id}`} hidden={!open}>
        {open && (
          <Thread
            postId={post.id}
            selfName={selfName}
            selfAvatar={selfAvatar}
            load={() => loadComments(post.id)}
            submit={(body) => submitComment(post.id, body)}
            onCountChange={setCount}
          />
        )}
      </div>
    </article>
  )
}
