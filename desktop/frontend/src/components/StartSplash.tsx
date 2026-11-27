import { useEffect, useState } from 'react'

const SHOWN_MS = 1600

export function StartSplash() {
  const [shown, setShown] = useState(true)

  useEffect(() => {
    const timer = setTimeout(() => setShown(false), SHOWN_MS)
    return () => clearTimeout(timer)
  }, [])

  if (!shown) return null
  return (
    <div className="start-splash" aria-hidden="true">
      <span className="start-splash-mark">
        orchestrator<span className="start-splash-dot">.</span>
      </span>
    </div>
  )
}
