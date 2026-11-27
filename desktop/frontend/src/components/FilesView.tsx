import { useMemo, useRef, useState, useSyncExternalStore } from 'react'
import type { KeyboardEvent } from 'react'
import { useOrchestrator, useOrchestratorSelector } from '../OrchestratorContext'
import { api } from '../api'
import { isMarkdownPath, relativeTo, withFolders } from '../fileTree'
import { FILES_RENDERED_KEY, FILES_TREE_KEY, FILES_TREE_WIDTH_KEY, FILES_WRAP_KEY, filesKey, onPicked, pickedFile, pickFile } from '../filesView'
import type { PanelViewProps } from '../panelViews'
import { readStored, writeStored } from '../storage'
import { useElementWidth } from '../useElementWidth'
import { useFileListing } from '../useFileListing'
import { useOpenFile } from '../useOpenFile'
import { CenteredSpinner } from './CenteredSpinner'
import { FileBody } from './FileBody'
import { FileCrumbs } from './FileCrumbs'
import { FileHeader } from './FileHeader'
import { FileTreeDivider } from './FileTreeDivider'
import { FileTreePane } from './FileTreePane'

const TREE_WIDTH = { default: 256, min: 160, max: 560 }
// The open file keeps at least this much of the panel's width.
const FILE_MIN_WIDTH = 256

const reason = (err: unknown) => (err instanceof Error ? err.message : String(err))

// FilesView is the files of the thread's folder, its worktree else the project's, or of the project's on Home: the open
// file on the left and the tree of the folder on the right, which fills the view while no file is open.
export function FilesView({ thread }: PanelViewProps) {
  const { settings, report } = useOrchestrator('settings', 'report')
  const project = useOrchestratorSelector((a) => a.project)
  const threadId = thread?.id
  const projectKey = project?.key ?? ''
  const key = filesKey(threadId, projectKey)
  const folder = thread?.worktree || project?.dir || ''
  const picked = useSyncExternalStore(onPicked, () => pickedFile(key))
  const path = picked === null ? null : relativeTo(folder, picked)
  const [listing, actions] = useFileListing(threadId, projectKey, thread?.state)
  const entries = useMemo(() => (listing.status === 'ready' ? withFolders(listing.entries) : []), [listing])
  const file = useOpenFile(key, threadId, projectKey, path, thread?.state)
  const [treeOpen, setTreeOpen] = useState(() => readStored(FILES_TREE_KEY, true))
  const [treeWidth, setTreeWidth] = useState(() => readStored(FILES_TREE_WIDTH_KEY, TREE_WIDTH.default))
  const [rendered, setRendered] = useState(() => readStored(FILES_RENDERED_KEY, false))
  const [wrap, setWrap] = useState(() => readStored(FILES_WRAP_KEY, true))
  const [reveal, setReveal] = useState<{ path: string; n: number } | null>(null)
  const box = useRef<HTMLDivElement>(null)
  const search = useRef<HTMLInputElement>(null)
  const room = useElementWidth(box)

  const remember = <T,>(set: (v: T) => void, name: string) => (value: T) => {
    set(value)
    writeStored(name, value)
  }
  const open = (to: string) => pickFile(key, to)
  const showTree = path === null || treeOpen
  const maxTree = Math.max(TREE_WIDTH.min, Math.min(TREE_WIDTH.max, room - FILE_MIN_WIDTH))
  const shownTree = Math.min(maxTree, Math.max(TREE_WIDTH.min, treeWidth))

  const content = file?.status === 'ready' ? file.content : null
  const text = content?.kind === 'text' ? content : null
  const canRender = text !== null && text.text !== '' && !text.truncated && isMarkdownPath(path ?? '')
  const showsLines = text !== null && text.text !== '' && !(canRender && rendered)
  const openInEditor = (target: string) => report(() => (threadId ? api.openInEditor(threadId, target) : api.openProjectInEditor(projectKey, target)))

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (!(e.metaKey || e.ctrlKey) || e.key.toLowerCase() !== 'f' || e.shiftKey || e.altKey) return
    e.preventDefault()
    if (path !== null && !treeOpen) {
      setTreeOpen(true)
      writeStored(FILES_TREE_KEY, true)
    }
    requestAnimationFrame(() => {
      search.current?.focus()
      search.current?.select()
    })
  }

  return (
    <div ref={box} className="absolute inset-0 flex" onKeyDown={onKeyDown}>
      {path !== null && (
        <section className="flex min-w-0 flex-1 flex-col" aria-label="File">
          <FileHeader
            crumbs={
              <FileCrumbs
                project={project?.name ?? ''}
                path={path}
                entries={entries}
                onOpen={open}
                onReveal={(to) => {
                  setTreeOpen(true)
                  writeStored(FILES_TREE_KEY, true)
                  setReveal((r) => ({ path: to, n: (r?.n ?? 0) + 1 }))
                }}
                onRefresh={actions.refresh}
              />
            }
            editor={settings.editorApp}
            rendered={canRender ? rendered : null}
            wrap={showsLines ? wrap : null}
            copyable={text !== null && !text.truncated && text.text !== '' ? text.text : null}
            treeOpen={treeOpen}
            onOpenInEditor={() => openInEditor(path)}
            onRendered={remember(setRendered, FILES_RENDERED_KEY)}
            onWrap={remember(setWrap, FILES_WRAP_KEY)}
            onTree={remember(setTreeOpen, FILES_TREE_KEY)}
          />
          {file?.status === 'loading' && <CenteredSpinner />}
          {file?.status === 'error' && (
            <div className="flex flex-1 flex-col items-center justify-center gap-3 px-4 text-center">
              <p className="text-small text-danger">{reason(file.error)}</p>
              <button className="btn" onClick={file.retry}>
                Try again
              </button>
            </div>
          )}
          {content && <FileBody linksFrom={{ key, path }} content={content} rendered={rendered} wrap={wrap} />}
        </section>
      )}
      <aside className={`flex min-w-0 ${path === null ? 'flex-1' : 'flex-none'}`} style={path === null ? undefined : { width: shownTree }} hidden={!showTree} aria-label="File explorer">
        {path !== null && <FileTreeDivider width={shownTree} min={TREE_WIDTH.min} max={maxTree} onWidth={(w, commit) => (commit ? remember(setTreeWidth, FILES_TREE_WIDTH_KEY)(w) : setTreeWidth(w))} />}
        <FileTreePane
          project={projectKey}
          folder={folder}
          entries={entries}
          listing={listing}
          actions={actions}
          openPath={path}
          reveal={reveal}
          onOpen={open}
          onOpenInEditor={openInEditor}
          searchRef={search}
        />
      </aside>
    </div>
  )
}
