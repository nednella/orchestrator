import { createContext } from 'react'
import type { ChipScope } from './chipScope'

// ChatChips is the thread a chat message belongs to, whose folder its paths start from, and the chips the message shows.
// It is set for the text of an assistant's reply only; elsewhere a path is no chip.
export const ChatChips = createContext<{ thread: string; scope: ChipScope } | null>(null)

// InsideLink is set in the text of a link, where a chip would be a button inside a link.
export const InsideLink = createContext(false)
