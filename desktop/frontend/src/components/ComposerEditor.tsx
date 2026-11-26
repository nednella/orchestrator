import { useEffect, useImperativeHandle, useLayoutEffect, useRef, useState } from 'react'
import type { RefObject } from 'react'
import { EditorContent, useEditor } from '@tiptap/react'
import type { Editor, JSONContent, UseEditorOptions } from '@tiptap/react'
import { Node as DocNode, Slice } from '@tiptap/pm/model'
import { Selection } from '@tiptap/pm/state'
import { composerSchema, draftOf, fromPlainText } from '../composerMarkdown'
import type { ComposerDraft } from '../composerMarkdown'
import { ComposerTyping } from '../composerTyping'
import { MarkEdges } from '../markEdges'

export type ComposerInput = {
  focus(at?: 'end'): void
  caretOnFirstLine(): boolean
  caretOnLastLine(): boolean
  blur(): void
  insert(text: string): void
  insertLiteral(text: string): void
  insertDoc(doc: JSONContent): void
  deleteBefore(length: number): void
}

// onEdgeLine measures the caret against the first or last line box of the draft, so a wrapped line counts as several.
function onEdgeLine(editor: Editor | null, edge: 'first' | 'last'): boolean {
  if (!editor || editor.isEmpty) return true
  const { doc, selection } = editor.state
  const caret = editor.view.coordsAtPos(selection.head)
  if (edge === 'first') {
    const line = editor.view.coordsAtPos(Selection.atStart(doc).from)
    return caret.top < line.bottom
  }
  const line = editor.view.coordsAtPos(Selection.atEnd(doc).to)
  return caret.bottom > line.top
}

const PROMPT_PLACEHOLDER = 'Ask anything. @ files, / commands'

type ComposerEditorProps = {
  inputRef: RefObject<ComposerInput | null>
  value: ComposerDraft
  label: string
  placeholder?: string
  onChange(draft: ComposerDraft): void
  onCaret?(before: string, after: string): void
  onKeyDown?(e: KeyboardEvent): void
  onAttach?(data: DataTransfer): boolean
  onSend(): void
}

// Edits markdown as rendered text. The value is read in when it changes from outside and written out on every edit.
export function ComposerEditor({ inputRef, value, label, placeholder = PROMPT_PLACEHOLDER, onChange, onCaret, onKeyDown, onAttach, onSend }: ComposerEditorProps) {
  const handlers = useRef({ onChange, onCaret, onKeyDown, onAttach, onSend })
  const written = useRef(value)
  const focused = useRef(false)
  const [empty, setEmpty] = useState(() => value.text.trim() === '')

  useLayoutEffect(() => {
    handlers.current = { onChange, onCaret, onKeyDown, onAttach, onSend }
  })

  const [options] = useState<UseEditorOptions>(() => ({
    extensions: [...composerSchema, ComposerTyping.configure({ send: () => handlers.current.onSend() }), MarkEdges],
    content: value.doc,
    // Pasted text stays as it is: no `**bold**` turned bold.
    enablePasteRules: false,
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
      // Pasted text is taken as it is and joins the line it lands in. Pasted HTML goes through the schema.
      clipboardTextParser: (text, _context, _plain, view) => new Slice(DocNode.fromJSON(view.state.schema, fromPlainText(text)).content, 1, 1),
    },
    onFocus: () => {
      focused.current = true
    },
    onTransaction: ({ editor, transaction }) => {
      if (transaction.docChanged && !transaction.getMeta('preventUpdate')) {
        written.current = draftOf(editor.getJSON())
        handlers.current.onChange(written.current)
      }
      const { doc, selection } = editor.state
      handlers.current.onCaret?.(doc.textBetween(0, selection.from, '\n', '\n'), doc.textBetween(selection.to, doc.content.size, '\n', '\n'))
      setEmpty(editor.isEmpty)
    },
  }))
  const editor = useEditor(options)

  useEffect(() => {
    if (!editor || value === written.current) return
    written.current = value
    // Undo must not bring back a message already sent.
    editor.chain().setMeta('addToHistory', false).setContent(value.doc, { emitUpdate: false }).run()
  }, [editor, value])

  useEffect(() => {
    editor?.view.dom.setAttribute('aria-label', label)
    editor?.view.dom.setAttribute('aria-placeholder', placeholder)
  }, [editor, label, placeholder])

  useImperativeHandle(
    inputRef,
    () => ({
      focus: (at) => editor?.commands.focus(at),
      caretOnFirstLine: () => onEdgeLine(editor, 'first'),
      caretOnLastLine: () => onEdgeLine(editor, 'last'),
      blur: () => editor?.commands.blur(),
      insert: (text) => editor?.commands.insertContent({ type: 'text', text }),
      insertLiteral: (text) => editor?.commands.insertContent({ type: 'text', text, marks: [{ type: 'literal' }] }),
      insertDoc: (doc) => {
        // A caret never placed sits at the start of the draft; the quote goes after it.
        const chain = editor?.chain()
        if (!focused.current) chain?.focus('end')
        chain?.insertContent(doc.content ?? []).run()
      },
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
