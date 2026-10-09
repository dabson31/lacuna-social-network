'use client'

import { useCallback, useEffect, useRef, useState } from 'react'
import { ApiError } from './errors'

/**
 * the entire data layer.
 * ? loading, data, error, refetch covers every list and detail page in this app. a cache would
 * ? buy exactly one thing, a live notification count, and the socket already keeps that live,
 * ? so there is nothing left for a query library to do here
 */
export interface Fetched<T> {
  data: T | null
  loading: boolean
  error: ApiError | null
  refetch: () => void
  /** for the rare case where a mutation already knows the new value and a round trip is waste */
  mutate: (next: T) => void
}

export function useFetch<T>(fn: () => Promise<T>, deps: readonly unknown[]): Fetched<T> {
  const [data, setData] = useState<T | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<ApiError | null>(null)
  const [nonce, setNonce] = useState(0)

  //! the call is held in a ref and the effect depends on deps, not on fn. an inline arrow
  //! passed as fn is a new function on every render, so depending on it would re-fetch
  //! forever, which is the single most common way this hook gets written wrong
  const call = useRef(fn)
  call.current = fn

  useEffect(() => {
    //! the unmounted guard. a page that navigates away mid flight otherwise writes its
    //! response into a component that is gone, and react logs about it, but the real damage
    //! is stale data landing in a remounted component
    let live = true
    setLoading(true)
    setError(null)

    call
      .current()
      .then((value) => {
        if (live) setData(value)
      })
      .catch((e: unknown) => {
        if (!live) return
        setError(e instanceof ApiError ? e : new ApiError(0, { code: 'client_error', message: String(e) }))
      })
      .finally(() => {
        if (live) setLoading(false)
      })

    return () => {
      live = false
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, nonce])

  const refetch = useCallback(() => setNonce((n) => n + 1), [])
  const mutate = useCallback((next: T) => setData(next), [])

  return { data, loading, error, refetch, mutate }
}
