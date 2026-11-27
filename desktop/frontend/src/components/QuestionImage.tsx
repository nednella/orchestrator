import { useEffect, useState } from 'react'
import { createPortal } from 'react-dom'
import { api, errorMessage } from '../api'
import { imagePath } from '../questionPreview'
import { ImageViewer } from './ImageViewer'

type QuestionImageProps = { thread: string; src: string; alt: string }

// The latest images read, by thread and path, so that a row the list mounts again does not read its image again.
const kept = new Map<string, string>()
const keptImages = 12

const read = async (thread: string, path: string): Promise<string> => {
  const key = `${thread} ${path}`
  const hit = kept.get(key)
  if (hit) return hit
  const file = await api.chatReadImage(thread, path)
  if (file.kind !== 'image') throw 'This file is not an image'
  const url = `data:${file.mime};base64,${file.data}`
  kept.set(key, url)
  if (kept.size > keptImages) kept.delete(kept.keys().next().value as string)
  return url
}

// QuestionImage is an image of a question's preview, read from the thread's own folders. A path the app refuses, or an
// address of the web, shows its description instead. A click opens it in the image viewer.
export function QuestionImage({ thread, src, alt }: QuestionImageProps) {
  const path = imagePath(src)
  const [shown, setShown] = useState<{ src: string; url: string } | { src: string; error: string } | null>(null)
  const [viewing, setViewing] = useState(false)

  useEffect(() => {
    if (path === null) return
    let stale = false
    read(thread, path).then(
      (url) => !stale && setShown({ src, url }),
      (err) => !stale && setShown({ src, error: errorMessage(err) }),
    )
    return () => {
      stale = true
    }
  }, [thread, path, src])

  const current = shown?.src === src ? shown : null
  if (current && 'url' in current) {
    return (
      <>
        <button type="button" className="q-image" aria-label={alt ? `Open ${alt}` : 'Open image'} onClick={() => setViewing(true)}>
          <img src={current.url} alt={alt} />
        </button>
        {viewing && createPortal(<ImageViewer url={current.url} alt={alt} onClose={() => setViewing(false)} />, document.body)}
      </>
    )
  }
  const error = path === null ? 'Only a file of this thread can be shown' : current && 'error' in current ? current.error : undefined
  return (
    <span className="q-image-missing" title={error}>
      {alt || src}
    </span>
  )
}
