'use client'

import { NavButton } from '@/components/ui/NavButton'
import { usePathname } from 'next/navigation'
import { channelFor } from './channels'

/**
 * the crumb, the heading with its blinking cursor, and the subtitle.
 * ? the mockup recomputed these in renderShell() on every route change. here the pathname is
 * ? the input and the channel table is the lookup, which is the same thing with the router
 * ? doing the bookkeeping
 */
export function MainHead({ trail }: { trail?: string }) {
  const pathname = usePathname()
  const channel = channelFor(pathname)

  return (
    <div className="main__hd">
      <div className="crumb">
        <NavButton href="/feed">lacuna</NavButton>
        <span>/</span>
        <span>layer 02</span>
        <span>/</span>
        <span className="cur">{channel.name.toLowerCase()}</span>
        {trail && (
          <>
            <span>/</span>
            <span className="cur">{trail}</span>
          </>
        )}
      </div>
      <div className="title">
        <h1>
          {channel.name}
          <em>_</em>
        </h1>
        <p className="sub">{channel.sub}</p>
      </div>
    </div>
  )
}
