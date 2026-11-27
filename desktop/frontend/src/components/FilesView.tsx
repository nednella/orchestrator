import { useEffect, useMemo, useRef, useState, useSyncExternalStore } from 'react'
import type { KeyboardEvent } from 'react'
import { useOrchestrator, useOrchestratorSelector } from '../OrchestratorContext'
import { api } from '../api'
import { isMarkdownPath, withFolders } from '../fileTree'
import { FILES_RENDERED_KEY, FILES_TREE_KEY, FILES_TREE_WIDTH_KEY, FILES_WRAP_KEY, filesKey, onPicked, pickedPath, pickFile, pickOf } from '../filesView'
import type { PanelViewProps } from '../panelViews'
import { readStored, writeStored } from '../storage'
import { useElementWidth } from '../useElementWidth'
import { useFileListing } from '../useFileListing'
import { useOpenFile } from '../useOpenFile'
import { CenteredSpinner } from './CenteredSpinner'
import { FileBody } from './FileBody'
import { FileCrumbs } from './FileCrumbs'
import { FileHeader } from './FileHeader'
import { FileOutsideCrumbs } from './FileOutsideCrumbs'
import { FileOutsideNote } from './FileOutsideNote'
import { FileTreeDivider } from './FileTreeDivider'
import { FileTreePane } from './FileTreePane'

const TREE_WIDTH = { default: 256, min: 160, max: 560 }
// The open file keeps at least this much of the panel's width.
const FILE_MIN_WIDTH = 256

// FilesView is the files of the thread's folder, its worktree else the project's, or of the project's on Home: the open
// file on the left and the tree of the folder on the right, which fills the view while no file is open. Closing the file,
// with its button or Esc in it, gives the tree the view again.
export function FilesView({ thread }: PanelViewProps) {
  const { settings, report } = useOrchestrator('settings', 'report')
  const project = useOrchestratorSelector((a) => a.project)
  const threadId = thread?.id
  const projectKey = project?.key ?? ''
  const key = filesKey(threadId, projectKey)
  const folder = thread?.worktree || project?.dir || ''
  const pick = useSyncExternalStore(onPicked, () => pickOf(key))
  const path = pick === null ? null : pickedPath(folder, pick)
  // A file outside the thread's folder is shown read only.
  const outside = path !== null && pick?.outside === true
  const place = pick?.place ?? null
  const [listing, actions] = useFileListing(threadId, projectKey, thread?.state)
  const entries = useMemo(() => (listing.status === 'ready' ? withFolders(listing.entries) : []), [listing])
  const file = useOpenFile(key, threadId, projectKey, path, thread?.state, outside)
  const [treeOpen, setTreeOpen] = useState(() => readStored(FILES_TREE_KEY, true))
  const [treeWidth, setTreeWidth] = useState(() => readStored(FILES_TREE_WIDTH_KEY, TREE_WIDTH.default))
  const [rendered, setRendered] = useState(() => readStored(FILES_RENDERED_KEY, false))
  const [wrap, setWrap] = useState(() => readStored(FILES_WRAP_KEY, true))
  const [reveal, setReveal] = useState<{ path: string; n: number } | null>(null)
  const folderPick = pick?.folder ?? null
  useEffect(() => {
    if (folderPick !== null) setReveal((r) => ({ path: folderPick, n: (r?.n ?? 0) + 1 }))
  }, [pick, folderPick])
  const box = useRef<HTMLDivElement>(null)
  const search = useRef<HTMLInputElement>(null)
  const room = useElementWidth(box)

  const remember = <T,>(set: (v: T) => void, name: string) => (value: T) => {
    set(value)
    writeStored(name, value)
  }
  const open = (to: string) => pickFile(key, to)
  const close = () => pickFile(key, null)
  const showTree = path === null || treeOpen
  const maxTree = Math.max(TREE_WIDTH.min, Math.min(TREE_WIDTH.max, room - FILE_MIN_WIDTH))
  const shownTree = Math.min(maxTree, Math.max(TREE_WIDTH.min, treeWidth))

  const content = file?.status === 'ready' ? file.content : null
  const text = content?.kind === 'text' ? content : null
  const canRender = text !== null && text.text !== '' && !text.truncated && isMarkdownPath(path ?? '')
  // A link to a line shows the source, which has lines.
  const showRendered = rendered && place === null
  const showsLines = text !== null && text.text !== '' && !(canRender && showRendered)
  const openInEditor = (target: string) =>
    report(() => {
      if (threadId && outside) return api.openFileInEditor(threadId, target, place?.line)
      return threadId ? api.openInEditor(threadId, target) : api.openProjectInEditor(projectKey, target)
    })

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

  // A menu or a search open in the view takes its Esc first.
  const onFileKeyDown = (e: KeyboardEvent<HTMLElement>) => {
    if (e.key !== 'Escape' || e.defaultPrevented || e.nativeEvent.isComposing) return
    e.preventDefault()
    e.stopPropagation()
    close()
  }

  return (
    <div ref={box} className="absolute inset-0 flex" onKeyDown={onKeyDown}>
      {path !== null && (
        <section tabIndex={-1} className="flex min-w-0 flex-1 flex-col outline-none" aria-label="File" onKeyDown={onFileKeyDown}>
          <FileHeader
            crumbs={
              outside ? (
                <FileOutsideCrumbs path={path} />
              ) : (
                <FileCrumbs
                  project={project?.name ?? ''}
                  path={path}
                  entries={entries}
                  onOpen={open}
                  onShowFolder={(folder) => {
                    close()
                    setReveal((r) => ({ path: folder, n: (r?.n ?? 0) + 1 }))
                  }}
                  onReveal={(to) => {
                    setTreeOpen(true)
                    writeStored(FILES_TREE_KEY, true)
                    setReveal((r) => ({ path: to, n: (r?.n ?? 0) + 1 }))
                  }}
                  onRefresh={actions.refresh}
                />
              )
            }
            editor={settings.editorApp}
            rendered={canRender ? showRendered : null}
            wrap={showsLines ? wrap : null}
            copyable={text !== null && !text.truncated && text.text !== '' ? text.text : null}
            treeOpen={treeOpen}
            onOpenInEditor={() => openInEditor(path)}
            onRendered={(on) => {
              if (on) pickFile(key, pick?.path ?? null)
              remember(setRendered, FILES_RENDERED_KEY)(on)
            }}
            onWrap={remember(setWrap, FILES_WRAP_KEY)}
            onTree={remember(setTreeOpen, FILES_TREE_KEY)}
            onClose={close}
          />
          {outside && <FileOutsideNote path={path} />}
          {file?.status === 'loading' && <CenteredSpinner />}
          {file?.status === 'error' && (
            <div className="flex flex-1 flex-col items-center justify-center gap-3 px-4 text-center">
              <p className="text-small text-danger">{file.error}</p>
              <button className="btn" onClick={file.retry}>
                Try again
              </button>
            </div>
          )}
          {content && <FileBody linksFrom={{ key, path }} content={content} rendered={showRendered} wrap={wrap} place={place} />}
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
          openPath={outside ? null : path}
          reveal={reveal}
          onOpen={open}
          onOpenInEditor={openInEditor}
          searchRef={search}
        />
      </aside>
    </div>
  )
}
