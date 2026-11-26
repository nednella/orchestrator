import { Maximize, Minus, Plus, Scan } from 'lucide-react'
import { useState } from 'react'
import type { FileContent } from '../types'

type FileImageViewProps = { path: string; content: FileContent }

const STEP = 1.25
const MIN_ZOOM = 0.1
const MAX_ZOOM = 8

const toolButton = 'btn btn-ghost h-7 w-7 flex-none justify-center px-0'

// FileImageView is an image of the file, with the size it has and buttons that zoom it. Zoom is a share of its own size,
// null while it fits the room.
export function FileImageView({ path, content }: FileImageViewProps) {
  const [size, setSize] = useState<{ w: number; h: number } | null>(null)
  const [failed, setFailed] = useState(false)
  const [zoom, setZoom] = useState<number | null>(null)
  const shown = zoom ?? 1
  const zoomTo = (to: number) => setZoom(Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, to)))

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex min-h-10 flex-none items-center gap-1 border-b border-(--border-rule) px-3 py-1 text-small text-soft">
        <span className="tabular-nums">{failed ? 'Cannot be shown' : size ? `${size.w} × ${size.h}` : ''}</span>
        <span className="flex-1" />
        <button className={toolButton} title="Zoom out" aria-label="Zoom out" disabled={failed} onClick={() => zoomTo(shown / STEP)}>
          <Minus size={14} aria-hidden="true" />
        </button>
        <span className="min-w-10 text-center tabular-nums">{zoom === null ? 'Fit' : `${Math.round(zoom * 100)}%`}</span>
        <button className={toolButton} title="Zoom in" aria-label="Zoom in" disabled={failed} onClick={() => zoomTo(shown * STEP)}>
          <Plus size={14} aria-hidden="true" />
        </button>
        <button className={toolButton} title="Fit to the room" aria-label="Fit to the room" aria-pressed={zoom === null} disabled={failed} onClick={() => setZoom(null)}>
          <Scan size={14} aria-hidden="true" />
        </button>
        <button className={toolButton} title="Actual size" aria-label="Actual size" aria-pressed={zoom === 1} disabled={failed} onClick={() => setZoom(1)}>
          <Maximize size={14} aria-hidden="true" />
        </button>
      </div>
      <div className="min-h-0 flex-1 overflow-auto p-4">
        <div className="flex min-h-full min-w-full w-fit items-center justify-center">
          {failed ? (
            <p className="text-small text-dim">This image cannot be shown.</p>
          ) : (
            <img
              src={`data:${content.mime};base64,${content.data}`}
              alt={path}
              draggable={false}
              className={`select-none ${zoom === null ? 'max-h-full max-w-full object-contain' : 'max-w-none flex-none'} ${zoom === null ? 'cursor-zoom-in' : ''}`}
              style={zoom !== null && size ? { width: size.w * zoom, height: size.h * zoom } : undefined}
              onLoad={(e) => setSize({ w: e.currentTarget.naturalWidth, h: e.currentTarget.naturalHeight })}
              onError={() => setFailed(true)}
              onClick={() => zoom === null && setZoom(1)}
            />
          )}
        </div>
      </div>
    </div>
  )
}
