import type { KeyboardEvent } from 'react'

/**
 * enter sends, shift+enter breaks the line.
 * ! a textarea's default is the other way round, and the post composers are the one place in
 * ! this app where that default is wrong. everything else you type into is an <input> — the
 * ! chat box, the reply box, the search fields — and an input has submitted on enter since
 * ! forever, so a post box that could only be sent with the mouse was the odd one out
 * ! this lives here rather than in either composer because the two of them have to agree. a
 * ! feed post and a group post are the same gesture, and two copies of a key handler is how
 * ! one of them quietly keeps the old behaviour
 * ? the modifier check is not only about shift. ctrl+enter and cmd+enter are a send in other
 * ? clients, but here they would be a send the caller did not choose, and alt+enter is a
 * ? newline on some keyboard layouts. only a bare enter sends
 */
export function submitOnEnter(e: KeyboardEvent<HTMLTextAreaElement>, form: HTMLFormElement | null): void {
  if (e.key !== 'Enter' || e.shiftKey || e.altKey || e.ctrlKey || e.metaKey) return
  //! the ime guard, and the reason this is not a one liner. with a japanese or chinese
  //! keyboard the first enter commits the candidate word and the browser reports it as a
  //! keydown like any other. without this check that enter posts a half typed sentence, and
  //! it is unreproducible for anyone typing in latin script
  if (e.nativeEvent.isComposing) return
  e.preventDefault()
  //? requestSubmit and not submit(): submit() skips the submit event entirely, so the react
  //? onSubmit handler never runs and the form does a native page navigation instead
  form?.requestSubmit()
}
