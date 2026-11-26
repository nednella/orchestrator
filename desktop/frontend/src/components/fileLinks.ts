import { createContext } from 'react'

// FileLinks is the file a markdown text is, when the Files view shows it: its Files view's key, and its path, which its
// relative links and images start from.
export const FileLinks = createContext<{ key: string; path: string } | null>(null)
