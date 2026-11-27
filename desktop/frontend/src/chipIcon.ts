// ChipIcon is the kind of icon a file chip has.
export type ChipIcon = 'folder' | 'code' | 'image' | 'text' | 'web' | 'file'

const IMAGE = new Set(['png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp', 'ico', 'avif', 'svg', 'heic', 'tiff'])
const TEXT = new Set(['md', 'mdx', 'markdown', 'txt', 'rst', 'adoc', 'log', 'rtf'])
const WEB = new Set(['html', 'htm', 'xhtml'])
const CODE = new Set(['ts', 'tsx', 'js', 'jsx', 'mjs', 'cjs', 'go', 'py', 'rs', 'rb', 'java', 'kt', 'swift', 'c', 'h', 'cc', 'cpp', 'hpp', 'cs', 'php', 'lua', 'sh', 'bash', 'zsh', 'fish', 'sql', 'css', 'scss', 'less', 'vue', 'svelte', 'json', 'jsonc', 'yaml', 'yml', 'toml', 'xml', 'ini', 'mod', 'sum', 'lock'])
const CODE_NAMES = new Set(['Makefile', 'Dockerfile', 'Rakefile', 'Gemfile', 'Justfile', 'Jenkinsfile'])

// chipIcon is the icon of the file chip for a file or folder of this name.
export function chipIcon(name: string, dir: boolean): ChipIcon {
  if (dir) return 'folder'
  if (CODE_NAMES.has(name)) return 'code'
  const dot = name.lastIndexOf('.')
  const ext = dot > 0 ? name.slice(dot + 1).toLowerCase() : name.startsWith('.') ? 'ini' : ''
  if (IMAGE.has(ext)) return 'image'
  if (TEXT.has(ext)) return 'text'
  if (WEB.has(ext)) return 'web'
  if (CODE.has(ext)) return 'code'
  return 'file'
}
