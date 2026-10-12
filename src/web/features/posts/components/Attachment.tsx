import { uploadUrl } from '@/lib/api/client'

/**
 * the design's .plate, a dithered frame around an image.
 * ? the stylesheet's own note says "a dithered plate, never a stock photo" — the plate is the
 * ? frame and the real upload goes inside it
 * ! image_path is a bare filename. storage.Save returns a random name and the database stores
 * ! only that, so the client builds the url. the design's sha span is dropped rather than
 * ! filled with a fake digest, which would read as real and is not
 */
export function Attachment({ path }: { path: string }) {
  return (
    <div className="plate">
      <div className="img">
        <img src={uploadUrl(path)} alt="" loading="lazy" />
      </div>
      <div className="cap">
        <span>{path}</span>
        <div className="spacer" />
        <span>upload</span>
      </div>
    </div>
  )
}
