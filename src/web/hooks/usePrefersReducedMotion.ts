'use client'

import { useEffect, useState } from 'react'

/**
 * the design has one global motion off switch and the stylesheet already honours it.
 * ! this is for the javascript half: the mockup gates four separate intervals on the same
 * ! query, the carrier bars, the loop player, the socket tick and the sweep. css alone cannot
 * ! stop a setInterval
 * ? it starts false so the server render and the first client render agree, otherwise
 * ? hydration mismatches on every reader who has the preference set
 */
export function usePrefersReducedMotion(): boolean {
  const [reduced, setReduced] = useState(false)

  useEffect(() => {
    const mq = matchMedia('(prefers-reduced-motion: reduce)')
    setReduced(mq.matches)
    const onChange = (e: MediaQueryListEvent) => setReduced(e.matches)
    mq.addEventListener('change', onChange)
    return () => mq.removeEventListener('change', onChange)
  }, [])

  return reduced
}
