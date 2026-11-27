import type { FileTree as FileTreeModel } from '@pierre/trees'
import { FileTree, useFileTree, useFileTreeSelector } from '@pierre/trees/react'
import { ChevronsDownUp, ChevronsUpDown, RefreshCw, Search } from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { KeyboardEvent, RefObject } from 'react'
import { useOrchestrator } from '../OrchestratorContext'
import { editorLabel } from '../contextCard'
import { ancestorsOf, folderHandle, folderRow, onlyChildFolders, treePath } from '../fileTree'
import { FILE_ICONS, LOADER_ICON } from '../fileIcons'
import { readStored, writeStored } from '../storage'
import type { FileListingActions, FileListingState } from '../useFileListing'
import type { FileEntry } from '../types'
import { ContextMenu } from './ContextMenu'

const OPEN_KEY = (project: string) => `orchestrator.filesOpen.${project}`
const MAX_OPEN = 500

// What the tree is told to look like, in the CSS of its own shadow root, which the page's CSS does not reach.
const TREE_CSS = `
  [data-type='item'] { cursor: pointer; }
  [data-item-loading] [data-icon-name] { animation: spin 1s linear infinite; opacity: 0.6; }
  @keyframes spin { to { transform: rotate(360deg); } }
  @media (prefers-reduced-motion: reduce) { [data-item-loading] [data-icon-name] { animation: none; } }
`

// showPath opens the folders on the way to a tree path and the folder it names, makes its row the one selected and
// scrolls it into view.
function showPath(model: FileTreeModel, path: string, focus: boolean) {
  for (const folder of ancestorsOf(path)) folderHandle(model, folder)?.expand()
  folderHandle(model, path)?.expand()
  const item = model.getItem(path)
  if (item && !item.isSelected()) {
    for (const selected of model.getSelectedPaths()) model.getItem(selected)?.deselect()
    item.select()
  }
  model.scrollToPath(path, { focus, offset: 'center' })
}

const matchCount = (m: { getSearchMatchingPaths(): readonly string[] }) => m.getSearchMatchingPaths().length

type FileTreePaneProps = {
  project: string
  // folder is the full path of the folder the tree lists, for the path a row copies.
  folder: string
  entries: FileEntry[]
  listing: FileListingState
  actions: FileListingActions
  openPath: string | null
  // reveal asks the tree to show a path, and to say so again each time n changes.
  reveal: { path: string; n: number } | null
  onOpen(path: string): void
  onOpenInEditor(path: string): void
  searchRef: RefObject<HTMLInputElement>
}

// FileTreePane is the tree of the folder's files, with a search field and buttons that read the files again and open or
// close every folder.
export function FileTreePane({ project, folder, entries, listing, actions, openPath, reveal, onOpen, onOpenInEditor, searchRef }: FileTreePaneProps) {
  const { report, settings } = useOrchestrator('report', 'settings')
  const [query, setQuery] = useState('')
  const modifier = useRef(false)
  const latest = useRef<{ openPath: string | null; onOpen(path: string): void; loading: ReadonlySet<string>; folders: Map<string, FileEntry>; onlyChild: Map<string, string>; actions: FileListingActions }>({ openPath, onOpen, loading: new Set<string>(), folders: new Map<string, FileEntry>(), onlyChild: new Map<string, string>(), actions })
  const loading = listing.status === 'ready' ? listing.loading : null
  latest.current = { ...latest.current, openPath, onOpen, loading: loading ?? latest.current.loading, actions }

  const [openAtStart] = useState(() => readStored<string[]>(OPEN_KEY(project), []))
  const { model } = useFileTree({
    paths: [],
    initialExpandedPaths: openAtStart,
    flattenEmptyDirectories: true,
    initialExpansion: 'closed',
    density: 'compact',
    icons: FILE_ICONS,
    fileTreeSearchMode: 'hide-non-matches',
    composition: { contextMenu: { enabled: true, triggerMode: 'right-click' } },
    unsafeCSS: TREE_CSS,
    renderRowDecoration: ({ item }) => (latest.current.loading.has(item.path.replace(/\/$/, '')) ? { icon: { name: LOADER_ICON }, title: 'Loading…' } : null),
    onSelectionChange: (selected) => {
      const path = selected.length === 1 ? selected[0] : undefined
      if (path === undefined || path.endsWith('/') || modifier.current || path === latest.current.openPath) return
      latest.current.onOpen(path)
    },
  })

  const folders = useMemo(() => entries.filter((e) => e.dir), [entries])
  const folderByPath = useMemo(() => new Map(folders.map((f) => [treePath(f), f])), [folders])
  latest.current.folders = folderByPath
  latest.current.onlyChild = useMemo(() => onlyChildFolders(entries), [entries])
  const searchable = listing.status === 'ready'

  // The tree lists what the app listed, and keeps the folders that were open.
  useEffect(() => {
    const open = new Set<string>(readStored<string[]>(OPEN_KEY(project), []))
    for (const path of folderByPath.keys()) {
      const folder = folderHandle(model, path)
      if (folder?.isExpanded()) open.add(path)
      else if (folder) open.delete(path)
    }
    model.resetPaths(
      entries.map(treePath),
      { initialExpandedPaths: [...open].filter((p) => folderByPath.has(p)) },
    )
    model.setGitStatus(entries.filter((e) => e.ignored).map((e) => ({ path: treePath(e), status: 'ignored' as const })))
  }, [model, entries, project, folderByPath])

  // A folder git ignores holds nothing until it is opened; opening it lists it. The tree redraws its rows when asked
  // for its labels, which is how a row learns that its folder is loading.
  useEffect(() => {
    model.setLabels({})
  }, [model, loading])

  useEffect(
    () =>
      model.subscribe(() => {
        for (const [path, entry] of latest.current.folders) {
          if (!entry.ignored) continue
          const folder = folderHandle(model, path)
          if (!folder?.isExpanded()) continue
          latest.current.actions.loadFolder(entry.path)
          // A folder that holds only a folder opens it too, as the tree's rows of such a chain are one.
          const inner = latest.current.onlyChild.get(entry.path)
          if (inner !== undefined) folderHandle(model, `${inner}/`)?.expand()
        }
      }),
    [model],
  )

  useEffect(() => {
    let timer: ReturnType<typeof setTimeout> | undefined
    const save = () => {
      clearTimeout(timer)
      timer = setTimeout(() => {
        // Before the files are listed no folder is known, and none open would be saved.
        if (latest.current.folders.size === 0) return
        const open: string[] = []
        for (const path of latest.current.folders.keys()) {
          if (folderHandle(model, path)?.isExpanded()) open.push(path)
        }
        writeStored(OPEN_KEY(project), open.slice(0, MAX_OPEN))
      }, 400)
    }
    const stop = model.subscribe(save)
    return () => {
      stop()
      clearTimeout(timer)
    }
  }, [model, project])

  // A closed file lets go of its row, so that a click on the row opens it again.
  useEffect(() => {
    if (openPath !== null) return
    for (const selected of model.getSelectedPaths()) model.getItem(selected)?.deselect()
  }, [model, openPath])

  // The open file is the tree's selected row, its folders open and in view.
  useEffect(() => {
    if (openPath === null || entries.length === 0) return
    showPath(model, openPath, false)
  }, [model, openPath, entries])

  useEffect(() => {
    if (reveal === null) return
    if (reveal.path === '') {
      model.focusFirstItem()
      return
    }
    const row = `${folderRow(latest.current.onlyChild, reveal.path)}/`
    showPath(model, folderHandle(model, row) ? row : reveal.path, true)
  }, [model, reveal])

  const everyFolderOpen = useCallback(
    (m: typeof model) => {
      const shown = folders.filter((f) => !f.ignored)
      return shown.length > 0 && shown.every((f) => folderHandle(m, treePath(f))?.isExpanded() === true)
    },
    [folders],
  )
  const allOpen = useFileTreeSelector(model, everyFolderOpen)
  const toggleAll = () => {
    for (const f of folders) {
      const item = folderHandle(model, treePath(f))
      if (allOpen) item?.collapse()
      else if (!f.ignored) item?.expand()
    }
  }

  const search = (value: string) => {
    setQuery(value)
    model.setSearch(value === '' ? null : value)
  }
  const matches = useFileTreeSelector(model, matchCount)
  const onSearchKey = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.nativeEvent.isComposing) return
    if (e.key === 'Escape') {
      e.preventDefault()
      e.stopPropagation()
      if (query !== '') search('')
      else e.currentTarget.blur()
    } else if (e.key === 'ArrowDown' || e.key === 'Enter') {
      e.preventDefault()
      model.focusNextSearchMatch()
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      model.focusPreviousSearchMatch()
    }
  }

  const copy = (text: string) => report(() => navigator.clipboard.writeText(text))
  const ready = listing.status === 'ready'
  const noMatch = ready && query !== '' && matches === 0

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <div className="flex h-10 flex-none items-center gap-1 border-b border-(--border-rule) px-2">
        <button className="btn btn-ghost h-7 w-7 flex-none justify-center px-0" title="Refresh files" aria-label="Refresh files" onClick={actions.refresh}>
          <RefreshCw size={14} className={ready && listing.refreshing ? 'animate-spin' : ''} aria-hidden="true" />
        </button>
        <label className="flex h-7 min-w-0 flex-1 items-center gap-1.5 rounded-md border border-line-strong px-2 text-small focus-within:border-accent">
          <Search size={14} className="flex-none text-dim" aria-hidden="true" />
          <input
            ref={searchRef}
            type="text"
            role="searchbox"
            spellCheck={false}
            autoComplete="off"
            maxLength={256}
            placeholder="Search files"
            value={query}
            disabled={!searchable}
            className="min-w-0 flex-1 bg-transparent outline-none placeholder:text-dim"
            onChange={(e) => search(e.target.value)}
            onKeyDown={onSearchKey}
          />
        </label>
        {folders.length > 0 && (
          <button className="btn btn-ghost h-7 w-7 flex-none justify-center px-0" title={allOpen ? 'Collapse all' : 'Expand all'} aria-label={allOpen ? 'Collapse all' : 'Expand all'} onClick={toggleAll}>
            {allOpen ? <ChevronsDownUp size={14} aria-hidden="true" /> : <ChevronsUpDown size={14} aria-hidden="true" />}
          </button>
        )}
      </div>
      {listing.status === 'loading' && <p className="px-3 py-4 text-small text-dim">Loading files…</p>}
      {listing.status === 'error' && (
        <button className="p-4 text-left text-small leading-relaxed text-danger" onClick={actions.refresh}>
          {listing.error} Click to retry.
        </button>
      )}
      {ready && entries.length === 0 && <p className="px-3 py-4 text-small text-dim">This folder has no files.</p>}
      {noMatch && <p className="px-3 py-4 text-small text-dim">No matching files.</p>}
      {ready && listing.truncated && <p className="px-3 py-1 text-small text-dim">Showing the first files only.</p>}
      <div className="file-tree min-h-0 flex-1" hidden={!ready || entries.length === 0 || noMatch} onPointerDownCapture={(e) => (modifier.current = e.metaKey || e.ctrlKey || e.shiftKey || e.altKey)} onKeyDownCapture={() => (modifier.current = false)}>
        <FileTree
          model={model}
          style={{ height: '100%' }}
          renderContextMenu={(item, ctx) => (
            <ContextMenu
              label={item.name}
              at={{ x: ctx.anchorRect.left, y: ctx.anchorRect.bottom }}
              items={[
                item.kind === 'file' && { label: editorLabel(settings.editorApp), run: () => onOpenInEditor(item.path) },
                { label: 'Copy path', run: () => copy(`${folder}/${item.path.replace(/\/$/, '')}`) },
                { label: 'Copy relative path', run: () => copy(item.path.replace(/\/$/, '')) },
              ]}
              onClose={() => ctx.close()}
            />
          )}
        />
      </div>
    </div>
  )
}
