import type { Limits } from '../types'
import { usageRows } from '../usage'
import { UsageLimitCard } from './UsageLimitCard'

type ProviderLimitsProps = { limits: Limits; now: number }

// ProviderLimits is one provider's block of the Usage modal: its name and its plan windows.
export function ProviderLimits({ limits, now }: ProviderLimitsProps) {
  const rows = usageRows(limits, now)
  return (
    <section className="flex flex-col gap-2" aria-label={`${limits.providerLabel} plan limits`}>
      <h3 className="text-small font-medium text-soft">{limits.providerLabel}</h3>
      {rows.length > 0 && (
        <ul className="flex flex-col gap-2">
          {rows.map((row) => (
            <UsageLimitCard key={row.label} row={row} />
          ))}
        </ul>
      )}
      {limits.unsupported && <p className="text-small text-dim">This {limits.providerLabel} does not report plan limits. The windows show what its latest turn said.</p>}
      {limits.error && (
        <p className="text-small text-danger" title={limits.error}>
          Could not read plan limits.
        </p>
      )}
      {rows.length === 0 && !limits.error && <p className="text-small text-dim">No plan limits yet. Refresh to read them.</p>}
    </section>
  )
}
