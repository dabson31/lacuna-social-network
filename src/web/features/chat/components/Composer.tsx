'use client'

import { useState } from 'react'
import { LIMITS, runeLength } from '@/lib/utils/count'

/**
 * the design's .chat__in form.
 * ! emoji need nothing built. validContent trims and length-checks and that is all — the
 * ! comment on it says so: "it does NOT sanitise ... stripping characters here is how emojis
 * ! and apostrophes get mangled and it protects nothing the output layer doesnt already
 * ! handle". react escapes on render, so the whole path is already correct
 * ? and no picker: the design's chat input is a plain text input, so the os keyboard and paste
 * ? are how emoji get typed
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
