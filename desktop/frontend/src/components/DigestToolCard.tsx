import { Package } from 'lucide-react'
import { useId } from 'react'
import { isNewItem } from '../digestFeed'
import type { DigestTool } from '../digestFeed'
import { DigestItemRow } from './DigestItemRow'
import { DigestTile } from './DigestTile'

type DigestToolCardProps = { tool: DigestTool; seen: number }

export function DigestToolCard({ tool, seen }: DigestToolCardProps) {
  const heading = useId()
  return (
    <section className="scroll-mt-4 rounded-xl bg-[var(--bg-card)] shadow-[inset_0_0_0_1px_var(--border)]" data-tool={tool.key} aria-labelledby={heading}>
      <header className="grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-x-3 gap-y-1 border-b border-line px-3.5 py-3 @max-[40rem]:grid-cols-[auto_minmax(0,1fr)]">
        <DigestTile size="sm">
          <Package size={14} aria-hidden />
        </DigestTile>
        <div className="min-w-0">
          <h2 id={heading} className="truncate text-body leading-[1.375] font-semibold">
            {tool.name}
          </h2>
          <p className="text-small text-soft">{tool.items.length === 1 ? '1 item' : `${tool.items.length} items`}</p>
        </div>
        {tool.fresh > 0 && (
          <div className="flex flex-wrap items-center justify-end gap-x-3 gap-y-1 @max-[40rem]:col-start-2 @max-[40rem]:justify-start">
            <span className="inline-flex h-[1.375rem] items-center gap-1 rounded-full px-2 text-small font-medium whitespace-nowrap text-soft shadow-[inset_0_0_0_1px_var(--border-strong)]">
              <span className="h-1.5 w-1.5 rounded-full bg-accent" aria-hidden="true" />
              {tool.fresh} new
            </span>
          </div>
        )}
      </header>
      <ol className="flex flex-col gap-0.5 p-1.5">
        {tool.items.map((item) => (
          <DigestItemRow key={item.id} item={item} fresh={isNewItem(item, seen)} />
        ))}
      </ol>
    </section>
  )
}
