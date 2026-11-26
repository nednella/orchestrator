import { Check, Download, RefreshCw, RotateCw } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useAgentos } from '../AgentosContext'
import { api } from '../api'
import { askConfirm } from '../confirm'
import { installConfirm, updateAction, updateLook } from '../updateButton'
import { useDelayedTip } from '../useDelayedTip'
import { FloatingTip } from './FloatingTip'

const ICONS = { 'refresh-cw': RefreshCw, download: Download, 'rotate-cw': RotateCw }
const RING_R = 14
const RING_LENGTH = 2 * Math.PI * RING_R

export function UpdateButton() {
  const { update, report } = useAgentos('update', 'report')
  const look = updateLook(update)
  const action = updateAction(update)
  const { anchor, tipHandlers } = useDelayedTip()
  // The spin plays whole turns: it starts with the check and stops at the end of the turn the check ends in.
  const [turning, setTurning] = useState(look.spin)
  const spinning = useRef(look.spin)
  spinning.current = look.spin
  useEffect(() => {
    if (look.spin) setTurning(true)
  }, [look.spin])

  const inPlay = update.status === 'available' || update.status === 'downloading' || update.status === 'downloaded' || update.status === 'installed'
  // A turn that runs on keeps the check's icon, so only it spins.
  const Icon = turning ? RefreshCw : ICONS[look.icon]
  const tone = look.success ? 'text-[var(--finished)]' : inPlay ? 'text-ink' : 'text-[var(--sidebar-icon)] hover:text-ink'

  const onClick = () =>
    report(async () => {
      if (action === 'check') await api.checkUpdate()
      if (action === 'download') await api.downloadUpdate()
      if (action === 'restart') await api.installUpdate()
      if (action === 'install' && (await askConfirm(installConfirm(update.version))).ok) await api.installUpdate()
    })

  return (
    <button
      className={`relative ml-auto inline-flex h-[32px] w-[32px] flex-none items-center justify-center rounded-full transition-colors duration-[120ms] hover:bg-[var(--bg-row-hover)] aria-disabled:cursor-default ${tone} ${inPlay ? 'bg-[var(--bg-update)]' : ''}`}
      aria-label={look.tooltip}
      aria-disabled={action === null}
      onClick={() => action !== null && onClick()}
      {...tipHandlers}
    >
      {look.ring !== null && (
        <svg className="pointer-events-none absolute inset-0 -rotate-90" viewBox="0 0 32 32" aria-hidden="true">
          <circle cx="16" cy="16" r={RING_R} fill="none" strokeWidth="1.5" style={{ stroke: 'var(--update-ring-track)' }} />
          <circle
            cx="16"
            cy="16"
            r={RING_R}
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            strokeLinecap="round"
            strokeDasharray={RING_LENGTH}
            strokeDashoffset={RING_LENGTH * (1 - look.ring / 100)}
            className="transition-[stroke-dashoffset] duration-300 ease-out"
          />
        </svg>
      )}
      <Icon
        size={16}
        aria-hidden="true"
        className={turning ? 'animate-spin' : undefined}
        onAnimationIteration={() => {
          if (!spinning.current) setTurning(false)
        }}
      />
      {look.dot && !turning && (
        <span
          className="absolute top-[6px] right-[6px] h-[6px] w-[6px] rounded-full"
          style={{ background: 'var(--finished)', boxShadow: '0 0 0 2px var(--bg-update)' }}
          aria-hidden="true"
        />
      )}
      {look.badge && !turning && (
        <span
          className="absolute right-[4px] bottom-[4px] flex h-[10px] w-[10px] items-center justify-center rounded-full"
          style={{ background: 'var(--text)', color: 'var(--bg-app)', boxShadow: '0 0 0 2px var(--bg-update)' }}
          aria-hidden="true"
        >
          <Check size={8} strokeWidth={3} />
        </span>
      )}
      {anchor && <FloatingTip anchor={anchor} text={look.tooltip} />}
    </button>
  )
}
