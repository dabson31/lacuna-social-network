import { GLYPH } from './glyphs'

/**
 * one <rect> per filled pixel in a 12x12 viewBox, exactly as the mockup's glyph() builds it
 */
export function Glyph({ id }: { id: string }) {
  const rows = GLYPH[id]
  if (!rows) return null
  const cells: React.ReactElement[] = []
  rows.forEach((row, y) => {
    for (let x = 0; x < 12; x++) {
      if (row[x] === '#') cells.push(<rect key={`${x}-${y}`} x={x} y={y} width="1" height="1" />)
    }
  })
  return (
    <svg viewBox="0 0 12 12" aria-hidden="true">
      {cells}
    </svg>
  )
}
