'use client'

import { ErrorView } from '@/components/ui/ErrorView'

/**
 * the error boundary for every authenticated route.
 * ! one line, because ErrorView already knows what each failure looks like. this is the point
 * ! of the whole error model: a page or a boundary never decides how an error renders
 */
export default function MainError({ error, reset }: { error: Error; reset: () => void }) {
  return <ErrorView error={error} onRetry={reset} />
}
