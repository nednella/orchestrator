import { Check, Code, Copy, Eye, FolderTree, WrapText } from 'lucide-react'
import type { ReactNode } from 'react'
import { editorLabel } from '../contextCard'
import { useCopied } from '../useCopied'
import { EditorMark } from './EditorMark'

const toolButton = 'btn btn-ghost h-7 w-7 flex-none justify-center px-0 aria-pressed:bg-(--bg-segment-on) aria-pressed:text-ink'

type FileHeaderProps = {
  crumbs: ReactNode
  editor: string
  // rendered is whether the file shows rendered; null when it has no rendered form.
  rendered: boolean | null
  // wrap is whether long lines wrap; null when the file shows no lines of text.
  wrap: boolean | null
  // copyable is the text the copy button copies, null when it has none.
  copyable: string | null
  treeOpen: boolean
  onOpenInEditor(): void
  onRendered(rendered: boolean): void
  onWrap(wrap: boolean): void
  onTree(open: boolean): void
}

// FileHeader is the bar over the open file: the way to it, and buttons that open it in the editor, switch it between its
// rendered form and its source, wrap its lines, copy it, and show or hide the tree.
export function FileHeader({ crumbs, editor, rendered, wrap, copyable, treeOpen, onOpenInEditor, onRendered, onWrap, onTree }: FileHeaderProps) {
  const [copied, copy] = useCopied(copyable ?? '')
  const label = editorLabel(editor)
  return (
    <div className="flex h-10 flex-none items-center gap-2 border-b border-(--border-rule) px-3 text-small">
      {crumbs}
      <div className="flex flex-none items-center gap-0.5">
        <button className={toolButton} title={label} aria-label={label} onClick={onOpenInEditor}>
          <EditorMark editor={editor} size={14} />
        </button>
        {rendered !== null && (
          <button className={toolButton} title={rendered ? 'Show source' : 'Show rendered'} aria-label={rendered ? 'Show source' : 'Show rendered'} aria-pressed={rendered} onClick={() => onRendered(!rendered)}>
            {rendered ? <Code size={14} aria-hidden="true" /> : <Eye size={14} aria-hidden="true" />}
          </button>
        )}
        {wrap !== null && (
          <button className={toolButton} title={wrap ? 'Disable word wrap' : 'Enable word wrap'} aria-label={wrap ? 'Disable word wrap' : 'Enable word wrap'} aria-pressed={wrap} onClick={() => onWrap(!wrap)}>
            <WrapText size={14} aria-hidden="true" />
          </button>
        )}
        {copyable !== null && (
          <button className={toolButton} title={copied ? 'Copied' : 'Copy contents'} aria-label={copied ? 'Copied' : 'Copy contents'} onClick={copy}>
            {copied ? <Check size={14} aria-hidden="true" /> : <Copy size={14} aria-hidden="true" />}
          </button>
        )}
        <button className={toolButton} title={treeOpen ? 'Hide file explorer' : 'Show file explorer'} aria-label={treeOpen ? 'Hide file explorer' : 'Show file explorer'} aria-pressed={treeOpen} onClick={() => onTree(!treeOpen)}>
          <FolderTree size={14} aria-hidden="true" />
        </button>
      </div>
    </div>
  )
}
