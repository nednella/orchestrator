import { RefreshCw, X } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useOrchestrator } from '../OrchestratorContext'
import { keepFocusIn } from '../focusTrap'
import { dollars, tokenCount } from '../spend'
import { closeUsage, useUsageOpen } from '../usageOpen'
import { usageRows } from '../usage'
import { useUsage } from '../useUsage'
import { UsageLimitCard } from './UsageLimitCard'

const ICON_BUTTON = 'inline-flex h-[28px] w-[28px] items-center justify-center rounded-[8px] text-soft hover:bg-hover hover:text-ink disabled:opacity-60'

// UsageModal shows the plan limits, read again each time it opens, and what today cost.
export function UsageModal() {
  return useUsageOpen() ? <Usage /> : null
}

function Usage() {
  const { focus } = useOrchestrator('focus')
  const { limits, today, reading, error, refresh } = useUsage()
  const dialog = useRef<HTMLDivElement>(null)
  const close = useRef<HTMLButtonElement>(null)
  const [now, setNow] = useState(Date.now)
  const focusRef = useRef(focus)
  focusRef.current = focus
  const rows = limits ? usageRows(limits, now) : []

  useEffect(() => {
    const tick = setInterval(() => setNow(Date.now()), 30_000)
    return () => clearInterval(tick)
  }, [])

  useEffect(() => {
    const opener = document.activeElement
    close.current?.focus()
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault()
        e.stopImmediatePropagation()
        closeUsage()
      } else if (e.key === 'Tab') keepFocusIn(dialog.current, e)
    }
    window.addEventListener('keydown', onKey, true)
    return () => {
      window.removeEventListener('keydown', onKey, true)
      if (opener instanceof HTMLElement && opener.isConnected && opener !== document.body) opener.focus()
      else focusRef.current('thread')
    }
  }, [])

  return (
    <div className="confirm-back" onMouseDown={(e) => e.target === e.currentTarget && closeUsage()}>
      <div ref={dialog} className="confirm-dialog" role="dialog" aria-modal="true" aria-label="Usage">
        <div className="usage-head">
          <div className="flex items-center justify-between">
            <h2 className="confirm-title">Usage</h2>
            <div className="flex items-center gap-1">
              <button className={ICON_BUTTON} aria-label="Refresh usage" title="Refresh" disabled={reading} onClick={() => void refresh()}>
                <RefreshCw size={16} aria-hidden="true" className={reading ? 'animate-spin' : undefined} />
              </button>
              <button ref={close} className={ICON_BUTTON} aria-label="Close" onClick={closeUsage}>
                <X size={16} aria-hidden="true" />
              </button>
            </div>
          </div>
          <div className="mt-2 flex flex-col gap-5">
            <section className="flex flex-col gap-2" aria-label="Plan limits">
              <h3 className="text-small font-medium text-soft">Plan limits</h3>
              {rows.length > 0 && (
                <ul className="flex flex-col gap-2">
                  {rows.map((row) => (
                    <UsageLimitCard key={row.label} row={row} />
                  ))}
                </ul>
              )}
              {limits?.unsupported && <p className="text-small text-dim">This Claude Code does not report plan limits. The windows show what its latest turn said.</p>}
              {error && (
                <p className="text-small text-danger" title={error}>
                  Could not read plan limits.
                </p>
              )}
              {rows.length === 0 && !error && <p className="text-small text-dim">{reading || !limits ? 'Reading plan limits…' : 'No plan limits yet. Refresh to read them.'}</p>}
            </section>
            <section className="flex flex-col gap-1" aria-label="Today">
              <h3 className="text-small font-medium text-soft">Today</h3>
              <p className="min-h-[1.5em] text-body text-ink">{today && `${dollars(today.cost)} · ${tokenCount(today.tokens)} tokens`}</p>
            </section>
          </div>
        </div>
      </div>
    </div>
  )
}
