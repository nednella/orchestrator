import { useEffect, useRef, useSyncExternalStore } from 'react'
import type { PointerEvent, RefObject } from 'react'
import { shellsOf } from './shells'
import { readStored, writeStored } from './storage'
import { prune, TERMINALS_KEY, withShells } from './terminalGroups'
import type { ThreadTerms } from './terminalGroups'
import { START_GRACE_MS, stillStarted } from './startedShells'
import type { Started } from './startedShells'
import type { Shell } from './types'

// Each thread's drawer, '' for Home's: whether it is open, the height it was dragged to in px, and its terminals.
const OPEN_KEY = 'agentos.terminalOpen'
const HEIGHT_KEY = 'agentos.terminalHeight'

export const DEFAULT_HEIGHT_PX = 280
export const MIN_HEIGHT_PX = 180

type Saved<T> = Record<string, T>

let opened = readStored<Saved<boolean>>(OPEN_KEY, {})
let heights = readStored<Saved<number>>(HEIGHT_KEY, {})
let terms = readStored<Saved<ThreadTerms>>(TERMINALS_KEY, {})
// live are the current project's shells as Go last announced them; started are shells whose start returned before
// their first announce, which an announce sent before the start can come ahead of.
let live: Shell[] = []
let project = ''
let started: Started = new Map()
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

// changeTerms changes the thread's terminals and saves them. Only an announce of the shells takes a pane out.
export function changeTerms(thread: string, change: (t: ThreadTerms) => ThreadTerms) {
  const next = change(withShells(terms[thread], shellsOf(live, thread)))
  terms = put(terms, TERMINALS_KEY, thread, next.groups.length > 0 ? next : undefined)
  changed()
}

// shellStarted keeps a new shell's pane until Go announces the shell, or the grace passes.
export function shellStarted(thread: string, id: string) {
  if (live.some((s) => s.id === id)) return
  started = new Map(started).set(id, { thread, project, at: Date.now() })
  setTimeout(() => settle(), START_GRACE_MS + 50)
}

// shellClosed forgets a started shell closed before Go announced it.
export function shellClosed(id: string) {
  if (!started.has(id)) return
  started = new Map(started)
  started.delete(id)
}

// startedShells are the shells started that Go has not announced yet: the thread's, or with no thread every one.
export function startedShells(thread?: string): string[] {
  return [...started].filter(([, s]) => thread === undefined || s.thread === thread).map(([id]) => id)
}

// useStartedShells re-renders when the started shells change, as when the grace ends one.
export function useStartedShells(): string {
  return useSyncExternalStore(subscribe, () => startedShells().join())
}

function settle() {
  const left = stillStarted(started, project, new Set(live.map((s) => s.id)), Date.now())
  if (left.size === started.size) return
  started = left
  shellsChanged(project, live)
}

// shellsChanged takes the project's shells as Go announces them, and takes the panes of the gone ones out.
export function shellsChanged(key: string, shells: Shell[]) {
  project = key
  live = shells
  started = stillStarted(started, key, new Set(shells.map((s) => s.id)), Date.now())
  const keep = new Set([...shells.map((s) => s.id), ...started.keys()])
  let next: Saved<ThreadTerms> = {}
  for (const [thread, saved] of Object.entries(terms)) {
    const pruned = prune(saved, key, keep)
    if (pruned.groups.length > 0) next[thread] = JSON.stringify(pruned) === JSON.stringify(saved) ? saved : pruned
  }
  if (JSON.stringify(next) !== JSON.stringify(terms)) {
    terms = next
    writeStored(TERMINALS_KEY, terms)
  }
  changed()
}

// forgetTerminals drops what the drawer keeps of a thread that is gone.
export function forgetTerminals(thread: string) {
  if (!(thread in opened || thread in heights || thread in terms)) return
  opened = put(opened, OPEN_KEY, thread, undefined)
  heights = put(heights, HEIGHT_KEY, thread, undefined)
  terms = put(terms, TERMINALS_KEY, thread, undefined)
  changed()
}

const MAX_WINDOW_SHARE = 0.75

// The tallest the drawer gets: three quarters of the window, and never over the composer above it, with its padding.
function maxHeight(drawer: HTMLElement): number {
  const column = drawer.parentElement!
  const box = column.querySelector('.composer-box')
  const composer = (box?.closest('.chat-over') ?? box)?.getBoundingClientRect().height ?? 0
  return Math.max(MIN_HEIGHT_PX, Math.min(MAX_WINDOW_SHARE * window.innerHeight, column.clientHeight - composer))
}

// useDrawerCap keeps the drawer under its tallest as the column's size changes, as when the window resizes.
export function useDrawerCap(drawer: RefObject<HTMLDivElement>, shown: boolean) {
  useEffect(() => {
    const el = drawer.current
    if (!shown || !el) return
    const observer = new ResizeObserver(() => el.style.setProperty('--terminal-max', `${maxHeight(el)}px`))
    observer.observe(el.parentElement!)
    return () => observer.disconnect()
  }, [drawer, shown])
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
