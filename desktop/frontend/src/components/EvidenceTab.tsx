import { ArrowUpDown, Columns2 } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useOrchestrator } from '../OrchestratorContext'
import { readStored, writeStored } from '../storage'
import type { Thread } from '../types'
import { ChatActionButton } from './ChatActionButton'
import { EvidenceCard } from './EvidenceCard'
import { EvidenceCompare } from './EvidenceCompare'
import { EvidenceViewer } from './EvidenceViewer'
import { Notice } from './Notice'

type EvidenceTabProps = { thread: Thread }

const ORDER_KEY = 'orchestrator.evidenceNewest'

export function EvidenceTab({ thread }: EvidenceTabProps) {
  const { evidence } = useOrchestrator('evidence')
  const [newestFirst, setNewestFirst] = useState(() => readStored(ORDER_KEY, false))
  const [comparing, setComparing] = useState(false)
  const [picked, setPicked] = useState<string[]>([])
  const [viewing, setViewing] = useState<string | null>(null)
  const [showCompare, setShowCompare] = useState(false)

  const items = useMemo(() => {
    const list = [...(evidence[thread.id] ?? [])].sort((a, b) => a.at - b.at)
    return newestFirst ? list.reverse() : list
  }, [evidence, thread.id, newestFirst])
  const images = items.filter((i) => i.kind === 'image')
  const [left, right] = picked.map((id) => items.find((i) => i.id === id))

  const toggle = (id: string) => setPicked((list) => (list.includes(id) ? list.filter((x) => x !== id) : [...list.slice(-1), id]))

  if (items.length === 0) {
    return (
      <div className="absolute inset-0 flex flex-col">
        <Notice
          centered
          title="Nothing to show yet"
          hint={
            <>
              Screenshots, before and after pairs and short notes from this thread appear here. The agent files them with its <code className="mono">show</code> tool. You can also capture the browser yourself from its toolbar.
            </>
          }
        />
      </div>
    )
  }

  return (
    <div className="absolute inset-0 flex flex-col">
      <div className="flex min-h-11 flex-none flex-wrap items-center gap-x-1 gap-y-1 border-b border-(--border-rule) px-3 py-1.5">
        <span className="text-small text-soft tabular-nums">{comparing ? 'Pick two images' : `${items.length} ${items.length === 1 ? 'item' : 'items'}`}</span>
        <span className="ml-auto" />
        {comparing && (
          <button className="btn btn-primary h-7 rounded-full" disabled={picked.length !== 2} onClick={() => setShowCompare(true)}>
            Compare {picked.length} of 2
          </button>
        )}
        <ChatActionButton
          label={comparing ? 'Stop comparing' : 'Compare two images'}
          pressed={comparing}
          disabled={images.length < 2}
          onClick={() => {
            setComparing(!comparing)
            setPicked([])
          }}
        >
          <Columns2 size={14} aria-hidden="true" />
          {comparing ? 'Done' : 'Compare'}
        </ChatActionButton>
        <ChatActionButton
          label={newestFirst ? 'Show the oldest first' : 'Show the newest first'}
          onClick={() => {
            writeStored(ORDER_KEY, !newestFirst)
            setNewestFirst(!newestFirst)
          }}
        >
          <ArrowUpDown size={14} aria-hidden="true" />
          {newestFirst ? 'Newest first' : 'Oldest first'}
        </ChatActionButton>
      </div>
      <div className="grid min-h-0 flex-1 auto-rows-max grid-cols-[repeat(auto-fill,minmax(13rem,1fr))] content-start gap-3 overflow-y-auto p-3">
        {items.map((item) => (
          <EvidenceCard
            key={item.id}
            item={item}
            selectable={comparing && item.kind === 'image'}
            selected={picked.includes(item.id)}
            onOpen={() => setViewing(item.id)}
            onToggle={() => toggle(item.id)}
          />
        ))}
      </div>
      {viewing && <EvidenceViewer threadId={thread.id} items={images} startId={viewing} onClose={() => setViewing(null)} />}
      {showCompare && left && right && <EvidenceCompare left={left} right={right} onClose={() => setShowCompare(false)} />}
    </div>
  )
}
