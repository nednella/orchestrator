import { GITHUB_HOST } from '../listProviders'
import { Icon } from './Icon'

// ProviderLine names where the list's issues come from, and how many projects.
export function ProviderLine() {
  return (
    <h2 className="flex items-center gap-1.5 px-4 pt-1 pb-0.5 text-label font-medium text-dim">
      <Icon name="mark-github" size={14} />
      GitHub
      <span className="font-normal">· {GITHUB_HOST}</span>
      <span className="font-normal">· 1 project</span>
    </h2>
  )
}
