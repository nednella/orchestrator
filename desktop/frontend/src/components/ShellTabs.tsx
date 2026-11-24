import { useCallback, useEffect, useRef, useState } from 'react'
import { useAgentos, useAgentosSelector } from '../AgentosContext'
import { useLayout } from '../LayoutContext'
import { Icon } from './Icon'
import { ShellTab, tabRowButton } from './ShellTab'

export function ShellTabs() {
  const { shellIds, shellId, selectShell, newShell, closeShell, report } = useAgentos('shellIds', 'shellId', 'selectShell', 'newShell', 'closeShell', 'report')
  const projectName = useAgentosSelector((a) => a.project?.name ?? '')
  const { mode, togglePanel, focusPanel } = useLayout()
  const scroller = useRef<HTMLDivElement>(null)
  const [view, setView] = useState({ start: 0, end: 1 })
  const [projectShell = '', ...shells] = shellIds

  const measure = useCallback(() => {
    const { scrollLeft, clientWidth, scrollWidth } = scroller.current!
    setView({
      start: scrollLeft > 1 ? scrollLeft / scrollWidth : 0,
      end: scrollLeft + clientWidth < scrollWidth - 1 ? (scrollLeft + clientWidth) / scrollWidth : 1,
    })
  }, [])

  useEffect(() => {
    const el = scroller.current!
    const onWheel = (e: WheelEvent) => {
      if (Math.abs(e.deltaY) <= Math.abs(e.deltaX)) return
      e.preventDefault()
      el.scrollLeft += e.deltaY
    }
    const resize = new ResizeObserver(measure)
    resize.observe(el)
    el.addEventListener('wheel', onWheel, { passive: false })
    return () => {
      resize.disconnect()
      el.removeEventListener('wheel', onWheel)
    }
  }, [measure])

  useEffect(() => {
    scroller.current!.querySelector('[data-selected="true"]')?.scrollIntoView({ block: 'nearest', inline: 'nearest' })
    measure()
  }, [shellIds, shellId, measure])

  const pick = (id: string) => {
    selectShell(id)
    focusPanel('shell')
  }
  const closable = mode !== 'narrow'
  const close = () => togglePanel('shell')

  return (
    <div
      className="shell-tabs group/tabs flex h-10 flex-none items-stretch border-b border-line text-small"
      style={{ paddingRight: '0.5rem' }}
      onClick={(e) => {
        if (closable && e.target === e.currentTarget) close()
      }}
    >
      <div role="tablist" aria-label="Shells" className="flex min-w-0 items-stretch">
        <ShellTab label={projectName} pinned selected={shellId === projectShell} onSelect={() => pick(projectShell)} />
        <div className="relative flex min-w-0">
          <div
            ref={scroller}
            className="shell-tabs-scroll flex min-w-0 items-stretch overflow-x-auto"
            data-hidden-left={view.start > 0}
            data-hidden-right={view.end < 1}
            onScroll={measure}
          >
            {shells.map((id, i) => (
              <ShellTab
                key={id}
                label={`Shell ${i + 2}`}
                selected={id === shellId}
                onSelect={() => pick(id)}
                onClose={() => report(() => closeShell(id))}
              />
            ))}
          </div>
          {(view.start > 0 || view.end < 1) && (
            <span
              className="pointer-events-none absolute -bottom-px h-px bg-dim transition-colors group-hover/tabs:bg-soft"
              style={{ left: `${view.start * 100}%`, width: `${(view.end - view.start) * 100}%` }}
            />
          )}
        </div>
      </div>
      <button
        className={`${tabRowButton} ml-1.5 self-center`}
        title="New shell"
        aria-label="New shell"
        onClick={() =>
          report(async () => {
            await newShell()
            focusPanel('shell')
          })
        }
      >
        <Icon name="add" size={14} />
      </button>
      {closable && (
        <button className={`${tabRowButton} ml-auto self-center`} title="Close the shell (⌘⇧S)" aria-label="Close the shell" onClick={close}>
          <Icon name="chevron-down" size={14} />
        </button>
      )}
    </div>
  )
}
