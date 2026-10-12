'use client'

import { useEffect, useRef, useState } from 'react'

/**
 * the emoji tray, one component for every field in the app that takes words.
 *
 * ! the subject asks that "users be able to send emojis to each other" and the audit asks an
 * ! auditor to CONFIRM it at a keyboard they did not choose. paste and the OS picker were
 * ! always enough for the feature; they are not enough for the demonstration, which is the
 * ! gap this closes
 *
 * ! it goes on the fields that carry a person's own words and nowhere else. not on a search
 * ! box, which is a control rather than content, and never on the login or register forms —
 * ! an emoji in an email or a password is a support ticket, not a feature. the profile editor
 * ! is where a registered user adds one to their bio, which is the same field register asks
 * ! for and the right place to offer it
 *
 * ? no dependency. a picker package is ~50kb of data and a component library's opinions, for
 * ? a grid of buttons over a frozen list
 */

/**
 * the tray's contents. a deliberate selection rather than the whole of Unicode: enough to
 * answer with, small enough to read without scrolling, and the labels are what a screen
 * reader announces and what the tooltip shows
 * ? multi codepoint entries (❤️, ✌️) are here on purpose. they are the ones that get mangled
 * ? by a naive byte-wise insert, so having them in the default set means any such bug shows up
 * ? immediately rather than on the one emoji nobody tested
 */
const EMOJI: ReadonlyArray<readonly [string, string]> = [
  ['😀', 'grinning'],
  ['😁', 'beaming'],
  ['😂', 'tears of joy'],
  ['🤣', 'rolling on the floor'],
  ['😊', 'smiling'],
  ['😍', 'heart eyes'],
  ['😘', 'blowing a kiss'],
  ['😎', 'cool'],
  ['🤔', 'thinking'],
  ['😅', 'nervous laugh'],
  ['🙃', 'upside down'],
  ['😏', 'smirking'],
  ['😬', 'grimacing'],
  ['😭', 'sobbing'],
  ['😡', 'angry'],
  ['🥳', 'partying'],
  ['🤯', 'mind blown'],
  ['😴', 'sleeping'],
  ['👍', 'thumbs up'],
  ['👎', 'thumbs down'],
  ['👏', 'clapping'],
  ['🙏', 'folded hands'],
  ['🤝', 'handshake'],
  ['💪', 'flexed arm'],
  ['👋', 'waving'],
  ['✌️', 'victory'],
  ['👀', 'eyes'],
  ['🧠', 'brain'],
  ['❤️', 'red heart'],
  ['💔', 'broken heart'],
  ['🔥', 'fire'],
  ['✨', 'sparkles'],
  ['⭐', 'star'],
  ['💯', 'hundred'],
  ['✅', 'check mark'],
  ['❌', 'cross mark'],
  ['⚡', 'high voltage'],
  ['🎉', 'party popper'],
  ['🎂', 'cake'],
  ['☕', 'coffee'],
  ['🍕', 'pizza'],
  ['🚀', 'rocket'],
  ['💻', 'laptop'],
  ['🌙', 'crescent moon'],
]

export function EmojiPicker({
  target,
  onInsert,
  placement = 'up',
  className = 'act',
  label = 'emoji',
}: {
  /** the field the emoji lands in. the picker reads its caret, so it needs the node itself */
  target: React.RefObject<HTMLInputElement | HTMLTextAreaElement | null>
  /**
   * for a CONTROLLED field: react owns the value, so handing the node a new one is undone on
   * the next render. the new whole-string value goes back through this instead.
   * ? left out for an uncontrolled field — the ones read with `new FormData(form)` — where the
   * ? node IS the state and writing to it is the only way to change it
   */
  onInsert?: (next: string) => void
  /**
   * which way the tray opens. 'up' suits the composers, which sit under what they are about;
   * 'down' is for a field near the top of a form, where up would leave the viewport
   * ? a prop and not a measurement. measuring means reading layout on open, which is a reflow
   * ? and a class of bug that only shows up at one window size
   */
  placement?: 'up' | 'down'
  className?: string
  label?: string
}) {
  const [open, setOpen] = useState(false)
  const box = useRef<HTMLSpanElement>(null)

  /**
   * ! both listeners are bound only while the tray is open. a picker in every composer on a
   * ! page means several of these mounted at once, and leaving them bound would put a document
   * ! level handler per field on every keystroke anywhere
   */
  useEffect(() => {
    if (!open) return
    const onDown = (e: PointerEvent) => {
      if (!box.current?.contains(e.target as Node)) setOpen(false)
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
      setOpen(false)
      //? focus goes back where it came from, so escape does not strand the caret
      target.current?.focus()
    }
    document.addEventListener('pointerdown', onDown)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('pointerdown', onDown)
      document.removeEventListener('keydown', onKey)
    }
  }, [open, target])

  const insert = (emoji: string) => {
    const el = target.current
    if (!el) return

    //! at the caret, replacing any selection — the same thing typing the character would have
    //! done. appending to the end instead is the version of this that is wrong the moment
    //! somebody goes back to add a reaction mid-sentence
    const start = el.selectionStart ?? el.value.length
    const end = el.selectionEnd ?? start
    const next = el.value.slice(0, start) + emoji + el.value.slice(end)

    //! maxLength is enforced by the browser for TYPING and not for a value set from script, so
    //! without this the tray is the one way to push a field past the limit its own counter is
    //! showing. the server would then refuse the whole post
    //? maxLength counts utf-16 units while the server counts runes, so this is the stricter of
    //? the two and refusing here can never produce something the server would have taken
    if (el.maxLength > 0 && next.length > el.maxLength) return

    if (onInsert) onInsert(next)
    else {
      el.value = next
      //? so anything watching the field — a counter, a dirty flag — hears about a change it
      //? did not cause. react's own onChange is wired to this event
      el.dispatchEvent(new Event('input', { bubbles: true }))
    }

    /**
     * ! the caret has to be put back by hand. for a controlled field react re-renders with the
     * ! new value and the browser drops the caret at the end, so without this every emoji after
     * ! the first lands in the wrong place
     * ? after the frame, because the value has to be committed before the offset means
     * ? anything. measured in utf-16 units, which is what setSelectionRange takes and why a
     * ? two-codepoint emoji still lands the caret correctly
     */
    const caret = start + emoji.length
    requestAnimationFrame(() => {
      el.focus()
      el.setSelectionRange(caret, caret)
    })
  }

  return (
    <span className="emoji" ref={box}>
      {/*
        ! type="button", and this is not decoration: every one of these sits inside a <form>,
        ! and a button with no type is a SUBMIT button. without it, opening the tray posts the
        ! composer and picking an emoji posts it again
      */}
      <button
        type="button"
        className={className}
        aria-expanded={open}
        aria-haspopup="true"
        aria-label={open ? 'Close the emoji tray' : 'Open the emoji tray'}
        title="Insert an emoji"
        onClick={() => setOpen((v) => !v)}
      >
        {/*? the glyph alone where the row is tight — a chat bar, a reply box — and glyph plus
            word where there is room beside "attach image". an empty label must not leave a
            trailing space inside the button */}
        {label ? `🙂 ${label}` : '🙂'}
      </button>

      {open && (
        <div className="emoji__pop panel" data-place={placement} role="dialog" aria-label="Emoji">
          <div className="emoji__grid">
            {EMOJI.map(([glyph, name]) => (
              <button key={glyph} type="button" title={name} aria-label={name} onClick={() => insert(glyph)}>
                {glyph}
              </button>
            ))}
          </div>
          {/*? it stays open after a pick, because reactions come in twos and threes. outside
              click and escape are what close it */}
          <p className="lbl emoji__hint">click to insert · esc to close</p>
        </div>
      )}
    </span>
  )
}
