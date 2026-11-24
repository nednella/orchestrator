import type { CSSProperties } from 'react'
import { useAgentos, useLatestAgentos } from '../AgentosContext'
import { actionsFor, formatShortcut } from '../actions'
import { RAIL_REM, useLayout } from '../LayoutContext'
import { useFullscreen } from '../useFullscreen'
import { Icon } from './Icon'
import { TitleBarButton, titleBarButton } from './TitleBarButton'
import { TitleBreadcrumb } from './TitleBreadcrumb'

const drag = { '--wails-draggable': 'drag' } as CSSProperties
const noDrag = { '--wails-draggable': 'no-drag' } as CSSProperties

export function TopBar() {
  const { selectedId, setOverlay, report, update, updating, applyUpdate } = useAgentos('selectedId', 'setOverlay', 'report', 'update', 'updating', 'applyUpdate')
  const layout = useLayout()
  const { mode, mobilePanel, sidebarOpen, shellOpen, rightOpen, widths, statsOpen, newsOpen } = layout
  const latestAgentos = useLatestAgentos()
  const fullscreen = useFullscreen()
  const find = (id: string) => actionsFor(latestAgentos(), layout).find((a) => a.id === id)!
  const named = (label: string, id: string) => {
    const shortcut = find(id).shortcut
    return shortcut ? `${label} (${formatShortcut(shortcut)})` : label
  }
  const narrow = mode === 'narrow'
  const showsThread = selectedId !== null && !statsOpen && !newsOpen && !(narrow && (mobilePanel === 'queue' || mobilePanel === 'notes'))
  const inset = fullscreen ? 'var(--corner-pad)' : 'var(--traffic-light-zone)'
  const toggleZone = `calc(${inset} + 40px)`
  const left = sidebarOpen && !narrow ? `max(${toggleZone}, calc(${RAIL_REM + widths.sidebar}rem + 2px))` : toggleZone

  return (
    <header className="relative flex flex-none items-center border-b border-line" style={{ ...drag, height: 'var(--topbar-h)', paddingRight: 'var(--corner-pad)' }}>
      <div className="flex flex-none items-center" style={{ width: left, paddingLeft: inset }}>
        <span style={noDrag}>
          <TitleBarButton label={named(sidebarOpen ? 'Hide thread list' : 'Show thread list', 'toggle-sidebar')} icon="panel-left" pressed={sidebarOpen} onClick={() => find('toggle-sidebar').run([])} />
        </span>
      </div>
      <div className="flex min-w-0 flex-1 items-center px-3">
        <TitleBreadcrumb showsThread={showsThread} />
      </div>
      <div className="flex flex-none items-center gap-0.5" style={noDrag}>
        {update && (
          <button
            className={`${titleBarButton} relative`}
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
        <TitleBarButton label={named('Command palette', 'palette')} icon="search" onClick={() => setOverlay('palette')} />
        <TitleBarButton label={named(shellOpen ? 'Hide shell' : 'Show shell', 'toggle-shell')} icon="panel-bottom" pressed={shellOpen} onClick={() => find('toggle-shell').run([])} />
        <TitleBarButton
          label={rightOpen || mobilePanel === 'details' ? 'Hide right panel' : 'Show right panel'}
          icon="panel-right"
          pressed={narrow ? mobilePanel === 'details' : rightOpen}
          disabled={!showsThread}
          onClick={() => report(() => find('toggle-right').run([]))}
        />
      </div>
    </header>
  )
}
