import { Bot, Check, ChevronLeft, ChevronRight, Copy, Minus, Plus, Scan, Trash2, User, X } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useOrchestrator } from '../OrchestratorContext'
import { api } from '../api'
import { confirmed } from '../confirm'
import type { Evidence } from '../types'
import { Ago } from './Ago'
import { ChatActionButton } from './ChatActionButton'
import { ViewerBar } from './ViewerBar'
import { FIT, ZOOM_STEP, ZoomableImage, zoomTo, type ImageView } from './ZoomableImage'

const DELETE_EVIDENCE = { title: 'Delete this media?', confirm: 'Delete', tone: 'danger' } as const

type EvidenceViewerProps = {
  threadId: string
  items: Evidence[]
  startId: string
  onClose(): void
}

export function EvidenceViewer({ threadId, items, startId, onClose }: EvidenceViewerProps) {
  const { report } = useOrchestrator('report')
  const root = useRef<HTMLDivElement>(null)
  const [id, setId] = useState(startId)
  const [copied, setCopied] = useState(false)
  const [view, setView] = useState<ImageView>(FIT)
  const at = items.findIndex((i) => i.id === id)
  const item = items[at]
  const Source = item?.source === 'agent' ? Bot : User

  useEffect(() => root.current?.focus(), [])

  useEffect(() => {
    if (!item) onClose()
  }, [item, onClose])

  if (!item) return null

  const step = (delta: number) => {
    setView(FIT)
    setId(items[(at + delta + items.length) % items.length].id)
  }

  const remove = () => {
    const next = items[at + 1] ?? items[at - 1]
    setView(FIT)
    if (next) setId(next.id)
    return api.deleteEvidence(threadId, item.id)
  }

  return (
    <div
      ref={root}
      tabIndex={-1}
      role="dialog"
      aria-label="Media"
      className="fade-in fixed inset-0 flex flex-col items-center justify-center gap-3 p-4 outline-none"
      style={{ zIndex: 'var(--z-overlay)', background: 'var(--bg-overlay-strong)' }}
      onMouseDown={(e) => e.target === e.currentTarget && onClose()}
      onKeyDown={(e) => {
        if (e.key === 'Escape') onClose()
        if (e.key === 'ArrowLeft') step(-1)
        if (e.key === 'ArrowRight') step(1)
        if (e.metaKey) return
        if (e.key === '=' || e.key === '+') setView((v) => zoomTo(v, v.scale * ZOOM_STEP))
        if (e.key === '-') setView((v) => zoomTo(v, v.scale / ZOOM_STEP))
        if (e.key === '0') setView(FIT)
      }}
    >
      <ZoomableImage src={item.url} alt={item.caption || 'Image'} view={view} onView={setView} onDismiss={onClose} />
      <div className="flex max-w-3xl flex-col items-center gap-2 text-center">
        {item.caption && <p className="text-body">{item.caption}</p>}
        <p className="flex items-center gap-1.5 text-small text-dim tabular-nums">
          <span>
            {at + 1} of {items.length}
          </span>
          <span aria-hidden="true">·</span>
          <Source size={12} aria-hidden="true" />
          <span>{item.source === 'agent' ? 'Agent' : 'You'}</span>
          <span aria-hidden="true">·</span>
          <Ago at={item.at} />
          <span aria-hidden="true">·</span>
          <span>{Math.round(view.scale * 100)}%</span>
        </p>
        <ViewerBar>
          <ChatActionButton label="Previous image" disabled={items.length < 2} onClick={() => step(-1)}>
            <ChevronLeft size={14} aria-hidden="true" />
          </ChatActionButton>
          <ChatActionButton label="Next image" disabled={items.length < 2} onClick={() => step(1)}>
            <ChevronRight size={14} aria-hidden="true" />
          </ChatActionButton>
          <ChatActionButton label="Zoom out" disabled={view.scale === 1} onClick={() => setView((v) => zoomTo(v, v.scale / ZOOM_STEP))}>
            <Minus size={14} aria-hidden="true" />
          </ChatActionButton>
          <ChatActionButton label="Fit image" disabled={view.scale === 1} onClick={() => setView(FIT)}>
            <Scan size={14} aria-hidden="true" />
          </ChatActionButton>
          <ChatActionButton label="Zoom in" onClick={() => setView((v) => zoomTo(v, v.scale * ZOOM_STEP))}>
            <Plus size={14} aria-hidden="true" />
          </ChatActionButton>
          <ChatActionButton
            label={copied ? 'Copied' : 'Copy link'}
            onClick={() => {
              report(() => navigator.clipboard.writeText(item.url))
              setCopied(true)
              setTimeout(() => setCopied(false), 1500)
            }}
          >
            {copied ? <Check size={14} aria-hidden="true" /> : <Copy size={14} aria-hidden="true" />}
            {copied ? 'Copied' : 'Copy link'}
          </ChatActionButton>
          <ChatActionButton label="Delete" onClick={() => report(() => confirmed(DELETE_EVIDENCE, remove))}>
            <Trash2 size={14} aria-hidden="true" />
          </ChatActionButton>
          <ChatActionButton label="Close" onClick={onClose}>
            <X size={14} aria-hidden="true" />
          </ChatActionButton>
        </ViewerBar>
      </div>
    </div>
  )
}
