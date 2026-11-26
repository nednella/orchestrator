import { Spinner } from './Spinner'

export function CenteredSpinner() {
  return (
    <div className="flex h-full flex-1 items-center justify-center px-4 py-16">
      <Spinner size={16} />
    </div>
  )
}
