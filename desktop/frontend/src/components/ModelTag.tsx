import { shortModel } from '../model'

type ModelTagProps = { model: string; effort: string; problem?: string; className?: string }

export function ModelTag({ model, effort, problem = '', className = '' }: ModelTagProps) {
  const text = [shortModel(model), effort].filter(Boolean).join(' · ')
  if (!text) return null
  return (
    <span className={`flex-none text-small whitespace-nowrap ${problem ? 'text-danger' : 'text-dim'} ${className}`} title={problem || 'Model and effort'}>
      {text}
    </span>
  )
}
