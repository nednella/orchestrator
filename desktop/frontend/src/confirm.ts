import { useSyncExternalStore } from 'react'
import { api } from './api'
import { check, current, enqueue, queuedWith, settle } from './confirmQueue'
import type { ConfirmQueue, ConfirmRequest } from './confirmQueue'
import { deleteRequest } from './deleteThread'
import type { DeletePlan, Thread } from './types'

export type ConfirmAnswer = { ok: boolean; checked: boolean }

// An Ask is a confirm to show, or a way to make it, which runs again when it shows after waiting, so it says how things
// stand then.
type Ask = Omit<ConfirmRequest, 'id'> | (() => Omit<ConfirmRequest, 'id'>)

// How long an answered confirm takes to leave before the next one shows.
const LEAVE_MS = 200

let queue: ConfirmQueue = []
let leaving: number | null = null
let nextId = 1
// nudge changes when the confirm shown is asked for again, so it takes the focus back.
let nudge = 0
const answers = new Map<number, (a: ConfirmAnswer) => void>()
const makers = new Map<number, () => Omit<ConfirmRequest, 'id'>>()
const listeners = new Set<() => void>()

function publish(next: ConfirmQueue) {
  queue = next
  listeners.forEach((l) => l())
}

function subscribe(listener: () => void) {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

// askConfirm shows the confirm in the modal once the confirms before it are answered, and says whether the owner
// confirmed and whether the box was ticked. A confirm with the key of one already queued is not queued again: it
// answers no at once, and the one shown takes the focus back.
export function askConfirm(ask: Ask): Promise<ConfirmAnswer> {
  const r = typeof ask === 'function' ? ask() : ask
  if (r.key !== undefined && asked(r.key)) return Promise.resolve({ ok: false, checked: false })
  const id = nextId++
  if (typeof ask === 'function' && queue.length > 0) makers.set(id, ask)
  return new Promise((resolve) => {
    answers.set(id, resolve)
    publish(enqueue(queue, { ...r, id }))
  })
}

// asked says whether a confirm about key is queued, and gives it the focus back when it is the one shown.
function asked(key: string): boolean {
  const open = queuedWith(queue, key)
  if (open && current(queue)?.id === open.id) {
    nudge++
    publish(queue)
  }
  return open !== undefined
}

// showNext takes the answered confirm id out and makes the next one again when it has a maker.
function showNext(id: number) {
  const next = settle(queue, id)
  const shown = current(next)
  const make = shown && makers.get(shown.id)
  if (!shown || !make) return publish(next)
  makers.delete(shown.id)
  publish(next.map((r) => (r.id === shown.id ? { ...make(), id: shown.id } : r)))
}

// answerConfirm answers the confirm id; it leaves before the next one shows. A second answer is ignored.
export function answerConfirm(id: number, ok: boolean) {
  const resolve = answers.get(id)
  if (!resolve) return
  answers.delete(id)
  resolve({ ok, checked: ok && (queue.find((r) => r.id === id)?.checkbox?.checked ?? false) })
  leaving = id
  publish(queue)
  setTimeout(() => {
    leaving = null
    showNext(id)
  }, LEAVE_MS)
}

export function checkConfirm(id: number, checked: boolean) {
  publish(check(queue, id, checked))
}

// useConfirm is the confirm shown, whether it is leaving, and a count that changes when it is asked for again.
export function useConfirm(): { request: ConfirmRequest | undefined; leaving: boolean; nudge: number } {
  const request = useSyncExternalStore(subscribe, () => current(queue))
  const leavingId = useSyncExternalStore(subscribe, () => leaving)
  const nudged = useSyncExternalStore(subscribe, () => nudge)
  return { request, leaving: request !== undefined && leavingId === request.id, nudge: nudged }
}

// useConfirming says whether a confirm shows, and changes only when that does.
export function useConfirming(): boolean {
  return useSyncExternalStore(subscribe, () => queue.length > 0)
}

// confirmed runs run once the owner confirms, with whether they ticked the box.
export async function confirmed(ask: Ask, run: (checked: boolean) => unknown): Promise<void> {
  const { ok, checked } = await askConfirm(ask)
  if (ok) await run(checked)
}

// confirmDelete asks before the thread goes, with what goes with it, and deletes it on yes; one confirm per thread.
// threads is the latest list, read when the confirm shows, so it says whether the delete stops the thread. When the
// app cannot say what goes with it, the confirm offers no worktree.
export async function confirmDelete(thread: Thread, threads: () => Thread[], remove: (id: string, worktree: boolean) => Promise<void>): Promise<void> {
  const key = `delete:${thread.id}`
  if (asked(key)) return
  const plan = await api.deletePlan(thread.id).catch((): DeletePlan => ({ worktrees: 0, children: [] }))
  await confirmed(() => ({ ...deleteRequest(threads().find((s) => s.id === thread.id) ?? thread, plan), key }), (worktree) => remove(thread.id, worktree))
}
