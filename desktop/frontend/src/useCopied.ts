import { useEffect, useRef, useState } from 'react'
import { useOrchestrator } from './OrchestratorContext'

// useCopied copies text to the clipboard and says "copied" for ms, only when the copy worked.
export function useCopied(text: string, ms = 1500): [boolean, () => void] {
  const { report } = useOrchestrator('report')
  const [copied, setCopied] = useState(false)

  const timer = useRef<ReturnType<typeof setTimeout>>()
  useEffect(() => () => clearTimeout(timer.current), [])

  const copy = () => {
    report(async () => {
      await navigator.clipboard.writeText(text)
      setCopied(true)
      clearTimeout(timer.current)
      timer.current = setTimeout(() => setCopied(false), ms)
    })
  }

  return [copied, copy]
}
