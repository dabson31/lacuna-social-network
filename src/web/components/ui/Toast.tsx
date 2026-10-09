'use client'

import { createContext, useCallback, useContext, useMemo, useRef, useState } from 'react'

/**
 * the design's toast, ported from the mockup's toast() and its 2600ms timer.
 * ? the message style stays as the mockup writes it, lowercase with a middle dot separator,
 * ? "reply stored · post 4". it is part of the design's voice
 */
const TOAST_MS = 2600

const ToastContext = createContext<((message: string) => void) | null>(null)

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [message, setMessage] = useState('')
  const [shown, setShown] = useState(false)
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)

  const show = useCallback((next: string) => {
    setMessage(next)
    setShown(true)
    if (timer.current) clearTimeout(timer.current)
    timer.current = setTimeout(() => setShown(false), TOAST_MS)
  }, [])

  const value = useMemo(() => show, [show])

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div className="toast" data-show={shown ? '1' : '0'} role="status" aria-live="polite">
        <span className="dot dot--on" />
        <span>{message}</span>
      </div>
    </ToastContext.Provider>
  )
}

export function useToast(): (message: string) => void {
  const ctx = useContext(ToastContext)
  //? a no-op outside the provider rather than a throw: a toast is never load bearing, and a
  //? component that cannot announce something should still render
  return ctx ?? (() => {})
}
