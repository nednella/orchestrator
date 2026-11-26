import { File, Virtualizer } from '@pierre/diffs/react'
import type { CSSProperties } from 'react'
import { useColorScheme } from '../useColorScheme'

type FileCodeViewProps = { path: string; text: string; wrap: boolean }

export const CODE_THEMES = { dark: 'pierre-dark', light: 'pierre-light' } as const

export const CODE_LOOK = {
  '--diffs-bg': 'transparent',
  '--diffs-fg': 'var(--text)',
  '--diffs-fg-number': 'var(--text-dim)',
  '--diffs-font-family': 'var(--font-mono)',
  '--diffs-font-size': 'var(--fs-small)',
  '--diffs-line-height': '1.55',
} as CSSProperties

// FileCodeView is a file's text with its lines numbered and its code coloured by its language, read only. Only the
// lines in view are drawn, so a file of a megabyte scrolls as one of a screen does.
export function FileCodeView({ path, text, wrap }: FileCodeViewProps) {
  const scheme = useColorScheme()
  return (
    <Virtualizer className="file-code min-h-0 flex-1 overflow-auto" contentClassName="min-w-0">
      <File
        file={{ name: path, contents: text, cacheKey: `${path}:${text.length}` }}
        options={{ theme: CODE_THEMES, themeType: scheme, disableFileHeader: true, overflow: wrap ? 'wrap' : 'scroll' }}
        style={CODE_LOOK}
      />
    </Virtualizer>
  )
}
