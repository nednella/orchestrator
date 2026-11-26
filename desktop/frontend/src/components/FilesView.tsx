import { Check, ChevronDown, CodeXml, Copy, Eye, FileText, WrapText } from 'lucide-react'
import { Fragment, useEffect, useMemo, useRef, useState, useSyncExternalStore } from 'react'
import { useAgentos, useAgentosSelector } from '../AgentosContext'
import { api } from '../api'
import { editorLabel } from '../contextCard'
import { FILES_MODE_KEY, fileLabel, filesKey, LATEST_PLAN, onPicked, pickedFile, pickerOrder, pickFile } from '../filesView'
import type { FilesMode } from '../filesView'
import type { PanelViewProps } from '../panelViews'
import { readStored, writeStored } from '../storage'
import { useCopied } from '../useCopied'
import type { Point } from '../useFloatingAt'
import { ChatMarkdown } from './ChatMarkdown'
import { EditorMark } from './EditorMark'
import { FilePicker } from './FilePicker'
import { Loading } from './Loading'
import { Notice } from './Notice'

const toolButton = 'btn btn-ghost h-7 w-7 flex-none justify-center px-0 aria-pressed:bg-(--bg-segment-on) aria-pressed:text-ink'

type Listing = { key: string; files: string[]; plan: string } | { key: string; error: string }
type Read = { key: string; name: string; text: string } | { key: string; name: string; error: string }

const reason = (err: unknown) => (err instanceof Error ? err.message : String(err))

// FilesView shows one markdown file of the thread's folder, or on Home of the project's, rendered or as its source, to
// read only. The picker lists the folder's markdown files, after the plan Claude last asked to have approved.
export function FilesView({ session }: PanelViewProps) {
  const { settings, report } = useAgentos('settings', 'report')
  const project = useAgentosSelector((a) => a.project?.key ?? '')
  const thread = session?.id
  const state = session?.state
  const key = filesKey(thread, project)
  const picked = useSyncExternalStore(onPicked, () => pickedFile(key))
  const [listing, setListing] = useState<Listing | null>(null)
  const [read, setRead] = useState<Read | null>(null)
  const [mode, setMode] = useState<FilesMode>(() => (readStored<string>(FILES_MODE_KEY, 'rendered') === 'source' ? 'source' : 'rendered'))
  const [wrap, setWrap] = useState(true)
  const [pickerAt, setPickerAt] = useState<Point | null>(null)
  const pickerButton = useRef<HTMLButtonElement>(null)
  // A press on the picker while it is open closes it on mousedown; the click that follows must not open it again.
  const closing = useRef(false)

  // Read again whenever the thread's state moves, as at a turn's end, when its agent may have written files.
  useEffect(() => {
    let stale = false
    const files = thread ? api.chatMarkdownFiles(thread) : api.projectMarkdownFiles(project)
    const plan = thread ? api.chatLatestPlan(thread) : Promise.resolve('')
    Promise.all([files, plan]).then(
      ([f, p]) => !stale && setListing({ key, files: pickerOrder(f), plan: p }),
      (err) => !stale && setListing({ key, error: reason(err) }),
    )
    return () => {
      stale = true
    }
  }, [thread, project, key, state])

  const listed = listing?.key === key && !('error' in listing) ? listing : null
  const name = picked === LATEST_PLAN && !listed?.plan ? null : (picked ?? (listed?.plan ? LATEST_PLAN : (listed?.files[0] ?? null)))

  useEffect(() => {
    if (name === null || name === LATEST_PLAN) return
    let stale = false
    const text = thread ? api.chatReadFile(thread, name) : api.projectReadFile(project, name)
    text.then(
      (t) => !stale && setRead({ key, name, text: t }),
      (err) => !stale && setRead({ key, name, error: reason(err) }),
    )
    return () => {
      stale = true
    }
  }, [thread, project, key, name, state])

  const shown = name === LATEST_PLAN ? { text: listed?.plan ?? '' } : read?.key === key && read.name === name ? read : null
  const text = shown && 'text' in shown ? shown.text : null
  const [copied, copy] = useCopied(text ?? '')
  const linksFrom = useMemo(() => ({ key, path: name === LATEST_PLAN ? '' : (name ?? '') }), [key, name])

  if (listing?.key === key && 'error' in listing) return <Notice title="Could not list the files" hint={listing.error} centered />
  if (!listed) return <Loading />
  if (name === null) return <Notice title="No markdown files" hint={`${thread ? "This thread's" : "This project's"} folder has no markdown files git knows of.`} centered />

  const choices = listed.plan ? [LATEST_PLAN, ...listed.files] : listed.files
  const choose = (choice: string) => {
    setPickerAt(null)
    pickFile(key, choice)
    pickerButton.current?.focus()
  }
  const setShownMode = (next: FilesMode) => {
    setMode(next)
    writeStored(FILES_MODE_KEY, next)
  }
  const openPicker = () => {
    if (closing.current) {
      closing.current = false
      return
    }
    const box = pickerButton.current?.getBoundingClientRect()
    if (box) setPickerAt({ x: box.left, y: box.bottom + 4 })
  }

  const { folders, name: fileName } = fileLabel(name)
  const editor = editorLabel(settings.editorApp)
  const rendered = mode === 'rendered'
  const lines = text?.split('\n') ?? []
  return (
    <div className="absolute inset-0 flex flex-col">
      <div className="flex flex-none items-center gap-1 border-b border-line px-2 py-1.5">
        <button
          ref={pickerButton}
          className="btn btn-ghost h-7 min-w-0 gap-1 px-2 text-small text-soft"
          aria-haspopup="menu"
          aria-expanded={pickerAt !== null}
          title={name === LATEST_PLAN ? 'Latest plan' : name}
          onMouseDown={() => {
            closing.current = pickerAt !== null
          }}
          onClick={openPicker}
        >
          <FileText size={14} className="flex-none" aria-hidden="true" />
          <span className="truncate">
            {folders && <span>{folders} / </span>}
            <span className="font-medium text-ink">{fileName}</span>
          </span>
          <ChevronDown size={12} className="flex-none" aria-hidden="true" />
        </button>
        <span className="flex-1" />
        <button className={toolButton} title={rendered ? 'Show markdown source' : 'Show rendered markdown'} aria-label={rendered ? 'Show markdown source' : 'Show rendered markdown'} onClick={() => setShownMode(rendered ? 'source' : 'rendered')}>
          {rendered ? <CodeXml size={14} aria-hidden="true" /> : <Eye size={14} aria-hidden="true" />}
        </button>
        {!rendered && (
          <button className={toolButton} title={wrap ? 'Disable word wrap' : 'Enable word wrap'} aria-label={wrap ? 'Disable word wrap' : 'Enable word wrap'} aria-pressed={wrap} onClick={() => setWrap(!wrap)}>
            <WrapText size={14} aria-hidden="true" />
          </button>
        )}
        <button className={toolButton} title={copied ? 'Copied' : 'Copy contents'} aria-label={copied ? 'Copied' : 'Copy contents'} disabled={text === null} onClick={copy}>
          {copied ? <Check size={14} aria-hidden="true" /> : <Copy size={14} aria-hidden="true" />}
        </button>
        {thread && name !== LATEST_PLAN && (
          <button className={toolButton} title={editor} aria-label={editor} onClick={() => report(() => api.openInEditor(thread, name))}>
            <EditorMark editor={settings.editorApp} size={14} />
          </button>
        )}
      </div>
      <div className="min-h-0 flex-1 overflow-auto">
        {shown === null && <Loading />}
        {shown && 'error' in shown && <Notice title="Could not show this file" hint={shown.error} centered />}
        {text !== null && rendered && (
          <div className="mx-auto max-w-[56rem] px-6 pt-5 pb-6">
            <ChatMarkdown text={text} file={linksFrom} />
          </div>
        )}
        {text !== null && !rendered && (
          <div className={`mono grid grid-cols-[auto_1fr] py-3 text-small leading-relaxed ${wrap ? '' : 'w-max min-w-full'}`}>
            {lines.map((line, i) => (
              <Fragment key={i}>
                <span className="px-3 text-right text-dim select-none">{i + 1}</span>
                <span className={`pr-3 ${wrap ? 'break-words whitespace-pre-wrap' : 'whitespace-pre'}`}>{line || ' '}</span>
              </Fragment>
            ))}
          </div>
        )}
      </div>
      {pickerAt && <FilePicker at={pickerAt} choices={choices} current={name} onChoose={choose} onClose={() => setPickerAt(null)} />}
    </div>
  )
}
