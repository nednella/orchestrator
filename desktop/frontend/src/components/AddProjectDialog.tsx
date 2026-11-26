import { ArrowLeft, ChevronRight, CornerLeftUp, Folder, FolderGit2, FolderPlus } from 'lucide-react'
import { useEffect, useId, useRef, useState } from 'react'
import type { KeyboardEvent, ReactNode } from 'react'
import { api, errorMessage } from '../api'
import { useOrchestrator } from '../OrchestratorContext'
import { keepFocusIn } from '../focusTrap'
import { completeFolder, enter, isBrowsePath, needsProject, planBrowse, projectPath, repositoryOf } from '../projectPath'
import type { NewProjectOptions } from '../types'
import { useFolderListing } from '../useFolderListing'
import { AddProjectCheck } from './AddProjectCheck'
import { AddProjectFooter } from './AddProjectFooter'
import { AddProjectRow } from './AddProjectRow'
import { AddProjectSubmit } from './AddProjectSubmit'
import { Icon } from './Icon'

type View = 'sources' | 'create' | 'browse'

type Row = { id: string; icon: ReactNode; title: string; description?: string; trailing?: ReactNode; checked?: boolean; run(): void }
type Group = { label: string; rows: Row[] }

const PLACEHOLDER: Record<View, string> = { sources: 'Search...', create: 'Project name', browse: 'Enter path (e.g. ~/projects/my-app)' }
const SOURCE_TERMS: Record<string, string[]> = { new: ['new project', 'create', 'empty', 'repository', 'git init'], local: ['local folder', 'folder', 'directory', 'browse'] }
const ICON = 16

export function AddProjectDialog() {
  const { overlay } = useOrchestrator('overlay')
  if (overlay !== 'add-project') return null
  return <AddProjectPopup />
}

function AddProjectPopup() {
  const { project, setOverlay, addProject, createProject, pushToast, focus } = useOrchestrator('project', 'setOverlay', 'addProject', 'createProject', 'pushToast', 'focus')
  const [stack, setStack] = useState<View[]>(['sources'])
  const [query, setQuery] = useState('')
  const [highlighted, setHighlighted] = useState<string | null>(null)
  const [options, setOptions] = useState<NewProjectOptions | null>(null)
  const [owner, setOwner] = useState<string | null>(null)
  const [publish, setPublish] = useState(false)
  const [working, setWorking] = useState(false)
  const input = useRef<HTMLInputElement>(null)
  const dialog = useRef<HTMLDivElement>(null)
  const busy = useRef(false)
  const closing = useRef(false)
  const previous = useRef(document.activeElement)
  const listId = useId()

  const view = stack[stack.length - 1]
  const browsing = view !== 'create' && isBrowsePath(query)
  const noProject = browsing && needsProject(query) && !project
  const name = query.trim()
  const folders = useFolderListing(browsing && !noProject ? planBrowse(query, null).directory : null)
  const plan = planBrowse(query, folders.listing)

  useEffect(() => {
    api.newProjectOptions().then(setOptions, (err) => pushToast({ tone: 'error', text: errorMessage(err) }))
  }, [pushToast])

  useEffect(() => {
    if (view !== 'create' || owner !== null) return
    api.githubOwner().then(setOwner, () => setOwner(''))
  }, [view, owner])

  useEffect(
    () => () => {
      // Strict Mode's rehearsal unmount leaves the dialog in the page.
      if (dialog.current?.isConnected || closing.current) return
      const el = previous.current
      if (el instanceof HTMLElement && el.isConnected && el !== document.body) el.focus()
      else focus('thread')
    },
    [focus],
  )

  const close = () => setOverlay(null)
  const closeDone = () => {
    closing.current = true
    close()
  }

  const push = (next: View, text: string) => {
    setStack((s) => [...s, next])
    setQuery(text)
    setHighlighted(null)
    setPublish(false)
    input.current?.focus()
  }
  const pop = () => {
    if (stack.length <= 1) return close()
    setStack((s) => s.slice(0, -1))
    setQuery('')
    setHighlighted(null)
    input.current?.focus()
  }
  const change = (text: string) => {
    setQuery(text)
    setHighlighted(null)
    if (text === '' && view === 'browse') pop()
  }

  const browseTo = async (path: string) => {
    await folders.prefetch(path)
    setQuery(path)
    setHighlighted(null)
  }

  const toGroups = (): Group[] => {
    if (view === 'create') {
      const repository = owner ? (name === '' ? owner : repositoryOf(owner, projectPath(options?.root ?? '', name))) : ''
      return [
        ...(owner
          ? [
              {
                label: 'Options',
                rows: [
                  {
                    id: 'create:publish',
                    icon: <Icon name="mark-github" size={ICON} />,
                    title: 'Create private repository on GitHub',
                    description: repository,
                    trailing: <AddProjectCheck checked={publish} />,
                    checked: publish,
                    run: () => !working && setPublish((on) => !on),
                  },
                ],
              },
            ]
          : []),
        {
          label: '',
          rows: [
            {
              id: 'create:existing',
              icon: <FolderPlus size={ICON} aria-hidden="true" />,
              title: 'Add existing project',
              description: 'Open a folder on disk',
              trailing: <ChevronRight size={ICON} className="-me-0.5 ms-auto flex-none text-soft/70" aria-hidden="true" />,
              run: pop,
            },
          ],
        },
      ]
    }
    if (browsing) {
      if (noProject) return []
      const rows: Row[] = []
      if (plan.parent !== null) rows.push({ id: 'browse:up', icon: <CornerLeftUp size={ICON} aria-hidden="true" />, title: '..', run: () => browseTo(plan.parent!) })
      for (const entry of plan.visible) rows.push({ id: `browse:${entry.path}`, icon: <Folder size={ICON} aria-hidden="true" />, title: entry.name, run: () => browseTo(enter(plan.directory, entry.name)) })
      return [{ label: 'Directories', rows }]
    }
    const needle = query.trim().toLowerCase()
    const listed = (id: string, title: string) => needle === '' || [title, ...SOURCE_TERMS[id]].some((t) => t.toLowerCase().includes(needle))
    const rows: Row[] = []
    if (options && listed('new', 'New project'))
      rows.push({ id: 'source:new', icon: <FolderGit2 size={ICON} aria-hidden="true" />, title: 'New project', description: 'Start a new Git repository from a name', run: () => push('create', '') })
    if (listed('local', 'Local folder'))
      rows.push({
        id: 'source:local',
        icon: <FolderPlus size={ICON} aria-hidden="true" />,
        title: 'Local folder',
        description: 'Browse a folder on disk',
        run: async () => {
          const start = options?.startFolder ?? '~/'
          await folders.prefetch(start)
          push('browse', start)
        },
      })
    return [{ label: 'Sources', rows }]
  }
  const groups = toGroups()
  const rows = groups.flatMap((g) => g.rows)
  const autoHighlights = view === 'sources' && !browsing
  const active = highlighted !== null && rows.some((r) => r.id === highlighted) ? highlighted : autoHighlights ? (rows[0]?.id ?? null) : null
  const activeBrowse = active?.startsWith('browse:') ?? false

  useEffect(() => {
    if (active) dialog.current?.querySelector(`[data-row="${CSS.escape(active)}"]`)?.scrollIntoView({ block: 'nearest' })
  }, [active])

  const canCreate = view === 'create' && name !== '' && !working
  const canAdd = browsing && !noProject && !working
  const creates = canAdd && !folders.pending && !folders.error && name !== '' && !activeBrowse && plan.creates

  const run = async (task: () => Promise<void>, failure: string) => {
    if (busy.current) return
    busy.current = true
    setWorking(true)
    try {
      await task()
    } catch (err) {
      pushToast({ tone: 'error', text: failure, detail: errorMessage(err) })
    } finally {
      busy.current = false
      setWorking(false)
    }
  }

  const submitCreate = () => {
    if (!canCreate) return
    void run(async () => {
      if (await createProject(name, publish && !!owner)) closeDone()
    }, 'Failed to open project')
  }

  const addFolder = (path: string, create: boolean) =>
    run(async () => {
      await addProject(path, create)
      closeDone()
      focus('thread')
    }, 'Failed to add project')

  const submitAdd = () => {
    if (canAdd) void addFolder(plan.path, creates)
  }

  const openInFinder = () =>
    run(async () => {
      const dir = await api.pickFolder()
      if (!dir) return
      await addProject(dir)
      closeDone()
      focus('thread')
    }, 'Failed to add project')

  const step = (delta: 1 | -1) => {
    const ids: (string | null)[] = autoHighlights ? rows.map((r) => r.id) : [null, ...rows.map((r) => r.id)]
    if (ids.length === 0) return
    const at = Math.max(ids.indexOf(active), 0)
    setHighlighted(ids[(at + delta + ids.length) % ids.length])
  }

  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.nativeEvent.isComposing || e.keyCode === 229) return
    const withCommand = e.metaKey && !e.ctrlKey
    if (e.key === 'Enter') {
      if (view === 'create' && (active === null || withCommand)) {
        e.preventDefault()
        submitCreate()
        return
      }
      if (browsing && canAdd && (!activeBrowse || withCommand)) {
        e.preventDefault()
        submitAdd()
        return
      }
      const row = rows.find((r) => r.id === active)
      if (row) {
        e.preventDefault()
        void row.run()
      }
      return
    }
    if (e.key === 'Tab' && !browsing) {
      keepFocusIn(dialog.current, e.nativeEvent)
      return
    }
    if (e.ctrlKey || e.shiftKey || e.altKey || e.metaKey) return
    if (e.key === 'Backspace' && query === '' && !e.repeat) {
      e.preventDefault()
      pop()
    } else if (e.key === 'Tab' && !noProject) {
      e.preventDefault()
      if (active === 'browse:up' && plan.parent !== null) return void browseTo(plan.parent)
      const entry = plan.visible.find((f) => `browse:${f.path}` === active)
      const completion = entry ? { name: entry.name, whole: true } : completeFolder(plan.visible, plan.tail)
      if (completion?.whole) void browseTo(enter(plan.directory, completion.name))
      else if (completion) change(`${plan.directory}${completion.name}`)
    } else if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault()
      step(e.key === 'ArrowDown' ? 1 : -1)
    }
  }

  const keys = (highlight: boolean) => (highlight ? '⌘ Enter' : 'Enter')
  const submit =
    view === 'create' ? (
      <AddProjectSubmit label={working ? 'Creating' : 'Create'} keys={keys(active !== null)} disabled={!canCreate} onSubmit={submitCreate} />
    ) : browsing ? (
      <AddProjectSubmit label={creates ? 'Create & Add' : 'Add'} keys={keys(activeBrowse)} disabled={!canAdd} onSubmit={submitAdd} />
    ) : null
  const endPadding = view === 'create' ? (active !== null ? 'pr-38' : 'pr-32') : browsing ? (creates ? 'pr-38' : activeBrowse ? 'pr-30' : 'pr-24') : ''
  const footerAction = view === 'create' ? (active === null ? 'Create' : active === 'create:publish' ? 'Toggle' : 'Select') : browsing ? (!canAdd || activeBrowse ? 'Select' : undefined) : 'Select'
  const root = options?.root ?? ''
  const emptyMessage = noProject ? 'Relative paths require an active project.' : creates ? 'Press Enter to create this folder and add it as a project.' : browsing ? 'No matching folders.' : 'No matching actions.'

  return (
    <div
      className="fade-in fixed inset-0 flex flex-col items-center px-4 pt-[max(10vh,3rem)] pb-[10vh]"
      style={{ zIndex: 'var(--z-overlay)', background: 'var(--bg-dialog-backdrop)', backdropFilter: 'blur(4px)' }}
      onMouseDown={(e) => e.target === e.currentTarget && close()}
    >
      <div
        ref={dialog}
        role="dialog"
        aria-modal="true"
        aria-label="Add project"
        className="flex max-h-[26.25rem] min-h-0 w-full max-w-xl flex-col overflow-hidden rounded-2xl border border-(--border-dialog) bg-(--bg-dialog) shadow-(--shadow-dialog)"
        style={{ backdropFilter: 'var(--glass-filter)' }}
      >
        <div className="relative px-2 py-1.5">
          <button type="button" className="absolute top-1/2 left-4 flex -translate-y-1/2 cursor-pointer items-center text-soft hover:text-ink" aria-label="Back" tabIndex={-1} onMouseDown={(e) => e.preventDefault()} onClick={pop}>
            <ArrowLeft size={ICON} aria-hidden="true" />
          </button>
          <input
            ref={input}
            autoFocus
            role="combobox"
            aria-expanded="true"
            aria-controls={listId}
            aria-activedescendant={active ?? undefined}
            aria-label={PLACEHOLDER[view]}
            className={`h-10 w-full bg-transparent pl-10 text-title text-ink outline-none placeholder:text-dim ${endPadding}`}
            placeholder={PLACEHOLDER[view]}
            spellCheck={false}
            autoComplete="off"
            autoCapitalize="off"
            maxLength={view === 'create' ? (options?.nameMax ?? 200) : undefined}
            value={query}
            onChange={(e) => change(e.target.value)}
            onKeyDown={onKeyDown}
          />
          {submit}
        </div>
        {view === 'create' && (
          <div className="p-2 pb-0">
            <AddProjectRow icon={<FolderGit2 size={ICON} aria-hidden="true" />} title={name === '' ? 'New project' : name} description={name === '' ? `Goes in ${root}` : `Creates ${projectPath(root, name)}`} />
          </div>
        )}
        {browsing && !noProject && folders.error && (
          <div role="alert" className="px-4 py-3 text-body text-soft">
            {folders.error}
          </div>
        )}
        {browsing && !noProject && folders.pending && (
          <div role="status" className="px-4 py-3 text-body text-soft">
            Loading folders...
          </div>
        )}
        <div id={listId} role="listbox" className="min-h-0 flex-1 overflow-y-auto p-2">
          {rows.length === 0 ? (
            !folders.pending && !folders.error && <div className="py-10 text-center text-body text-soft">{emptyMessage}</div>
          ) : (
            groups.map((group, i) => (
              <div key={group.label || i} className={i > 0 ? 'pt-1.5' : undefined}>
                {group.label && <div className="px-2 py-1.5 text-small font-medium text-soft">{group.label}</div>}
                {group.rows.map((row) => (
                  <AddProjectRow key={row.id} id={row.id} icon={row.icon} title={row.title} description={row.description} trailing={row.trailing} checked={row.checked} active={row.id === active} onHover={() => row.id !== active && setHighlighted(row.id)} onRun={() => void row.run()} />
                ))}
              </div>
            ))
          )}
        </div>
        <AddProjectFooter
          action={footerAction}
          trailing={
            browsing && (
              <button type="button" className="btn btn-ghost h-auto px-2 py-1 text-small hover:bg-transparent hover:text-ink" disabled={working} onMouseDown={(e) => e.preventDefault()} onClick={() => void openInFinder()}>
                Open in Finder
              </button>
            )
          }
        />
      </div>
    </div>
  )
}
