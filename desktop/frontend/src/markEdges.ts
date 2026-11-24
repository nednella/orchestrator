import { Extension } from '@tiptap/react'
import type { Editor } from '@tiptap/react'
import { Mark } from '@tiptap/pm/model'
import { Plugin, Selection, TextSelection } from '@tiptap/pm/state'
import type { EditorState } from '@tiptap/pm/state'
import { Decoration, DecorationSet } from '@tiptap/pm/view'
import type { EditorView } from '@tiptap/pm/view'

type Side = 'before' | 'after'
type Edge = { pos: number; before: readonly Mark[]; after: readonly Mark[]; side: Side | null; atEnd: boolean }

// Literal text looks like any other text, so its mark makes no edge.
const shown = (marks: readonly Mark[] = Mark.none) => marks.filter((mark) => mark.type.name !== 'literal')

// Where the styles of the text before and after the caret differ, the caret has two stops: one takes the style
// before the edge for what is typed next, the other the style after it. Stored marks record which one it is on.
function edgeAt(state: EditorState): Edge | null {
  const { selection } = state
  if (!(selection instanceof TextSelection) || !selection.$cursor) return null
  const $pos = selection.$cursor
  if ($pos.textOffset > 0) return null
  const before = shown($pos.nodeBefore?.marks)
  const after = shown($pos.nodeAfter?.marks)
  if (Mark.sameSet(before, after)) return null
  const typing = shown(state.storedMarks ?? $pos.marks())
  const side = Mark.sameSet(typing, after) ? 'after' : Mark.sameSet(typing, before) ? 'before' : null
  return { pos: $pos.pos, before, after, side, atEnd: $pos.pos === $pos.end() }
}

function takeSide(editor: Editor, marks: readonly Mark[]) {
  return editor.commands.command(({ tr }) => {
    tr.setStoredMarks(marks)
    return true
  })
}

// Moves the caret one character, landing on the stop nearer where it came from if it reaches an edge. The drawn
// caret sits where the browser would move the native one, so at an edge the editor moves it itself.
function moveTo(editor: Editor, pos: number) {
  const from = editor.state.selection.from
  return editor.commands.command(({ tr }) => {
    tr.setSelection(TextSelection.create(tr.doc, pos))
    const $pos = tr.doc.resolve(pos)
    const before = shown($pos.nodeBefore?.marks)
    const after = shown($pos.nodeAfter?.marks)
    if ($pos.textOffset === 0 && !Mark.sameSet(before, after)) tr.setStoredMarks(pos < from ? after : before)
    return true
  })
}

const isLowSurrogate = (code: number) => code >= 0xdc00 && code <= 0xdfff
const isHighSurrogate = (code: number) => code >= 0xd800 && code <= 0xdbff

function stepRight(editor: Editor) {
  const edge = edgeAt(editor.state)
  if (!edge?.side) return false
  if (edge.side === 'before') return takeSide(editor, edge.after)
  const { $from } = editor.state.selection
  if (edge.atEnd) {
    // Past styled text that ends its block, the caret goes on to the next block, never into a new space.
    const next = Selection.findFrom(editor.state.doc.resolve($from.after()), 1)
    return editor.commands.command(({ tr }) => {
      if (next) tr.setSelection(next)
      return true
    })
  }
  const text = $from.nodeAfter?.isText ? ($from.nodeAfter.text ?? '') : ''
  if (!text) return false
  return moveTo(editor, edge.pos + (isHighSurrogate(text.charCodeAt(0)) ? 2 : 1))
}

function stepLeft(editor: Editor) {
  const edge = edgeAt(editor.state)
  if (edge?.side === 'after') return takeSide(editor, edge.before)
  const { $cursor } = editor.state.selection as TextSelection
  if (!$cursor) return false
  const text = $cursor.nodeBefore?.isText ? ($cursor.nodeBefore.text ?? '') : ''
  const size = isLowSurrogate(text.charCodeAt(text.length - 1)) ? 2 : 1
  // Away from an edge, the browser moves the caret, unless the move reaches one.
  if (!text || (!edge?.side && $cursor.textOffset !== size)) return false
  return moveTo(editor, $cursor.pos - size)
}

// The browser would delete the drawn caret instead of a character, so at an edge the editor deletes it, and the
// caret keeps its stop.
function deleteAtEdge(editor: Editor, dir: 1 | -1) {
  const edge = edgeAt(editor.state)
  const { $from } = editor.state.selection
  const node = dir < 0 ? $from.nodeBefore : $from.nodeAfter
  if (!edge?.side || !node?.isText) return false
  const text = node.text ?? ''
  const size = dir < 0 ? (isLowSurrogate(text.charCodeAt(text.length - 1)) ? 2 : 1) : isHighSurrogate(text.charCodeAt(0)) ? 2 : 1
  const typing = edge.side === 'after' ? edge.after : edge.before
  return editor.commands.command(({ tr }) => {
    tr.delete(dir < 0 ? edge.pos - size : edge.pos, dir < 0 ? edge.pos : edge.pos + size).setStoredMarks(typing)
    return true
  })
}

function caret() {
  const el = document.createElement('span')
  el.className = 'composer-caret'
  return el
}

export const MarkEdges = Extension.create({
  name: 'markEdges',
  priority: 1000,

  addKeyboardShortcuts() {
    return {
      ArrowRight: ({ editor }) => stepRight(editor),
      ArrowLeft: ({ editor }) => stepLeft(editor),
      Backspace: ({ editor }) => deleteAtEdge(editor, -1),
      Delete: ({ editor }) => deleteAtEdge(editor, 1),
    }
  },

  addProseMirrorPlugins() {
    let view: EditorView | null = null
    const drawn = (state: EditorState) => {
      const edge = edgeAt(state)
      return edge?.side && !view?.composing ? edge : null
    }
    return [
      new Plugin({
        view(editorView) {
          view = editorView
          return {}
        },
        props: {
          // The native caret cannot show which stop it is on, so a drawn one sits inside or outside the styled text.
          decorations(state) {
            const edge = drawn(state)
            if (!edge) return null
            const after = edge.side === 'after'
            return DecorationSet.create(state.doc, [Decoration.widget(edge.pos, caret, { side: after ? 1 : -1, marks: after ? edge.after : edge.before, key: edge.side ?? '' })])
          },
          attributes: (state): Record<string, string> => (drawn(state) ? { 'data-drawn-caret': '' } : {}),
        },
      }),
    ]
  },
})
