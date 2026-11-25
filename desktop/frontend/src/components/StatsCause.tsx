import { workTime } from '../statsFigures'
import { KIND_COLOR, KIND_ICON, KIND_LABEL } from '../statsMeta'
import type { Stats } from '../types'

type StatsCauseProps = { cause: Stats['byCause'][number] }

export function StatsCause({ cause }: StatsCauseProps) {
  const KindIcon = KIND_ICON[cause.kind]
  const tool = cause.kind === 'permission' && cause.label !== 'Permission'
  return (
    <li className="grid min-h-9 grid-cols-[1rem_minmax(0,1fr)_auto] items-center gap-2 border-t border-line text-small">
      <span role="img" aria-label={KIND_LABEL[cause.kind]} className="flex" style={{ color: KIND_COLOR[cause.kind] }}>
        <KindIcon size={14} aria-hidden="true" />
      </span>
      <span className={`truncate ${tool ? 'mono' : ''}`} title={cause.label}>
        {cause.label}
      </span>
      <span className="tabular-nums">
        {cause.count.toLocaleString()} <span className="text-soft">· {workTime(cause.totalWaitMs)}</span>
      </span>
    </li>
  )
}
