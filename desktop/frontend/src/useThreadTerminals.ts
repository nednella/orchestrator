import { useEffect, useRef } from 'react'
import { useAgentos } from './AgentosContext'
import { api } from './api'
import { askConfirm } from './confirm'
import { shellsOf } from './shells'
import { keepSessions, termSession } from './terminal/session'
import { changeTerms, savedTerms, setDrawerOpen, shellClosed, shellStarted, startedShells, useSavedTerms, useStartedShells } from './terminalDrawer'
import { activeShell, addGroup, closePane, keepRecent, MAX_PANES, reconcile, select, split } from './terminalGroups'
import type { ThreadTerms } from './terminalGroups'
import type { Shell } from './types'

// Splits asked for by thread whose shell has not started yet; they count against the group's room.
const splitting = new Map<string, number>()

// openTerminal starts a shell for the thread, '' for the project, and shows it in the drawer: in a group of its own,
// or beside the active terminal, with the keys. shells are the thread's shell ids now.
export async function openTerminal(thread: string, shells: string[], beside: boolean): Promise<string> {
  const now = reconcile(savedTerms(thread), [...shells, ...startedShells(thread)])
  const panes = now.groups.find((g) => g.id === now.active)?.panes.length ?? 0
  const asked = splitting.get(thread) ?? 0
  if (beside && panes + asked >= MAX_PANES) throw `A group holds ${MAX_PANES} terminals at most`
  setDrawerOpen(thread, true)
  if (beside) splitting.set(thread, asked + 1)
  const start = api.shellNew(thread).finally(() => {
    if (beside) splitting.set(thread, (splitting.get(thread) ?? 1) - 1)
  })
  const { id } = await start
  shellStarted(thread, id)
  // split leaves a group that filled in the meantime as it is; the shell then gets a group of its own.
  changeTerms(thread, (t) => {
    const placed = beside ? split(t, id) : t
    return placed === t ? addGroup(t, id) : placed
  })
  termSession(id).focus()
  return id
}

// closeTerminal stops the shell, once the owner says so when something other than the shell runs in it.
export async function closeTerminal(thread: string, shell: Shell) {
  if (shell.busy) {
    const { ok } = await askConfirm({ title: 'Close this terminal?', body: [`${shell.command} is still running in it.`], confirm: 'Close', tone: 'danger' })
    if (!ok) return
  }
  await api.shellClose(shell.id)
  shellClosed(shell.id)
  changeTerms(thread, (t) => closePane(t, shell.id))
}

export type ThreadTerminals = {
  terms: ThreadTerms
  shells: Shell[]
  open(beside: boolean): Promise<string>
  close(id: string): Promise<void>
  select(id: string): void
}

// useThreadTerminals is the thread's terminals, '' for Home's: its shells in the groups the owner made of them.
export function useThreadTerminals(thread: string): ThreadTerminals {
  const { shells: all } = useAgentos('shells')
  const ids = shellsOf(all, thread)
  useStartedShells()
  const terms = reconcile(useSavedTerms(thread), [...ids, ...startedShells(thread)])
  return {
    terms,
    shells: all.filter((s) => s.thread === thread),
    open: (beside) => openTerminal(thread, ids, beside),
    // A shell Go has not announced yet runs nothing but itself.
    close: (id) => closeTerminal(thread, all.find((s) => s.id === id) ?? { id, thread, command: '', busy: false }),
    select(id) {
      if (activeShell(terms) !== id) changeTerms(thread, (t) => select(t, id))
    },
  }
}

// useKeptTerminals keeps the terminals of the threads opened last alive, so going back to one shows its terminals at
// once, and disposes the others'. Their shells run on in tmux. A shell just started is kept until Go announces it.
export function useKeptTerminals(thread: string) {
  const { shells } = useAgentos('shells')
  const started = useStartedShells()
  const recent = useRef<string[]>([])
  useEffect(() => {
    recent.current = keepRecent(recent.current, thread)
    keepSessions(new Set([...shells.filter((s) => recent.current.includes(s.thread)).map((s) => s.id), ...startedShells()]))
  }, [thread, shells, started])
}
