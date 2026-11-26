import { useContext, useEffect, useState } from 'react'
import { api } from '../api'
import { imageTarget } from '../filesView'
import { FileLinks } from './fileLinks'

type FileMarkdownImageProps = { src?: string; alt?: string }

const reason = (err: unknown) => (err instanceof Error ? err.message : String(err))

// FileMarkdownImage is an image a markdown file names by a path in its own folder, read as any file of the folder is, so
// a path out of the folder, or of a file that is no image, shows its text and not the image.
export function FileMarkdownImage({ src, alt }: FileMarkdownImageProps) {
  const from = useContext(FileLinks)
  const [shown, setShown] = useState<{ src: string; url: string } | { src: string; error: string } | null>(null)

  useEffect(() => {
    const path = from && src ? imageTarget(from.path, src) : null
    if (!from || !src || path === null) return
    let stale = false
    const read = from.key.startsWith('project:') ? api.projectReadFile(from.key.slice('project:'.length), path) : api.chatReadFile(from.key, path)
    read.then(
      (c) => !stale && setShown(c.kind === 'image' ? { src, url: `data:${c.mime};base64,${c.data}` } : { src, error: 'This file is not an image' }),
      (err) => !stale && setShown({ src, error: reason(err) }),
    )
    return () => {
      stale = true
    }
  }, [from, src])

  const current = shown?.src === src ? shown : null
  if (current && 'url' in current) return <img src={current.url} alt={alt ?? ''} />
  return (
    <span className="text-dim" title={current && 'error' in current ? current.error : undefined}>
      {alt || src}
    </span>
  )
}
