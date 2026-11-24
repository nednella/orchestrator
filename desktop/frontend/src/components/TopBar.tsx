import type { CSSProperties } from 'react'
import { useAgentos, useLatestAgentos } from '../AgentosContext'
import { actionsFor, formatShortcut } from '../actions'
import { useLayout } from '../LayoutContext'
import type { State } from '../types'
import { useFullscreen } from '../useFullscreen'
import { Icon } from './Icon'
import { StateDot } from './StateDot'

const drag = { '--wails-draggable': 'drag' } as CSSProperties
const noDrag = { '--wails-draggable': 'no-drag' } as CSSProperties

const DOTS_MIN = 640
const HELP_MIN = 560

type CountProps = { state: State; count: number; label: string }

function Count({ state, count, label }: CountProps) {
  return (
    <span className="flex items-center gap-1.5 text-body" style={{ opacity: count > 0 ? 1 : 0.5 }} title={`${count} ${label}`}>
      <StateDot state={state} />
      <span className="mono font-semibold" style={{ color: count > 0 ? 'var(--c)' : undefined }} data-state={state}>
        {count}
      </span>
    </span>
  )
}

export function TopBar() {
  const { sessions, setOverlay, report, update, updating, applyUpdate } = useAgentos('sessions', 'setOverlay', 'report', 'update', 'updating', 'applyUpdate')
  const layout = useLayout()
  const { width, sidebarOpen } = layout
  const latestAgentos = useLatestAgentos()
  const fullscreen = useFullscreen()
  const count = (state: State) => sessions.filter((s) => s.state === state).length
  const waiting = count('waiting')
  const idle = count('idle')
  const find = (id: string) => actionsFor(latestAgentos(), layout).find((a) => a.id === id)!
  const keys = (id: string) => formatShortcut(find(id).shortcut!)
  const iconButton = 'btn btn-ghost h-[32px] w-[32px] justify-center px-0'
  const badge = width < DOTS_MIN

  return (
    <header
      className="relative flex flex-none items-center border-b border-line"
      style={{ ...drag, height: 'var(--topbar-h)', paddingLeft: fullscreen ? 'var(--corner-pad)' : 'var(--traffic-light-zone)', paddingRight: 'var(--corner-pad)' }}
    >
      <div className="flex min-w-0 items-center gap-3" style={noDrag}>
        <button
          className={iconButton}
          title={`${sidebarOpen ? 'Hide' : 'Show'} thread list (${keys('toggle-sidebar')})`}
          aria-label={sidebarOpen ? 'Hide thread list' : 'Show thread list'}
          aria-pressed={sidebarOpen}
          onClick={() => find('toggle-sidebar').run([])}
        >
          <Icon name="panel-left" />
        </button>
        {badge ? (
          <span
            className="mono grid h-6 min-w-6 place-items-center rounded-sm border px-1.5 text-small font-semibold"
            data-state="waiting"
            style={{ color: waiting > 0 ? 'var(--c)' : 'var(--text-dim)', borderColor: waiting > 0 ? 'var(--c)' : 'var(--border-strong)' }}
            title={`${waiting} need you, ${count('working')} working, ${idle} idle`}
          >
            {waiting}
          </span>
        ) : (
          <>
            <Count state="waiting" count={waiting} label="need you" />
            <Count state="working" count={count('working')} label="working" />
            <Count state="idle" count={idle} label="idle" />
          </>
        )}
      </div>
      <div className="ml-auto flex items-center justify-end gap-1" style={noDrag}>
        {update && (
          <button
            className={`${iconButton} relative`}
            style={{ color: 'var(--accent)' }}
            disabled={updating}
            title={updating ? `Installing agentos v${update}…` : `Install agentos v${update} and relaunch`}
            aria-label={updating ? 'Updating' : `Update to v${update}`}
            onClick={() => report(applyUpdate)}
          >
            {updating ? (
              <span className="spinner" role="status" />
            ) : (
              <>
                <Icon name="update" />
                <span className="absolute top-[5px] right-[5px] h-[7px] w-[7px] rounded-full bg-accent" />
              </>
            )}
          </button>
        )}
        <button className={iconButton} title={`Next that needs you (${keys('next-attention')})`} aria-label="Next that needs you" onClick={() => find('next-attention').run([])}>
          <Icon name="next" />
        </button>
        <button className={iconButton} onClick={() => setOverlay('palette')} title={`Command palette (${keys('palette')})`} aria-label="Command palette">
          <Icon name="search" />
        </button>
        {width >= HELP_MIN && (
          <button className={iconButton} onClick={() => setOverlay('shortcuts')} title={`Keyboard shortcuts (${keys('shortcuts')})`} aria-label="Keyboard shortcuts">
            <Icon name="help" />
          </button>
        )}
      </div>
    </header>
  )
}
