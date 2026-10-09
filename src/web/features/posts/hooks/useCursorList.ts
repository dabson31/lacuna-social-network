'use client'

import { useCallback, useEffect, useRef, useState } from 'react'
import { ApiError } from '@/lib/api/errors'

/**
 * keyset pagination for the lists that grow at the top.
 * ! cursor and not offset, and chat/handler.go gives the reason, which applies to the feed
 * ! identically: with an offset the page boundary slides as new rows arrive, so you get shown
 * ! the same post twice and miss one. `before` is a row id, so the boundary is anchored to a
 * ! row and cannot drift
 * ? zero means "from the newest". the limit is clamped server side, so asking for 500 gets 20
 */
export interface CursorList<T> {
  items: T[]
  loading: boolean
  /** true while a second or later page is in flight, so the button can say so */
  loadingMore: boolean
  error: ApiError | null
  /** no more pages, which is the signal to draw the design's .eof marker */
  exhausted: boolean
  loadMore: () => void
  reload: () => void
  /** a new row the caller just created, placed at the top without a refetch */
  prepend: (item: T) => void
  remove: (id: number) => void
  replace: (items: T[]) => void
}

export function useCursorList<T extends { id: number }>(
  fetchPage: (before: number, limit: number) => Promise<T[]>,
  limit = 20,
  deps: readonly unknown[] = [],
): CursorList<T> {
  const [items, setItems] = useState<T[]>([])
  const [loading, setLoading] = useState(true)
  const [loadingMore, setLoadingMore] = useState(false)
  const [error, setError] = useState<ApiError | null>(null)
  const [exhausted, setExhausted] = useState(false)

  const call = useRef(fetchPage)
  call.current = fetchPage
  // guards a second loadMore firing while the first is still out, which a scroll handler will
  // absolutely do and which would fetch the same page twice
  const busy = useRef(false)

  const load = useCallback(async (before: number) => {
    if (busy.current) return
    busy.current = true
    if (before === 0) setLoading(true)
    else setLoadingMore(true)
    setError(null)
    try {
      const page = await call.current(before, limit)
      setItems((prev) => {
        if (before === 0) return page
        //! dedupe on append. a post created between two page fetches shifts the window, and a
        //! row arriving over the socket may already be in the list. keying on id makes both
        //! harmless instead of visible
        const seen = new Set(prev.map((p) => p.id))
        return [...prev, ...page.filter((p) => !seen.has(p.id))]
      })
      //! a short page is the only end-of-list signal the api gives, there is no total count
      //! anywhere. so never try to render "page 3 of 9", the design does not ask for it
      setExhausted(page.length < limit)
    } catch (e: unknown) {
      setError(e instanceof ApiError ? e : new ApiError(0, { code: 'client_error', message: String(e) }))
    } finally {
      busy.current = false
      setLoading(false)
      setLoadingMore(false)
    }
  }, [limit])

  useEffect(() => {
    setExhausted(false)
    void load(0)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps])

  const loadMore = useCallback(() => {
    if (exhausted || busy.current || items.length === 0) return
    //! the cursor is the id of the OLDEST row held, because the list is ordered id DESC. using
    //! the newest pages forward into rows already on screen and looks like the feed is stuck
    void load(items[items.length - 1].id)
  }, [exhausted, items, load])

  return {
    items,
    loading,
    loadingMore,
    error,
    exhausted,
    loadMore,
    reload: () => void load(0),
    prepend: (item) => setItems((prev) => [item, ...prev.filter((p) => p.id !== item.id)]),
    remove: (id) => setItems((prev) => prev.filter((p) => p.id !== id)),
    replace: (next) => setItems(next),
  }
}
