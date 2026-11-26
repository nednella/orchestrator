import { useSyncExternalStore } from 'react'
import { check, current, enqueue, settle } from './confirmQueue'
import type { ConfirmQueue, ConfirmRequest } from './confirmQueue'

export type ConfirmAnswer = { ok: boolean; checked: boolean }

// How long an answered confirm takes to leave before the next one shows.
const LEAVE_MS = 200

let queue: ConfirmQueue = []
let leaving: number | null = null
let nextId = 1
const answers = new Map<number, (a: ConfirmAnswer) => void>()
const listeners = new Set<() => void>()

function publish(next: ConfirmQueue) {
  queue = next
  listeners.forEach((l) => l())
}

function subscribe(listener: () => void) {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

// askConfirm shows r in the confirm modal once the confirms before it are answered, and says whether the owner
// confirmed and whether the box was ticked.
export function askConfirm(r: Omit<ConfirmRequest, 'id'>): Promise<ConfirmAnswer> {
  const id = nextId++
  return new Promise((resolve) => {
    answers.set(id, resolve)
    publish(enqueue(queue, { ...r, id }))
  })
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
    publish(settle(queue, id))
  }, LEAVE_MS)
}

export function checkConfirm(id: number, checked: boolean) {
  publish(check(queue, id, checked))
}

// useConfirm is the confirm shown, and whether it is leaving.
export function useConfirm(): { request: ConfirmRequest | undefined; leaving: boolean } {
  const request = useSyncExternalStore(subscribe, () => current(queue))
  const leavingId = useSyncExternalStore(subscribe, () => leaving)
  return { request, leaving: request !== undefined && leavingId === request.id }
}

// useConfirming says whether a confirm shows, and changes only when that does.
export function useConfirming(): boolean {
  return useSyncExternalStore(subscribe, () => queue.length > 0)
}

// confirmed runs run once the owner confirms r, with whether they ticked its box.
export async function confirmed(r: Omit<ConfirmRequest, 'id'>, run: (checked: boolean) => unknown): Promise<void> {
  const { ok, checked } = await askConfirm(r)
  if (ok) await run(checked)
}
