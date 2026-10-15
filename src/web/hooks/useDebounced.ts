'use client'

import { useEffect, useState } from 'react'

/**
 * ? the directory's search is server side, so every keystroke would otherwise be a request.
 * ? the mockup had to rebuild its list on each one and then restore the caret by hand, which
 * ? react makes unnecessary, but the throttling is still needed
 */
export function useDebounced<T>(value: T, ms = 250): T {
  const [settled, setSettled] = useState(value)
  useEffect(() => {
    const t = setTimeout(() => setSettled(value), ms)
    return () => clearTimeout(t)
  }, [value, ms])
  return settled
}
