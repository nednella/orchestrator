import type { CSSProperties } from 'react'
import { useAgentos, useLatestAgentos } from '../AgentosContext'
import { actionsFor, formatShortcut } from '../actions'
import { useLayout } from '../LayoutContext'
import { Icon } from './Icon'
import { TitleBarButton, titleBarButton } from './TitleBarButton'
import { TitleBreadcrumb } from './TitleBreadcrumb'

const drag = { '--wails-draggable': 'drag' } as CSSProperties
const noDrag = { '--wails-draggable': 'no-drag' } as CSSProperties

export function TopBar() {
  const { selectedId, report, update, updating, applyUpdate, digestUnseenCount } = useAgentos('selectedId', 'report', 'update', 'updating', 'applyUpdate', 'digestUnseenCount')
  const layout = useLayout()
  const { mode, mobilePanel, sidebarOpen, shellOpen, rightOpen, statsOpen, digestOpen } = layout
  const latestAgentos = useLatestAgentos()
  const find = (id: string) => actionsFor(latestAgentos(), layout).find((a) => a.id === id)!
  const named = (label: string, id: string) => {
    const shortcut = find(id).shortcut
    return shortcut ? `${label} (${formatShortcut(shortcut)})` : label
  }
  const narrow = mode === 'narrow'
  const rightShown = narrow ? mobilePanel === 'panel' : rightOpen
  const showsThread = selectedId !== null && !statsOpen && !digestOpen
  // With the sidebar hidden the bar starts under the window buttons.
  const left = sidebarOpen ? '0px' : 'calc(var(--title-inset) - 0.75rem)'

  return (
    <header className="relative flex flex-none items-center" style={{ ...drag, height: 'var(--topbar-h)', paddingLeft: left, paddingRight: 'var(--corner-pad)' }}>
      <div className="flex min-w-0 flex-1 items-center px-3">
        <TitleBreadcrumb showsThread={showsThread} />
      </div>
      <div className="flex flex-none items-center gap-0.5" style={noDrag}>
        <TitleBarButton label={named('Stats', 'stats')} icon="chart" pressed={statsOpen} onClick={() => find('stats').run([])} />
        <TitleBarButton label={named('Digest', 'digest')} icon="digest" pressed={digestOpen} unseen={digestUnseenCount} onClick={() => find('digest').run([])} />
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
        <TitleBarButton label={named(sidebarOpen ? 'Hide left sidebar' : 'Show left sidebar', 'toggle-sidebar')} icon="panel-left" pressed={sidebarOpen} onClick={() => find('toggle-sidebar').run([])} />
        <TitleBarButton label={named(shellOpen ? 'Hide shell' : 'Show shell', 'toggle-shell')} icon="panel-bottom" pressed={shellOpen} onClick={() => find('toggle-shell').run([])} />
        <TitleBarButton
          label={named(rightShown ? 'Hide right panel' : 'Show right panel', 'toggle-right')}
          icon="panel-right"
          pressed={rightShown}
          onClick={() => report(() => find('toggle-right').run([]))}
        />
      </div>
    </header>
  )
}
