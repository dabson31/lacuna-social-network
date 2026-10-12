'use client'

import { useRef, useState } from 'react'
import { ACCEPTED_IMAGE_TYPES } from '@shared/constants/domain'

/**
 * the file input, built from the composer's attachment stub so it looks like the rest of the
 * design: a hidden input plus a .chip showing what was chosen.
 * ! accept is a hint to the file dialog and nothing more. storage.Save sniffs the first 512
 * ! bytes with DetectContentType, so a .txt renamed to .png comes back 415 with fields.image
 * ! already filled in. never treat this attribute as validation
 */
export function ImagePicker({
  name,
  label = 'attach image',
  onPick,
}: {
  name: string
  label?: string
  onPick?: (file: File | null) => void
}) {
  const input = useRef<HTMLInputElement>(null)
  const [file, setFile] = useState<File | null>(null)

  const set = (next: File | null) => {
    setFile(next)
    onPick?.(next)
    if (!next && input.current) input.current.value = ''
  }

  return (
    <>
      <input
        ref={input}
        type="file"
        name={name}
        accept={ACCEPTED_IMAGE_TYPES}
        hidden
        onChange={(e) => set(e.target.files?.[0] ?? null)}
      />
      {file ? (
        <div className="attached" data-open="1">
          <span className="chip">
            {file.name} · {Math.round(file.size / 1024)} kb
            <button type="button" aria-label="Remove attachment" onClick={() => set(null)}>
              ×
            </button>
          </span>
        </div>
      ) : (
        <button type="button" className="chip chip--add" onClick={() => input.current?.click()}>
          + {label}
        </button>
      )}
    </>
  )
}
