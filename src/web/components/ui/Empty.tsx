/**
 * the design's .empty block. one element, one label, and the shell stays where it is.
 * ? the mockup's own strings are kept wherever it had one, they are written in the design's
 * ? voice and new copy would not be
 */
export function Empty({ label }: { label: string }) {
  return (
    <div className="empty">
      <span className="lbl">{label}</span>
    </div>
  )
}

/**
 * ! a list that is loading renders this and not a blank column, which would collapse the
 * ! layout and then push it back. there is no spinner in the stylesheet and adding one would
 * ! be a design change, so the design's idiom for waiting is the same label block
 */
export function Loading({ label = 'reading…' }: { label?: string }) {
  return <Empty label={label} />
}
