import { useEffect, useImperativeHandle, useLayoutEffect, useRef, useState } from 'react'
import type { RefObject } from 'react'
import { EditorContent, useEditor } from '@tiptap/react'
import type { UseEditorOptions } from '@tiptap/react'
import { Node as DocNode, Slice } from '@tiptap/pm/model'
import { composerSchema, fromMarkdown, toMarkdown } from '../composerMarkdown'
import { ComposerTyping } from '../composerTyping'

export type ComposerInput = {
  focus(): void
  blur(): void
  insert(text: string): void
  deleteBefore(length: number): void
}

type ComposerEditorProps = {
  inputRef: RefObject<ComposerInput | null>
  value: string
  label: string
  placeholder: string
  onChange(text: string): void
  onCaret?(before: string, after: string): void
  onKeyDown?(e: KeyboardEvent): void
  onAttach?(data: DataTransfer): boolean
  onSend(): void
}

// Edits markdown as rendered text. The value stays a markdown string: it is read in when it changes from outside
// and written out on every edit.
export function ComposerEditor({ inputRef, value, label, placeholder, onChange, onCaret, onKeyDown, onAttach, onSend }: ComposerEditorProps) {
  const handlers = useRef({ onChange, onCaret, onKeyDown, onAttach, onSend })
  const written = useRef(value)
  const [empty, setEmpty] = useState(() => value.trim() === '')

  useLayoutEffect(() => {
    handlers.current = { onChange, onCaret, onKeyDown, onAttach, onSend }
  })

  const [options] = useState<UseEditorOptions>(() => ({
    extensions: [composerSchema, ComposerTyping.configure({ send: () => handlers.current.onSend() })],
    content: fromMarkdown(value),
    immediatelyRender: true,
    shouldRerenderOnTransaction: false,
    editorProps: {
      attributes: { class: 'composer-editor markdown', role: 'textbox', 'aria-multiline': 'true' },
      // While an input method composes, its keys are its own: Enter picks a candidate, it does not send.
      handleDOMEvents: { keydown: (_view, e) => e.isComposing || e.keyCode === 229 },
      handleKeyDown: (_view, e) => {
        handlers.current.onKeyDown?.(e)
        return e.defaultPrevented
      },
      handlePaste: (_view, e) => Boolean(e.clipboardData && handlers.current.onAttach?.(e.clipboardData)),
      // The composer's box takes dropped images; the editor must not insert the file's name.
      handleDrop: (_view, e) => Boolean(handlers.current.onAttach && e.dataTransfer?.types.includes('Files')),
      clipboardTextParser: (text, _context, _plain, view) => {
        const { content } = DocNode.fromJSON(view.state.schema, fromMarkdown(text))
        // Pasted text joins the line it lands in; a pasted heading, list or block keeps its own.
        const open = (node: DocNode | null) => (node?.type.name === 'paragraph' ? 1 : 0)
        return new Slice(content, open(content.firstChild), open(content.lastChild))
      },
    },
    onTransaction: ({ editor, transaction }) => {
      if (transaction.docChanged && !transaction.getMeta('preventUpdate')) {
        written.current = toMarkdown(editor.getJSON())
        handlers.current.onChange(written.current)
      }
      const { $from, $to } = editor.state.selection
      handlers.current.onCaret?.($from.parent.textBetween(0, $from.parentOffset, undefined, '\n'), $to.parent.textBetween($to.parentOffset, $to.parent.content.size, undefined, '\n'))
      setEmpty(editor.isEmpty)
    },
  }))
  const editor = useEditor(options)

  useEffect(() => {
    if (!editor || value === written.current) return
    written.current = value
    editor.commands.setContent(fromMarkdown(value), { emitUpdate: false })
  }, [editor, value])

  useEffect(() => {
    editor?.view.dom.setAttribute('aria-label', label)
    editor?.view.dom.setAttribute('aria-placeholder', placeholder)
  }, [editor, label, placeholder])

  useImperativeHandle(
    inputRef,
    () => ({
      focus: () => editor?.commands.focus(),
      blur: () => editor?.commands.blur(),
      insert: (text) => editor?.commands.insertContent({ type: 'text', text }),
      deleteBefore: (length) => {
        const at = editor?.state.selection.from ?? 0
        editor?.commands.deleteRange({ from: at - length, to: at })
      },
    }),
    [editor],
  )

  return (
    <div className="composer-input text-body">
      {empty && (
        <div className="composer-placeholder" aria-hidden="true">
          {placeholder}
        </div>
      )}
      <EditorContent editor={editor} />
    </div>
  )
}
