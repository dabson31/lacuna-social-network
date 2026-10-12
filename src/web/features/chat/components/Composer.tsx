'use client'

import { useRef, useState } from 'react'
import { EmojiPicker } from '@/components/ui/EmojiPicker'
import { LIMITS, runeLength } from '@/lib/utils/count'

/**
 * the design's .chat__in form.
 * ! emoji need nothing built. validContent trims and length-checks and that is all — the
 * ! comment on it says so: "it does NOT sanitise ... stripping characters here is how emojis
 * ! and apostrophes get mangled and it protects nothing the output layer doesnt already
 * ! handle". react escapes on render, so the whole path is already correct
 * ? the tray is the one addition to the design's own .chat__in row. the os keyboard and paste
 * ? already worked; the audit asks an auditor to CONFIRM emoji at a keyboard they did not pick,
 * ? and this is the field that question is about
 */
export function ChatComposer({
  placeholder,
  onSend,
  onTyping,
  disabled,
}: {
  placeholder: string
  onSend: (content: string) => Promise<void>
  onTyping?: () => void
  disabled?: boolean
}) {
  const [content, setContent] = useState('')
  const [busy, setBusy] = useState(false)
  const field = useRef<HTMLInputElement>(null)

  //! counted in runes, because the server counts with utf8.RuneCountInString so an emoji costs
  //! one character and not four. with .length a message of 500 emoji would be refused by a
  //! counter the server would have accepted
  const length = runeLength(content)
  const tooLong = length > LIMITS.message

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    const text = content.trim()
    if (!text || tooLong) return
    setBusy(true)
    try {
      await onSend(text)
      setContent('')
    } finally {
      setBusy(false)
    }
  }

  return (
    <form className="chat__in" onSubmit={submit}>
      <input
        ref={field}
        autoComplete="off"
        placeholder={placeholder}
        aria-label="Message"
        value={content}
        disabled={disabled}
        onChange={(e) => {
          setContent(e.target.value)
          onTyping?.()
        }}
      />
      {/*? onInsert and not a write to the node: the field is controlled, so react would paint
          over anything set on it directly at the next render */}
      <EmojiPicker target={field} onInsert={setContent} label="" />
      {tooLong && (
        <span className="lbl" style={{ color: 'var(--red)' }}>
          {length} / {LIMITS.message}
        </span>
      )}
      <button className="btn btn--primary" type="submit" disabled={busy || disabled || tooLong}>
        send
      </button>
    </form>
  )
}
