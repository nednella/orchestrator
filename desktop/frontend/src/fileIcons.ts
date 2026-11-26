import type { FileTreeIconConfig } from '@pierre/trees'

const DOC = '<path fill="currentColor" opacity=".45" d="M8 1v3a3 3 0 0 0 3 3h3v5.5a2.5 2.5 0 0 1-2.5 2.5h-7A2.5 2.5 0 0 1 2 12.5v-9A2.5 2.5 0 0 1 4.5 1z"/><path fill="currentColor" d="M9.5 1a.5.5 0 0 1 .354.146l4 4A.5.5 0 0 1 14 5.5V6h-3a2 2 0 0 1-2-2V1z"/>'

// A glyph is the page every file icon is drawn on and a mark on it, in the colour of its token.
const glyph = (id: string, token: string, mark: string) =>
  `<symbol id="orc-file-${id}" viewBox="0 0 16 16"><g style="color:var(--file-icon-${token})">${DOC}<g fill="none" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round">${mark}</g></g></symbol>`

// A badge is a page with the language's letters on it.
const badge = (id: string, letters: string) =>
  `<symbol id="orc-file-${id}" viewBox="0 0 16 16"><g style="color:var(--file-icon-language)">${DOC}<text x="8" y="12.4" font-size="5.6" font-weight="700" font-family="system-ui,sans-serif" text-anchor="middle" fill="currentColor" stroke="none">${letters}</text></g></symbol>`

const SPRITE = `<svg data-icon-sprite aria-hidden="true" width="0" height="0">
${glyph('pdf', 'pdf', '<path d="M5 8.5h6M5 10.5h6M5 12.5h3.5"/>')}
${glyph('word', 'word', '<path d="M4.5 8.5h7M4.5 10.5h7M4.5 12.5h4"/>')}
${glyph('sheet', 'sheet', '<path d="M4.5 8.5h7v4.5h-7zM4.5 10.75h7M8 8.5V13"/>')}
${glyph('slides', 'slides', '<path d="M4.5 8.5h7v4h-7z"/>')}
${glyph('epub', 'epub', '<path d="M8 8.5v4.5M4.5 8.5h3.5v4.5H4.5zM8 8.5h3.5v4.5H8z"/>')}
${glyph('video', 'video', '<path d="M6.5 8.5v4l3.5-2z"/>')}
${glyph('diff', 'diff', '<path d="M8 8v4M6 10h4M5.5 13h5"/>')}
${glyph('xml', 'xml', '<path d="m6.5 8.5-2 2 2 2M9.5 8.5l2 2-2 2"/>')}
${glyph('test', 'test', '<path d="M7 8h2M7.5 8v2.2L5.5 13h5l-2-2.8V8"/>')}
${glyph('agent', 'agent', '<circle cx="8" cy="10.5" r="2.4"/><path d="M8 10.5h.01"/>')}
${glyph('pnpm', 'pnpm', '<path d="M4.5 8.5h2M7 8.5h2M9.5 8.5h2M4.5 11h2M7 11h2M4.5 13.2h2"/>')}
${glyph('vitest', 'vitest', '<path d="m5.5 10.5 1.8 2 3.4-4"/>')}
${badge('csharp', 'C#')}
${badge('dart', 'Dt')}
${badge('elixir', 'Ex')}
${badge('haskell', 'λ')}
${badge('java', 'Jv')}
${badge('kotlin', 'Kt')}
${badge('lua', 'Lu')}
${badge('objc', 'ObjC')}
${badge('php', 'php')}
${badge('powershell', 'PS')}
${badge('r', 'R')}
${badge('scala', 'Sc')}
${badge('toml', 'T')}
<symbol id="orc-file-loader" viewBox="0 0 16 16"><path fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" d="M8 2a6 6 0 1 0 6 6"/></symbol>
</svg>`

const byExtension: Record<string, string> = {
  pdf: 'pdf',
  doc: 'word',
  docx: 'word',
  odt: 'word',
  rtf: 'word',
  xls: 'sheet',
  xlsx: 'sheet',
  ods: 'sheet',
  csv: 'sheet',
  tsv: 'sheet',
  ppt: 'slides',
  pptx: 'slides',
  odp: 'slides',
  key: 'slides',
  epub: 'epub',
  mp4: 'video',
  mov: 'video',
  mkv: 'video',
  webm: 'video',
  avi: 'video',
  m4v: 'video',
  diff: 'diff',
  patch: 'diff',
  xml: 'xml',
  xsd: 'xml',
  xsl: 'xml',
  xslt: 'xml',
  'test.ts': 'test',
  'test.tsx': 'test',
  'test.js': 'test',
  'test.jsx': 'test',
  'test.mjs': 'test',
  'spec.ts': 'test',
  'spec.tsx': 'test',
  'spec.js': 'test',
  'spec.jsx': 'test',
  'spec.mjs': 'test',
  cs: 'csharp',
  dart: 'dart',
  ex: 'elixir',
  exs: 'elixir',
  hs: 'haskell',
  java: 'java',
  kt: 'kotlin',
  kts: 'kotlin',
  lua: 'lua',
  m: 'objc',
  mm: 'objc',
  php: 'php',
  ps1: 'powershell',
  psm1: 'powershell',
  r: 'r',
  scala: 'scala',
  sc: 'scala',
  toml: 'toml',
  heic: 'file-tree-builtin-image',
  heif: 'file-tree-builtin-image',
}

const byName: Record<string, string> = {
  'package.json': 'file-tree-builtin-npm',
  'package-lock.json': 'file-tree-builtin-npm',
  'tsconfig.json': 'file-tree-builtin-typescript',
  'pnpm-lock.yaml': 'orc-file-pnpm',
  'pnpm-workspace.yaml': 'orc-file-pnpm',
  '.pnpmfile.cjs': 'orc-file-pnpm',
  'vitest.config.ts': 'orc-file-vitest',
  'vitest.config.js': 'orc-file-vitest',
  'vitest.config.mts': 'orc-file-vitest',
  'vitest.workspace.ts': 'orc-file-vitest',
  'rolldown.config.ts': 'file-tree-builtin-vite',
  'rolldown.config.js': 'file-tree-builtin-vite',
  '.oxlintrc.json': 'file-tree-builtin-oxc',
  'oxlint.config.ts': 'file-tree-builtin-oxc',
  'agents.md': 'orc-file-agent',
  'claude.md': 'orc-file-agent',
}

const own = (name: string) => (name.includes('-builtin-') ? name : `orc-file-${name}`)

// FILE_ICONS is the tree's icon set: the library's complete set in colour, with the file types it has no icon for.
export const FILE_ICONS: FileTreeIconConfig = {
  set: 'complete',
  colored: true,
  spriteSheet: SPRITE,
  byFileName: byName,
  byFileExtension: Object.fromEntries(Object.entries(byExtension).map(([ext, icon]) => [ext, own(icon)])),
}

export const LOADER_ICON = 'orc-file-loader'
