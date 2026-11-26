import type { Shell } from './types'

// shellsOf are the ids of the thread's shells in order, and with thread '' the project's.
export function shellsOf(shells: Shell[], thread: string): string[] {
  return shells.filter((s) => s.thread === thread).map((s) => s.id)
}
