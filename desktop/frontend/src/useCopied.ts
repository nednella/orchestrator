import { useEffect, useRef, useState } from 'react'
import { useAgentos } from './AgentosContext'

// useCopied copies text to the clipboard and says "copied" for a moment, only when the copy worked.
export function useCopied(text: string): [boolean, () => void] {
  const { report } = useAgentos('report')
  const [copied, setCopied] = useState(false)

  const timer = useRef<ReturnType<typeof setTimeout>>()
  useEffect(() => () => clearTimeout(timer.current), [])

  const copy = () => {
    report(async () => {
      await navigator.clipboard.writeText(text)
      setCopied(true)
      clearTimeout(timer.current)
      timer.current = setTimeout(() => setCopied(false), 1500)
    })
  }

  return [copied, copy]
}
