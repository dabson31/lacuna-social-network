import { Empty } from '@/components/ui/Empty'

/**
 * ? inside the shell, so a missing thing never costs the reader their navigation. the copy does
 * ? not say "deleted": a 404 from this api means not found OR not visible to you, and claiming
 * ? deletion would be wrong every time a stranger opens an almost_private post
 */
export default function MainNotFound() {
  return <Empty label="not available · this is not here, or not yours to see" />
}
