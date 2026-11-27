import { providerColor } from '../usage'
import { ClaudeMark } from './ClaudeMark'

type ProviderMarkProps = { provider: string }

const SIZE = 20

// ProviderMark is a provider's icon: Claude's mark, and for any other a dot in its colour.
export function ProviderMark({ provider }: ProviderMarkProps) {
  if (provider === 'claude') return <ClaudeMark size={SIZE} />
  return <span className="inline-block flex-none rounded-full" style={{ width: SIZE * 0.6, height: SIZE * 0.6, margin: SIZE * 0.2, backgroundColor: providerColor(provider) }} aria-hidden="true" />
}
