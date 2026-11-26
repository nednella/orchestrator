import { useEffect, useRef } from 'react'
import { useLayout } from '../LayoutContext'
import { useOrchestrator } from '../OrchestratorContext'
import { formatShortcut } from '../actions'
import { NEW_TERMINAL_KEY } from '../actions/shell'
import { activateTerminal, closeTerminal, focusTerminal, selectTerminal, startTerminal } from '../projectTerminalOps'
import { activeShell, MAX_PANES, shownGroup, showsList } from '../terminalGroups'
import { useProjectTerminals } from '../useProjectTerminals'
import { Keycap } from './Keycap'
import { TerminalButtons } from './TerminalButtons'
import { TerminalList } from './TerminalList'
import { TerminalSplit } from './TerminalSplit'

// TerminalView is the Terminal surface of the right panel: the project's shown group of terminals, side by side or
// stacked, with its buttons over a lone terminal or its list at the side of two or more.
export function TerminalView() {
  const { report } = useOrchestrator('report')
  const { showPanelList } = useLayout()
  const { project, thread, terms, shells } = useProjectTerminals()
  const group = shownGroup(terms)
  const active = activeShell(terms)
  const groups = terms.groups.length
  const before = useRef(groups)

  useEffect(() => {
    focusTerminal(terms)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // The last terminal closed leaves nothing to show: the panel's list takes its place.
  useEffect(() => {
    if (before.current > 0 && groups === 0) showPanelList()
    before.current = groups
  }, [groups, showPanelList])

  const start = (placement: 'tab' | 'row' | 'column') => report(() => startTerminal(project, thread, shells, placement).then(() => {}))
  const buttons = (floating: boolean) => (
    <TerminalButtons
      floating={floating}
      full={(group?.panes.length ?? 0) >= MAX_PANES}
      onSplit={start}
      onNew={() => start('tab')}
      onClose={() => active && report(() => closeTerminal(shells, active))}
    />
  )

  if (!group) {
    return (
      <div className="absolute inset-0 flex flex-col items-center justify-center gap-3 text-small text-dim">
        No terminal sessions for this project yet.
        <button className="btn h-6 gap-2 bg-transparent px-2 text-label" onClick={() => start('tab')}>
          New terminal
          <Keycap>{formatShortcut(NEW_TERMINAL_KEY)}</Keycap>
        </button>
      </div>
    )
  }

  return (
    <div className="absolute inset-0 isolate flex">
      <TerminalSplit group={group} onActivate={activateTerminal} />
      {showsList(group) ? (
        <TerminalList group={group} shells={shells} buttons={buttons(false)} onSelect={selectTerminal} onClose={(id) => report(() => closeTerminal(shells, id))} />
      ) : (
        buttons(true)
      )}
    </div>
  )
}
