import { useState } from 'react'
import { useOrchestratorSelector } from './OrchestratorContext'
import { readStored, writeStored } from './storage'

export function useProjectStored<T>(name: string, fallback: T): [T, (value: T) => void] {
  const projectName = useOrchestratorSelector((a) => a.project?.name ?? '')
  const key = `orchestrator.${name}.${projectName}`
  const [stored, setStored] = useState(() => ({ key, value: readStored(key, fallback) }))

  let current = stored
  if (stored.key !== key) {
    current = { key, value: readStored(key, fallback) }
    setStored(current)
  }

  const store = (value: T) => {
    writeStored(key, value)
    setStored({ key, value })
  }

  return [current.value, store]
}
