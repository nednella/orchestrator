import { LoaderCircle } from 'lucide-react'

type SpinnerProps = {
  size: 12 | 14 | 16 | 20
  muted?: boolean
}

export function Spinner({ size, muted }: SpinnerProps) {
  return <LoaderCircle size={size} role="status" aria-label="Loading" className={`flex-none animate-spin ${muted ? 'text-dim' : 'text-soft'}`} />
}
