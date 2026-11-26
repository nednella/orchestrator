import { useCallback, useEffect, useRef, useState } from 'react'
import { api, on } from './api'
import { useOrchestrator } from './OrchestratorContext'
import type { Ledger, Limits } from './types'

// useUsage gives the plan limits and today's totals. It shows the last answer at once, then reads again; refresh is the
// owner's ask, which Go answers with a probe when no thread is live.
export function useUsage() {
  const { report } = useOrchestrator('report')
  const [limits, setLimits] = useState<Limits | null>(null)
  const [today, setToday] = useState<Ledger['totals'] | null>(null)
  const [reading, setReading] = useState(false)
  const [failure, setFailure] = useState('')
  const live = useRef(true)
  const reportRef = useRef(report)
  reportRef.current = report

  const refresh = useCallback(async () => {
    setReading(true)
    try {
      const next = await api.refreshLimits()
      if (!live.current) return
      setLimits(next)
      setFailure('')
    } catch (e) {
      if (live.current) setFailure(String(e))
    } finally {
      if (live.current) setReading(false)
    }
  }, [])

  useEffect(() => {
    live.current = true
    reportRef.current(async () => {
      const [known, ledger] = await Promise.all([api.limits(), api.ledger(1, '')])
      if (!live.current) return
      setLimits((current) => current ?? known)
      setToday(ledger.totals)
    })
    void refresh()
    const off = on('limits', (next) => {
      setLimits(next)
      if (!next.error) setFailure('')
    })
    return () => {
      live.current = false
      off()
    }
  }, [refresh])

  return { limits, today, reading, error: limits?.error || failure, refresh }
}
