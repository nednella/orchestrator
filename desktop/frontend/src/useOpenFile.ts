import { useEffect, useState } from 'react'
import { api, errorMessage } from './api'
import type { FileContent } from './types'

type Read = { key: string; path: string; content: FileContent } | { key: string; path: string; error: string }

export type OpenFile = { status: 'loading' } | { status: 'error'; error: string; retry(): void } | { status: 'ready'; content: FileContent }

// useOpenFile reads the file at path in the thread's folder, or in the project's with no thread; with outside, the path is absolute
// and in no folder of the thread, and is read, read only, as it is. It reads the file again
// when the thread's state moves, as at a turn's end, when its agent may have written it, or when retry is called.
export function useOpenFile(key: string, thread: string | undefined, project: string, path: string | null, state: string | undefined, outside = false): OpenFile | null {
  const [read, setRead] = useState<Read | null>(null)
  const [round, setRound] = useState(0)

  useEffect(() => {
    if (path === null) return
    let stale = false
    const file = thread ? (outside ? api.chatReadOutside(thread, path) : api.chatReadFile(thread, path)) : api.projectReadFile(project, path)
    file.then(
      (content) => !stale && setRead({ key, path, content }),
      (err) => !stale && setRead({ key, path, error: errorMessage(err) }),
    )
    return () => {
      stale = true
    }
  }, [key, thread, project, path, state, round, outside])

  if (path === null) return null
  if (read?.key !== key || read.path !== path) return { status: 'loading' }
  if ('error' in read) return { status: 'error', error: read.error, retry: () => setRound((n) => n + 1) }
  return { status: 'ready', content: read.content }
}
