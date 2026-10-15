import { uploadUrl } from '@/lib/api/client'
import { initials } from '@/lib/utils/identity'

/**
 * the design's .av block: initials in a machined square.
 * ! the stylesheet says "no stock faces anywhere" and the mockup only ever draws initials,
 * ! because it had no server to upload to. the subject does require an avatar though, so the
 * ! uploaded image goes INSIDE the same square and the initials stay as the fallback. the
 * ! geometry, border and sizing are untouched, and a user who skipped the upload looks exactly
 * ! like the mockup
 */
export function Avatar({
  name,
  path,
  size = 'md',
}: {
  name: string
  path?: string | null
  size?: 'sm' | 'md' | 'lg'
}) {
  const cls = size === 'sm' ? 'av av--sm' : size === 'lg' ? 'av av--lg' : 'av'

  if (path) {
    return (
      <span className={cls} aria-hidden="true">
        {/* plain img and not next/image: the files are served by the go process from
            /uploads/{name}, and the optimiser would need that host whitelisted to then
            re-encode an image the server already validated */}
        <img src={uploadUrl(path)} alt="" />
      </span>
    )
  }
  return (
    <span className={cls} aria-hidden="true">
      {initials(name)}
    </span>
  )
}
