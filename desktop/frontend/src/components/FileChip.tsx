import { File, FileCode, FileImage, FileText, Folder, Globe } from 'lucide-react'
import { createPortal } from 'react-dom'
import { useContext, useEffect, useState, useSyncExternalStore } from 'react'
import type { KeyboardEvent, MouseEvent, ReactNode } from 'react'
import { useOrchestrator } from '../OrchestratorContext'
import { api } from '../api'
import { chipIcon } from '../chipIcon'
import type { ChipIcon } from '../chipIcon'
import type { ChipScope } from '../chipScope'
import { ChatChips } from '../fileChips'
import { baseOf, placeLabel, refText } from '../filePaths'
import type { FileRef } from '../filePaths'
import { formatBytes } from '../fileTree'
import { filesKey, pickFile, pickFolder } from '../filesView'
import { useLayout } from '../LayoutContext'
import type { FileChip as FileChipInfo } from '../types'
import { useDelayedTip } from '../useDelayedTip'
import type { Point } from '../useFloatingAt'
import { useFileChip } from '../useFileChip'
import { ContextMenu } from './ContextMenu'
import { FloatingTip } from './FloatingTip'
import { ImageViewer } from './ImageViewer'

const ICONS: Record<ChipIcon, typeof File> = { folder: Folder, code: FileCode, image: FileImage, text: FileText, web: Globe, file: File }

type FileChipProps = { fileRef: FileRef; fallback: ReactNode }

// FileChip is a file or folder a chat message names, as a chip that opens it, when the thread's folder or the disk has it;
// else the fallback, the text as it was written.
export function FileChip({ fileRef, fallback }: FileChipProps) {
  const chats = useContext(ChatChips)
  const chip = useFileChip(chats?.thread, fileRef.path)
  if (!chats || !chip?.exists) return <>{fallback}</>
  return <FileChipButton thread={chats.thread} scope={chats.scope} chip={chip} fileRef={fileRef} />
}

type FileChipButtonProps = { thread: string; scope: ChipScope; chip: FileChipInfo; fileRef: FileRef }

const isMenuKey = (e: KeyboardEvent) => e.key === 'ContextMenu' || (e.shiftKey && e.key === 'F10')

function FileChipButton({ thread, scope, chip, fileRef }: FileChipButtonProps) {
  const { report, project } = useOrchestrator('report', 'project')
  const { showPanelView } = useLayout()
  const { anchor, tipHandlers } = useDelayedTip()
  const [menu, setMenu] = useState<Point | null>(null)
  const [viewing, setViewing] = useState<string | null>(null)
  // A file of the thread's folder is named by its path from the folder, one outside it by its whole path.
  const identity = chip.inside && chip.rel !== '' ? chip.rel : chip.abs
  useEffect(() => scope.add(identity), [scope, identity])
  const name = useSyncExternalStore(scope.subscribe, scope.names).get(identity) ?? baseOf(identity)
  const icon = chipIcon(baseOf(identity), chip.dir)
  const Icon = ICONS[icon]
  const place = fileRef.line === undefined ? undefined : { line: fileRef.line, col: fileRef.col, endLine: fileRef.endLine }
  const target = chip.inside ? chip.rel : chip.abs
  const key = filesKey(thread, project?.key ?? '')

  const openInFiles = () => {
    if (chip.dir) pickFolder(key, target)
    else pickFile(key, target, place, !chip.inside)
    showPanelView('files', { thread })
  }
  const openInEditor = () => report(() => api.openFileInEditor(thread, chip.abs, fileRef.line))
  const reveal = () => report(() => api.revealInFinder(thread, chip.abs))
  const copy = (text: string) => report(() => navigator.clipboard.writeText(text))
  const openImage = () =>
    report(async () => {
      const file = chip.inside ? await api.chatReadFile(thread, chip.rel) : await api.chatReadOutside(thread, chip.abs)
      if (file.kind === 'image') setViewing(`data:${file.mime};base64,${file.data}`)
      else openInFiles()
    })

  const open = (e: MouseEvent) => {
    if (e.metaKey) openInEditor()
    else if (chip.dir && !chip.inside) reveal()
    else if (icon === 'image') openImage()
    else openInFiles()
  }

  return (
    <>
      <button
        type="button"
        className="file-chip"
        aria-label={`${chip.dir ? 'Folder' : 'File'} ${name}${placeLabel(fileRef)}`}
        aria-haspopup="menu"
        onClick={open}
        onContextMenu={(e) => {
          e.preventDefault()
          setMenu({ x: e.clientX, y: e.clientY })
        }}
        onKeyDown={(e) => {
          if (!isMenuKey(e)) return
          e.preventDefault()
          const box = e.currentTarget.getBoundingClientRect()
          setMenu({ x: box.left, y: box.bottom + 4 })
        }}
        {...tipHandlers}
      >
        <Icon size={13} className="flex-none" aria-hidden="true" />
        <span className="min-w-0 truncate">
          {name}
          {placeLabel(fileRef)}
        </span>
      </button>
      {anchor && !menu && <FloatingTip anchor={anchor} text={`${refText(chip.abs, fileRef)}${chip.dir ? '' : `\n${formatBytes(chip.size)}`}`} wrap />}
      {menu && (
        <ContextMenu
          label={`Actions for ${name}`}
          at={menu}
          onClose={() => setMenu(null)}
          items={[
            !(chip.dir && !chip.inside) && { label: 'Open in Files', run: openInFiles },
            { label: 'Open in editor', run: openInEditor },
            { label: 'Reveal in Finder', run: reveal },
            { label: 'Copy path', separated: true, run: () => copy(refText(chip.abs, fileRef)) },
            chip.inside && { label: 'Copy relative path', run: () => copy(refText(chip.rel === '' ? '.' : chip.rel, fileRef)) },
          ]}
        />
      )}
      {viewing && createPortal(<ImageViewer url={viewing} alt={name} onClose={() => setViewing(null)} />, document.body)}
    </>
  )
}
