/**
 * the design's seven channels, from CHANNELS at line 3293 of the mockup.
 * ! ix is load bearing, it is the number printed on the tile AND the keyboard shortcut while
 * ! the bay has focus, so the order is part of the design and not an implementation detail
 * ? the only change from the mockup is that `id` was a hash fragment and is now a path. the
 * ? sub strings are kept verbatim, they are the subtitle under each channel's heading
 */
export interface Channel {
  id: string
  ix: string
  name: string
  href: string
  sub: string
  /** which badge the tile shows, since the three counters come from three different places */
  counter: 'none' | 'pending' | 'unread' | 'groups'
}

export const CHANNELS: readonly Channel[] = [
  {
    id: 'feed',
    ix: '01',
    name: 'Feed',
    href: '/feed',
    sub: 'Everything your follow graph permits you to see, newest first. Nothing here is replicated off this node.',
    counter: 'none',
  },
  {
    id: 'profile',
    ix: '02',
    name: 'Profile',
    href: '/profile',
    sub: 'Your half of the record: what you published, who may read it, and who is permitted to ask.',
    counter: 'none',
  },
  {
    id: 'directory',
    ix: '03',
    name: 'Directory',
    href: '/directory',
    sub: 'Every account this node has a record of. Private profiles answer with a request, not a page.',
    counter: 'none',
  },
  {
    id: 'groups',
    ix: '04',
    name: 'Groups',
    href: '/groups',
    sub: 'Membership, invitations and group posts. A group is a room this node keeps the key to.',
    counter: 'groups',
  },
  {
    id: 'events',
    ix: '05',
    name: 'Events',
    href: '/events',
    sub: 'Events raised inside your groups. Responses are stored locally and announced on the bus.',
    counter: 'none',
  },
  {
    id: 'messages',
    ix: '06',
    name: 'Messages',
    href: '/chat',
    sub: 'Private conversations over the one multiplexed socket. Delivery is per node, not per server.',
    counter: 'unread',
  },
  {
    id: 'notices',
    ix: '07',
    name: 'Notices',
    href: '/notifications',
    sub: 'Follow requests, invitations, join requests and event announcements, newest first.',
    counter: 'pending',
  },
]

/**
 * which channel a path belongs to, for the crumb, the heading and the active tile.
 * ! longest prefix wins, so /groups/4/chat resolves to groups and not to the first match. a
 * ! plain startsWith over the array in order would hand /chat to /chat and /groups/4/chat to
 * ! groups only by luck of ordering
 */
export function channelFor(pathname: string): Channel {
  let best = CHANNELS[0]!
  let bestLen = -1
  for (const c of CHANNELS) {
    if ((pathname === c.href || pathname.startsWith(c.href + '/')) && c.href.length > bestLen) {
      best = c
      bestLen = c.href.length
    }
  }
  //? a group's own chat room is still the Messages channel as far as the reader is concerned,
  //? even though the route hangs off the group
  if (/^\/groups\/\d+\/chat$/.test(pathname)) return CHANNELS.find((c) => c.id === 'messages')!
  return best
}
