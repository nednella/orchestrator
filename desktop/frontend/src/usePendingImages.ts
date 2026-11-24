import { useState } from 'react'
import { useAgentos } from './AgentosContext'
import { imageFiles, readImage } from './images'
import type { PreviewImage } from './images'

export type PendingImages = {
  images: PreviewImage[]
  attach(data: DataTransfer | null): boolean
  remove(preview: string): void
  clear(): void
}

// The images pasted or dropped into a new note, held until the note is filed.
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
  }
}
