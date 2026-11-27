import { api } from './api'
import { askConfirm } from './confirm'
import { changeTerms, savedTerms, shellClosed, shellStarted, startedShells } from './projectTerminals'
import { shellTerminal } from './terminal/shellTerminal'
import { activeShell, addGroup, closePane, MAX_PANES, reconcile, select, selectGroup, shownGroup, split, terminalLabel, terminalNumber } from './terminalGroups'
import type { ProjectTerms, SplitDir } from './terminalGroups'
import type { Shell } from './types'

// Where a new terminal goes: a group of its own, which makes a tab, or beside the active terminal of the shown group.
export type Placement = 'tab' | SplitDir

// The terminals asked for beside the shown group's whose shell has not started yet; they count against its room.
let splitting = 0

// termsOf is the project's terminals as its running shells, and the ones just started, allow.
export function termsOf(project: string, shells: Shell[]): ProjectTerms {
  return reconcile(savedTerms(project), [...shells.map((s) => s.id), ...startedShells()])
}

// startTerminal starts a shell in the project and shows it with the keys. thread is the one that picks the folder it
// starts in, '' for the project's folder.
export async function startTerminal(project: string, thread: string, shells: Shell[], placement: Placement): Promise<string> {
  const beside = placement !== 'tab'
  const room = shownGroup(termsOf(project, shells))?.panes.length ?? 0
  if (beside && room + splitting >= MAX_PANES) throw `A group holds ${MAX_PANES} terminals at most`
  if (beside) splitting++
  const start = api.shellNew(thread).finally(() => {
    if (beside) splitting--
  })
  const { id } = await start
  shellStarted(id)
  // A split leaves a group that filled in the meantime as it is; the shell then gets a group of its own.
  changeTerms((t) => {
    const placed = beside ? split(t, id, placement) : t
    return placed === t ? addGroup(t, id) : placed
  })
  shellTerminal(id).focus()
  return id
}

function shellOf(shells: Shell[], id: string): Shell {
  // A shell Go has not announced yet runs nothing but itself.
  return shells.find((s) => s.id === id) ?? { id, folder: '', command: '', busy: false }
}

// stopShells stops the shells and takes their panes out.
async function stopShells(ids: string[]) {
  for (const id of ids) {
    await api.shellClose(id)
    shellClosed(id)
  }
  changeTerms((t) => ids.reduce(closePane, t))
}

// closeTerminal stops the terminal's shell once the owner says so.
export async function closeTerminal(shells: Shell[], id: string) {
  const shell = shellOf(shells, id)
  const { ok } = await askConfirm({
    title: 'Close this terminal?',
    body: [shell.busy ? `${shell.command} is still running in it.` : 'Its shell ends with it.'],
    confirm: 'Close',
    tone: 'danger',
  })
  if (ok) await stopShells([id])
}

// closeTab stops every terminal of the group, once the owner says so when one runs a command or there are several.
export async function closeTab(shells: Shell[], terms: ProjectTerms, group: string) {
  const ids = terms.groups.find((g) => g.id === group)?.panes ?? []
  const busy = ids.map((id) => shellOf(shells, id)).filter((s) => s.busy)
  if (busy.length > 0 || ids.length > 1) {
    const { ok } = await askConfirm({
      title: ids.length > 1 ? `Close these ${ids.length} terminals?` : 'Close this terminal?',
      body: busy.length > 0 ? [`${busy.map((s) => s.command).join(', ')} is still running in it.`] : ['Their shells end with them.'],
      confirm: 'Close',
      tone: 'danger',
    })
    if (!ok) return
  }
  await stopShells(ids)
}

// activateTerminal makes the terminal the active one of its group, and shows the group.
export function activateTerminal(id: string) {
  changeTerms((t) => select(t, id))
}

export function selectTerminal(id: string) {
  activateTerminal(id)
  shellTerminal(id).focus()
}

export function selectTab(group: string) {
  changeTerms((t) => selectGroup(t, group))
}

// focusTerminal gives the keys to the shown group's active terminal.
export function focusTerminal(terms: ProjectTerms) {
  const id = activeShell(terms)
  if (id) shellTerminal(id).focus()
}

// labelOf is the name of the terminal: the process running in it, else its number.
export function labelOf(shells: Shell[], id: string): string {
  const { command, busy } = shellOf(shells, id)
  return terminalLabel(busy ? command : '', terminalNumber(id))
}
