import type { PanelRow } from '../panelViews'

type PanelHomeRowProps = { row: PanelRow; reason: string | null; unseen?: number; onOpen(): void }

// PanelHomeRow is one view of the panel's list, with the count of what it holds that the owner has not seen. A shut row
// says why on hover, and neither a click nor a key opens it.
export function PanelHomeRow({ row, reason, unseen = 0, onOpen }: PanelHomeRowProps) {
  return (
    <button
      data-panel-view={row.label}
      aria-disabled={reason !== null}
      aria-keyshortcuts={row.key}
      title={reason ?? undefined}
      className="flex h-8 w-full flex-none items-center gap-2.5 rounded-lg px-2.5 text-left text-body hover:bg-(--bg-row-hover) focus-visible:bg-(--bg-row-hover) aria-disabled:cursor-default aria-disabled:opacity-50 aria-disabled:hover:bg-transparent"
      onClick={() => reason === null && onOpen()}
    >
      <row.icon size={16} className="flex-none" aria-hidden="true" />
      <span className="min-w-0 flex-1 truncate">{row.label}</span>
      {unseen > 0 && (
        <span className="flex-none text-small text-accent tabular-nums" aria-label={`${unseen} new`}>
          {unseen}
        </span>
      )}
      <kbd className="inline-flex h-5 min-w-5 flex-none items-center justify-center rounded-sm bg-(--bg-key-badge) px-1 text-small font-medium text-soft" aria-hidden="true">
        {row.key}
      </kbd>
    </button>
  )
}
