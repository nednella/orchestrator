import { useEffect, useState } from 'react'
import type { RefObject } from 'react'
import { errorMessage } from './api'
import { commandMatches, commandToken, mentionToken } from './chatComposer'
import type { Command } from './chatTimeline'
import { doubledSpace } from './composerPick'
import type { ComposerInput } from './components/ComposerEditor'
import type { PickRow } from './components/ComposerPickList'
import { Icon } from './components/Icon'

// PickList is the pick list open in a prompt composer: what it lists and what picking a row does.
export type PickList = { label: string; rows: PickRow[]; empty: string; pick(index: number): void }

// Files are what the folder lists for the mention, or why it did not.
type Files = { paths: string[]; error: string }

// caret is the text before and after the caret; off keeps both lists closed. commands are the slash commands to offer,
// and noCommands what the list says when there are none at all. listFiles lists the folder's files for a mention, and
// keeps its identity while the folder stays the same. onPicked runs after a row is put in.
type PromptPicksOptions = {
  input: RefObject<ComposerInput | null>
  caret: { before: string; after: string }
  off: boolean
  commands: Command[]
  noCommands: string
  listFiles(query: string): Promise<string[]>
  onPicked(): void
}

const commandRow = (c: Command): PickRow => ({ key: c.name, icon: <Icon name="terminal" size={14} />, name: `/${c.name}`, description: c.description, badge: c.argumentHint })

const fileRow = (path: string): PickRow => {
  const slash = path.lastIndexOf('/')
  return { key: path, icon: <Icon name="file" size={14} />, name: path.slice(slash + 1), description: path.slice(0, slash + 1) }
}

// usePromptPicks is the / command list while the caret is in a message's first word that starts with /, and the @ file
// list while it ends a word that starts with @.
export function usePromptPicks({ input, caret, off, commands, noCommands, listFiles, onPicked }: PromptPicksOptions): PickList | null {
  const [files, setFiles] = useState<Files | null>(null)
  const command = off ? null : commandToken(caret.before)
  const mention = off || command !== null ? null : mentionToken(caret.before)

  useEffect(() => {
    if (mention === null) return
    let current = true
    listFiles(mention).then(
      (paths) => current && setFiles({ paths, error: '' }),
      (err) => current && setFiles({ paths: [], error: errorMessage(err) }),
    )
    return () => {
      current = false
    }
  }, [listFiles, mention])

  // replace puts text where the token typed so far is, with the @ or / before it.
  const replace = (token: string, text: string) => {
    input.current?.deleteBefore(token.length + 1)
    input.current?.insertLiteral(doubledSpace(text, caret.after) ? text.slice(0, -1) : text)
    onPicked()
  }

  if (command !== null) {
    const matches = commandMatches(commands, command)
    return {
      label: 'Commands',
      rows: matches.map(commandRow),
      empty: commands.length === 0 ? noCommands : 'No command matches',
      pick: (i) => replace(command, `/${matches[i].name} `),
    }
  }
  if (mention === null) return null
  const shown = files ?? { paths: [], error: '' }
  return {
    label: 'Files',
    rows: shown.paths.map(fileRow),
    empty: files === null ? 'Listing the files' : shown.error || 'No file matches',
    pick: (i) => replace(mention, `@${shown.paths[i]} `),
  }
}
