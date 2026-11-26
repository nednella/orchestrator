import { File } from '@pierre/diffs/react'
import type { SupportedLanguages } from '@pierre/diffs/react'
import { useColorScheme } from '../useColorScheme'
import { CODE_LOOK, CODE_THEMES } from './FileCodeView'

type FileHighlightProps = { language: string; code: string }

// FileHighlight is a piece of code coloured as the file view colours it, in the same theme, with no line numbers.
export function FileHighlight({ language, code }: FileHighlightProps) {
  const scheme = useColorScheme()
  return (
    <File
      file={{ name: `code.${language || 'txt'}`, contents: code, lang: (language || 'text') as SupportedLanguages }}
      options={{ theme: CODE_THEMES, themeType: scheme, disableFileHeader: true, disableLineNumbers: true, overflow: 'scroll' }}
      style={CODE_LOOK}
    />
  )
}
