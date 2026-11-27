import { useOrchestrator } from '../OrchestratorContext'
import { gigabytes, memoryNote } from '../memory'
import { useDelayedTip } from '../useDelayedTip'
import { FloatingTip } from './FloatingTip'

export function MemoryTotal() {
  const { memory } = useOrchestrator('memory')
  const { anchor, tipHandlers } = useDelayedTip()
  if (!memory || memory.total === 0) return null
  const machine = memory.machine > 0 ? `\nof ${Math.round(memory.machine / 2 ** 30)} GB` : ''
  const parts = [`orchestrator ${gigabytes(memory.app)}`, `Browsers ${gigabytes(memory.browsers)}`, `Threads and shells ${gigabytes(memory.threads)}`]
  const tip = `${parts.join('\n')}${machine}\n\n${memoryNote}`
  return (
    <span className="flex-none cursor-default px-2 tabular-nums text-small whitespace-nowrap text-dim" {...tipHandlers}>
      mem <span className="text-soft">{gigabytes(memory.total)}</span>
      <span className="sr-only">{tip}</span>
      {anchor && <FloatingTip anchor={anchor} text={tip} below />}
    </span>
  )
}
