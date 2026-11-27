import { providerColor } from '../usage'
import { ClaudeMark } from './ClaudeMark'

type ProviderMarkProps = { provider: string; size?: number }

// ProviderMark is a provider's icon: Claude's mark, and for any other a dot in its colour.
export function ProviderMark({ provider, size = 20 }: ProviderMarkProps) {
  if (provider === 'claude') return <ClaudeMark size={size} />
  return <span className="inline-block flex-none rounded-full" style={{ width: size * 0.6, height: size * 0.6, margin: size * 0.2, backgroundColor: providerColor(provider) }} aria-hidden="true" />
}
