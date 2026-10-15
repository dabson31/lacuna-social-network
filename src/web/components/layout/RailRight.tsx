import { PendingPanel } from './PendingPanel'
import { NextEventPanel } from './NextEventPanel'
import { PresencePanel } from './PresencePanel'
import { SocketPanel } from './SocketPanel'
import { ArchiveNote, CarrierPanel, IntegrityPanel, LinkPanel, LoopPanel } from './SimPanels'

/**
 * nine panels, always mounted, which is what makes notifications present on every page.
 * ! the order is fixed and no panel may be conditionally unmounted. the stylesheet's
 * ! responsive rules fold this rail into a two column grid below 1180px and give
 * ! .sec:nth-child(odd) a right border, so dropping a panel on some routes would put the
 * ! alternating borders on the wrong cells
 */
export function RailRight() {
  return (
    <aside className="rail-r" aria-label="Node status">
      <PendingPanel />
      <NextEventPanel />
      <PresencePanel />
      <LinkPanel />
      <CarrierPanel />
      <SocketPanel />
      <IntegrityPanel />
      <LoopPanel />
      <ArchiveNote />
    </aside>
  )
}
