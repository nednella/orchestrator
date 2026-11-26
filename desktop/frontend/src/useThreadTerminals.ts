import { useEffect, useRef } from 'react'
import { useAgentos } from './AgentosContext'
import { api } from './api'
import { askConfirm } from './confirm'
import { shellsOf } from './shells'
import { keepSessions, termSession } from './terminal/session'
import { changeTerms, savedTerms, setDrawerOpen, useSavedTerms } from './terminalDrawer'
import { activeShell, addGroup, closePane, keepRecent, MAX_PANES, reconcile, select, split } from './terminalGroups'
import type { ThreadTerms } from './terminalGroups'
import type { Shell } from './types'

// openTerminal starts a shell for the thread, '' for the project, and shows it in the drawer: in a group of its own,
// or beside the active terminal, with the keys. shells are the thread's shell ids now.
export async function openTerminal(thread: string, shells: string[], beside: boolean): Promise<string> {
  const before = reconcile(savedTerms(thread), shells)
  if (beside && (before.groups.find((g) => g.id === before.active)?.panes.length ?? 0) >= MAX_PANES) throw `A group holds ${MAX_PANES} terminals at most`
  setDrawerOpen(thread, true)
  const { id } = await api.shellNew(thread)
  changeTerms(thread, shells, (t) => (beside ? split(t, id) : addGroup(t, id)))
  termSession(id).focus()
  return id
}

// closeTerminal stops the shell, once the owner says so when something other than the shell runs in it.
export async function closeTerminal(thread: string, shells: string[], shell: Shell) {
  if (shell.busy) {
    const { ok } = await askConfirm({ title: 'Close this terminal?', body: [`${shell.command} is still running in it.`], confirm: 'Close', tone: 'danger' })
    if (!ok) return
  }
  await api.shellClose(shell.id)
  changeTerms(thread, shells, (t) => closePane(t, shell.id))
}

export type ThreadTerminals = {
  terms: ThreadTerms
  shells: Shell[]
  open(beside: boolean): Promise<string>
  close(shell: Shell): Promise<void>
  select(id: string): void
}

// useThreadTerminals is the thread's terminals, '' for Home's: its shells in the groups the owner made of them.
export function useThreadTerminals(thread: string): ThreadTerminals {
  const { shells: all } = useAgentos('shells')
  const ids = shellsOf(all, thread)
  const terms = reconcile(useSavedTerms(thread), ids)
  return {
    terms,
    shells: all.filter((s) => s.thread === thread),
    open: (beside) => openTerminal(thread, ids, beside),
    close: (shell) => closeTerminal(thread, ids, shell),
    select(id) {
      if (activeShell(terms) !== id) changeTerms(thread, ids, (t) => select(t, id))
    },
  }
}

// useKeptTerminals keeps the terminals of the threads opened last alive, so going back to one shows its terminals at
// once, and disposes the others'. Their shells run on in tmux.
export function useKeptTerminals(thread: string) {
  const { shells } = useAgentos('shells')
  const recent = useRef<string[]>([])
  useEffect(() => {
    recent.current = keepRecent(recent.current, thread)
    keepSessions(new Set(shells.filter((s) => recent.current.includes(s.thread)).map((s) => s.id)))
  }, [thread, shells])
}
