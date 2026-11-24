import { Extension } from '@tiptap/react'
import type { ChainedCommands, Editor } from '@tiptap/react'
import type { ResolvedPos } from '@tiptap/pm/model'
import { Plugin, Selection } from '@tiptap/pm/state'

type ComposerTypingOptions = { send(): void }

const WRAPS: Record<string, string> = { '*': 'italic', _: 'italic', '`': 'code' }
const LISTS = new Set(['bulletList', 'orderedList'])
const FENCE = /^```([^\s`]*)$/
const LINE_MARKER = /^(?:([-*+])|(\d{1,9})\.|(>)|(#{1,6}))$/

const inList = ($pos: ResolvedPos) => Array.from({ length: $pos.depth }, (_, d) => $pos.node(d + 1)).some((n) => n.type.name === 'listItem')
const inCodeBlock = ($pos: ResolvedPos) => $pos.parent.type.name === 'codeBlock'

// The caret's line in a paragraph: its text up to the caret, and the hard break that starts it, if any.
function lineBefore($pos: ResolvedPos) {
  let breakOffset = -1
  $pos.parent.forEach((node, offset) => {
    if (node.type.name === 'hardBreak' && offset < $pos.parentOffset) breakOffset = offset
  })
  return {
    text: $pos.parent.textBetween(breakOffset + 1, $pos.parentOffset),
    breakAt: breakOffset < 0 ? null : $pos.start() + breakOffset,
  }
}

// Clears the caret's line and, when a hard break starts it, splits it off into its own paragraph.
function clearLine(editor: Editor, $pos: ResolvedPos): ChainedCommands {
  const { breakAt } = lineBefore($pos)
  return editor.chain().command(({ tr }) => {
    const from = breakAt ?? $pos.start()
    tr.delete(from, $pos.pos)
    if (breakAt !== null) tr.split(from)
    return true
  })
}

function openFence(editor: Editor) {
  const { $from, empty } = editor.state.selection
  if (!empty || $from.parent.type.name !== 'paragraph' || $from.parentOffset !== $from.parent.content.size) return false
  const fence = FENCE.exec(lineBefore($from).text)
  if (!fence) return false
  return clearLine(editor, $from)
    .setNode('codeBlock', { language: fence[1] || null })
    .run()
}

function closeFence(editor: Editor) {
  const { $from, empty } = editor.state.selection
  const code = $from.parent.textContent
  if (!empty || $from.parentOffset !== code.length) return false
  const lineStart = code.lastIndexOf('\n') + 1
  if (code.slice(lineStart).trim() !== '```') return false
  const chain = editor.chain().deleteRange({ from: $from.start() + Math.max(lineStart - 1, 0), to: $from.pos })
  if (lineStart === 0) return chain.setParagraph().run()
  const next = editor.state.doc.nodeAt($from.after())
  if (next?.type.name !== 'paragraph' || next.content.size > 0) return chain.exitCode().run()
  return chain
    .command(({ tr }) => {
      tr.setSelection(Selection.near(tr.doc.resolve(tr.mapping.map($from.after()))))
      return true
    })
    .run()
}

// A new item, like a new line, starts unstyled, as in markdown source. On an empty item it leaves the list.
function newListItem(editor: Editor) {
  return editor
    .chain()
    .first(({ commands }) => [() => commands.splitListItem('listItem'), () => commands.liftEmptyBlock()])
    .command(({ tr }) => {
      tr.setStoredMarks([])
      return true
    })
    .run()
}

// Typing a list, quote or heading marker after a line break starts that block on the line, as it does at the
// start of a paragraph.
function startBlockAfterBreak(editor: Editor, from: number) {
  const $from = editor.state.doc.resolve(from)
  if ($from.parent.type.name !== 'paragraph' || inList($from)) return false
  const line = lineBefore($from)
  const marker = LINE_MARKER.exec(line.text)
  if (line.breakAt === null || !marker) return false
  const chain = clearLine(editor, $from)
  if (marker[1]) return chain.wrapInList('bulletList').run()
  if (marker[2]) return chain.wrapInList('orderedList', { start: Number(marker[2]) }).run()
  if (marker[3]) return chain.wrapIn('blockquote').run()
  return chain.setNode('heading', { level: marker[4].length }).run()
}

export const ComposerTyping = Extension.create<ComposerTypingOptions>({
  name: 'composerTyping',
  priority: 1000,

  addOptions() {
    return { send: () => {} }
  },

  addKeyboardShortcuts() {
    return {
      Enter: ({ editor }) => {
        const { $from } = editor.state.selection
        if (inCodeBlock($from)) return closeFence(editor)
        if (inList($from)) return newListItem(editor)
        if (openFence(editor)) return true
        this.options.send()
        return true
      },
      'Shift-Enter': ({ editor }) => {
        const { $from } = editor.state.selection
        if (inCodeBlock($from)) {
          return editor.commands.command(({ tr }) => {
            tr.insertText('\n')
            return true
          })
        }
        if (inList($from)) return newListItem(editor)
        // On an empty line in a quote it leaves the quote.
        if ($from.parent.content.size === 0 && $from.depth > 1) return editor.commands.liftEmptyBlock()
        if ($from.parent.type.name !== 'paragraph' || $from.parent.content.size === 0) return editor.commands.splitBlock({ keepMarks: false })
        // A second line break in a row starts a new paragraph instead.
        if ($from.nodeBefore?.type.name === 'hardBreak') return editor.chain().deleteRange({ from: $from.pos - 1, to: $from.pos }).splitBlock({ keepMarks: false }).run()
        return editor.commands.setHardBreak()
      },
      'Mod-Enter': () => {
        this.options.send()
        return true
      },
      Backspace: ({ editor }) => {
        const { $from, empty } = editor.state.selection
        if (!empty || !inCodeBlock($from) || $from.parentOffset > 0) return false
        return editor.commands.setParagraph()
      },
      Tab: ({ editor }) => {
        if (!inList(editor.state.selection.$from)) return false
        editor.commands.sinkListItem('listItem')
        return true
      },
      'Shift-Tab': ({ editor }) => {
        if (!inList(editor.state.selection.$from)) return false
        editor.commands.liftListItem('listItem')
        return true
      },
    }
  },

  addProseMirrorPlugins() {
    const { editor } = this
    return [
      new Plugin({
        // Markdown cannot write two lists of one kind side by side, so they are kept as one, numbered on.
        appendTransaction(transactions, _old, state) {
          if (!transactions.some((tr) => tr.docChanged)) return null
          const joins: number[] = []
          state.doc.descendants((node, pos) => {
            if (LISTS.has(node.type.name) && state.doc.resolve(pos).nodeBefore?.type === node.type) joins.push(pos)
          })
          if (joins.length === 0) return null
          const tr = state.tr
          for (const pos of joins.reverse()) tr.join(pos)
          return tr
        },
        props: {
          handleTextInput(view, from, to, text) {
            if (view.composing) return false
            if (from === to) return text === ' ' && startBlockAfterBreak(editor, from)
            const mark = WRAPS[text]
            if (!mark || inCodeBlock(view.state.doc.resolve(from))) return false
            return editor.commands.toggleMark(mark)
          },
        },
      }),
    ]
  },
})
