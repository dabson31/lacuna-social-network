import { redirect } from 'next/navigation'

/**
 * ? the root is not a destination. middleware.ts has already decided whether this reader has a
 * ? cookie, so by the time this runs the only sensible place to be is the feed
 */
export default function Index() {
  redirect('/feed')
}
