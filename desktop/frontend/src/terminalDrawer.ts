import { useRef, useSyncExternalStore } from 'react'
import type { PointerEvent, RefObject } from 'react'
import { readStored, writeStored } from './storage'
import { reconcile, TERMINALS_KEY } from './terminalGroups'
import type { ThreadTerms } from './terminalGroups'

// Each thread's drawer, '' for Home's: whether it is open, the height it was dragged to in px, and its terminals.
const OPEN_KEY = 'agentos.terminalOpen'
const HEIGHT_KEY = 'agentos.terminalHeight'

export const DEFAULT_HEIGHT_PX = 280
export const MIN_HEIGHT_PX = 180

type Saved<T> = Record<string, T>

let opened = readStored<Saved<boolean>>(OPEN_KEY, {})
let heights = readStored<Saved<number>>(HEIGHT_KEY, {})
let terms = readStored<Saved<ThreadTerms>>(TERMINALS_KEY, {})
const listeners = new Set<() => void>()

function subscribe(listener: () => void) {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

const changed = () => listeners.forEach((listener) => listener())

// put sets thread's value in a saved map; undefined drops it, so the maps hold only the threads that differ.
function put<T>(all: Saved<T>, key: string, thread: string, value: T | undefined): Saved<T> {
  const { [thread]: _, ...rest } = all
  const next = value === undefined ? rest : { ...rest, [thread]: value }
  writeStored(key, next)
  return next
}

export function useDrawerOpen(thread: string): boolean {
  return useSyncExternalStore(subscribe, () => opened[thread] === true)
}

export function drawerOpen(thread: string): boolean {
  return opened[thread] === true
}

export function setDrawerOpen(thread: string, open: boolean) {
  if (drawerOpen(thread) === open) return
  opened = put(opened, OPEN_KEY, thread, open ? true : undefined)
  changed()
}

export function useDrawerHeight(thread: string): number {
  return useSyncExternalStore(subscribe, () => heights[thread] ?? DEFAULT_HEIGHT_PX)
}

export function saveDrawerHeight(thread: string, px: number) {
  heights = put(heights, HEIGHT_KEY, thread, Math.round(px))
  changed()
}

export function useSavedTerms(thread: string): ThreadTerms | undefined {
  return useSyncExternalStore(subscribe, () => terms[thread])
}

export function savedTerms(thread: string): ThreadTerms | undefined {
  return terms[thread]
}

// changeTerms changes the thread's terminals as the shells running now show them, and saves them.
export function changeTerms(thread: string, shells: string[], change: (t: ThreadTerms) => ThreadTerms) {
  const next = change(reconcile(terms[thread], shells))
  terms = put(terms, TERMINALS_KEY, thread, next.groups.length > 0 ? next : undefined)
  changed()
}

const MAX_WINDOW_SHARE = 0.75

// The tallest the drawer gets: three quarters of the window, and never over the composer above it.
function maxHeight(drawer: HTMLElement): number {
  const column = drawer.parentElement!
  const composer = column.querySelector('.composer-box')?.getBoundingClientRect().height ?? 0
  return Math.max(MIN_HEIGHT_PX, Math.min(MAX_WINDOW_SHARE * window.innerHeight, column.clientHeight - composer))
}

// useDrawerDrag resizes the drawer from a strip on its top edge. The drag sets only the drawer's --terminal-height, so
// nothing renders while it moves; the height is saved once it ends, unless it ends where it began.
export function useDrawerDrag(thread: string, drawer: RefObject<HTMLDivElement>) {
  const drag = useRef<{ y: number; from: number; max: number } | null>(null)
  const heightAt = (e: PointerEvent) => {
    const { y, from, max } = drag.current!
    return Math.min(Math.max(from + y - e.clientY, MIN_HEIGHT_PX), max)
  }
  const end = (e: PointerEvent) => {
    if (!drag.current) return
    const moved = e.clientY !== drag.current.y
    const next = heightAt(e)
    drag.current = null
    if (moved) saveDrawerHeight(thread, next)
    else drawer.current!.style.setProperty('--terminal-height', `${heights[thread] ?? DEFAULT_HEIGHT_PX}px`)
  }
  return {
    onPointerDown(e: PointerEvent<HTMLElement>) {
      e.currentTarget.setPointerCapture(e.pointerId)
      drag.current = { y: e.clientY, from: drawer.current!.offsetHeight, max: maxHeight(drawer.current!) }
    },
    onPointerMove(e: PointerEvent) {
      if (drag.current) drawer.current!.style.setProperty('--terminal-height', `${heightAt(e)}px`)
    },
    onPointerUp: end,
    onPointerCancel: end,
  }
}
