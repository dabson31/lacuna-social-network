'use client'

import { useCallback, useEffect, useRef, useState } from 'react'
import { ApiError } from '@/lib/api/errors'

/**
 * a chat thread, which pages in the opposite direction from everything else in the app.
 *
 * ! the store queries ORDER BY id DESC LIMIT ? — the only way to get the most recent page —
 * ! and then calls reverse() before returning, so what arrives here is already chronological
 * ! and renders as-is. do NOT reverse it again: the query's ordering and the response's
 * ! ordering are deliberately different, and reading only the ORDER BY gets this backwards
 *
 * ! `before` for the next page is the id of the OLDEST message held, which is messages[0]
 * ! now that the page is ascending. using the newest pages forward into rows already on
 * ! screen and the thread looks stuck
 */
export function useThread<T extends { id: number }>(
  fetchPage: (before: number) => Promise<T[]>,
  deps: readonly unknown[],
  limit = 50,
) {
  const [messages, setMessages] = useState<T[]>([])
  const [loading, setLoading] = useState(true)
  const [loadingOlder, setLoadingOlder] = useState(false)
  const [error, setError] = useState<ApiError | null>(null)
  const [exhausted, setExhausted] = useState(false)

  const call = useRef(fetchPage)
  call.current = fetchPage
  const busy = useRef(false)

  const load = useCallback(async (before: number) => {
    if (busy.current) return
    busy.current = true
    if (before === 0) setLoading(true)
    else setLoadingOlder(true)
    try {
      const page = await call.current(before)
      setMessages((prev) => {
        if (before === 0) return page
        //? older messages prepend, already in order. deduped on id because a page boundary can
        //? overlap when something arrives mid scroll
        const seen = new Set(prev.map((m) => m.id))
        return [...page.filter((m) => !seen.has(m.id)), ...prev]
      })
      setExhausted(page.length < limit)
      setError(null)
    } catch (e: unknown) {
      setError(e instanceof ApiError ? e : new ApiError(0, { code: 'client_error', message: String(e) }))
    } finally {
      busy.current = false
      setLoading(false)
      setLoadingOlder(false)
    }
  }, [limit])

  useEffect(() => {
    setExhausted(false)
    void load(0)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps])

  const loadOlder = useCallback(() => {
    if (exhausted || busy.current || messages.length === 0) return
    void load(messages[0]!.id)
  }, [exhausted, messages, load])

  /** a message from the socket or from a send. deduped by id, which is the whole protocol */
  const append = useCallback((m: T) => {
    setMessages((prev) => (prev.some((x) => x.id === m.id) ? prev : [...prev, m]))
  }, [])

  return { messages, loading, loadingOlder, error, exhausted, loadOlder, append, reload: () => void load(0) }
}
