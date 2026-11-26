import { X } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

type PanelTabProps = { label: string; note?: string; tip?: string; icon: LucideIcon; active: boolean; onSelect(): void; onClose(): void }

// PanelTab is one tab of the right panel. Its icon turns into the button that closes it while the pointer or the focus
// is on the tab; the note, such as a folder, follows the label in a dimmer colour and gives way first.
export function PanelTab({ label, note, tip, icon: Icon, active, onSelect, onClose }: PanelTabProps) {
  return (
    <div
      role="presentation"
      title={tip}
      className={`group relative flex h-6 max-w-36 flex-none items-center rounded-md text-small ${active ? 'bg-(--bg-segment-on) text-ink' : 'text-soft hover:bg-(--bg-row-hover) hover:text-ink'}`}
      onMouseDown={(e) => e.button === 1 && e.preventDefault()}
      onAuxClick={(e) => e.button === 1 && onClose()}
    >
      <button role="tab" aria-selected={active} className="flex h-full min-w-0 items-baseline gap-1 rounded-md pr-2 pl-[1.375rem] leading-6" onClick={onSelect}>
        <span className="flex-none">{label}</span>
        {note && <span className="min-w-0 truncate text-(--text-terminal-count)">{note}</span>}
      </button>
      <span className="pointer-events-none absolute top-1/2 left-1 grid size-4 -translate-y-1/2 place-items-center group-focus-within:opacity-0 group-hover:opacity-0">
        <Icon size={12} aria-hidden="true" />
      </span>
      <button
        className="absolute top-1/2 left-1 grid size-4 -translate-y-1/2 place-items-center rounded-sm text-soft opacity-0 group-focus-within:opacity-100 group-hover:opacity-100 hover:bg-(--bg-segment-on) hover:text-ink"
        aria-label={`Close ${label}`}
        onClick={onClose}
      >
        <X size={12} aria-hidden="true" />
      </button>
    </div>
  )
}
