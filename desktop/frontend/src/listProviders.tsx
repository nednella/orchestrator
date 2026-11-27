import { Icon } from './components/Icon'
import type { ListOption } from './listOptions'

export const GITHUB_HOST = 'github.com'

// The one provider the app reads its issues and pull requests from.
export const PROVIDERS: ListOption[] = [{ value: 'github', label: 'GitHub', icon: <Icon name="mark-github" size={14} /> }]
