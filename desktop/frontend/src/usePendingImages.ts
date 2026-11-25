import { useState } from 'react'
import { useAgentos } from './AgentosContext'
import { imageFiles, readImage } from './images'
import type { PreviewImage } from './images'

export type PendingImages = {
  images: PreviewImage[]
  attach(data: { files: FileList | null } | null): boolean
  remove(preview: string): void
  clear(): void
  // restore puts back images taken for a send that failed, before any attached since.
  restore(images: PreviewImage[]): void
}

// The images pasted, dropped or picked for a note or a message, held until it is sent.
export function usePendingImages(): PendingImages {
  const { report } = useAgentos('report')
  const [images, setImages] = useState<PreviewImage[]>([])
  return {
    images,
    attach(data) {
      const files = imageFiles(data)
      if (files.length === 0) return false
      report(async () => {
        const read = await Promise.all(files.map(readImage))
        setImages((list) => [...list, ...read])
      })
      return true
    },
    remove: (preview) => setImages((list) => list.filter((i) => i.preview !== preview)),
    clear: () => setImages([]),
    restore: (taken) => setImages((list) => [...taken, ...list]),
  }
}
