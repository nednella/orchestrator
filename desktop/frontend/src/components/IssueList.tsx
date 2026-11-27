import { useCallback, useMemo, useRef } from 'react'
import type { KeyboardEvent } from 'react'
import { useOrchestrator } from '../OrchestratorContext'
import type { Issue } from '../types'
import { isTyping } from '../useListNav'
import { useProjectStored } from '../useProjectStored'
import { useDragEdgeScroll } from '../useDragEdgeScroll'
import { useIssueDrag } from '../useIssueDrag'
import { Collapse } from './Collapse'
import { IssueRow } from './IssueRow'
import { SkeletonRows } from './SkeletonRows'
import { Notice } from './Notice'
import { IssueSection } from './IssueSection'
import { IssueUnsortedHeader } from './IssueUnsortedHeader'
import { SectionHeader } from './SectionHeader'

const STEPS: Record<string, number> = { ArrowUp: -1, w: -1, ArrowDown: 1, s: 1 }

// The focus moves from row to row, so the row that holds it shows its buttons and Tab reaches them.
function moveFocus(list: HTMLElement, step: number) {
  const stops = [...list.querySelectorAll<HTMLElement>('.section-head, [data-nav]')].filter((el) => el.offsetParent !== null && !el.closest('[inert]'))
  const active = document.activeElement
  const current = active?.closest('[data-issue]')?.querySelector<HTMLElement>('[data-nav]') ?? active
  const at = stops.indexOf(current as HTMLElement)
  stops[at < 0 ? 0 : Math.min(Math.max(at + step, 0), stops.length - 1)]?.focus()
}

type IssueListProps = { shown: Issue[]; filtering: boolean; onRead(number: number): void }

export function IssueList({ shown: coreShown, filtering, onRead }: IssueListProps) {
  const { issues: coreIssues, issuesLoading } = useOrchestrator('issues', 'issuesLoading')
  const drag = useIssueDrag(coreIssues)
  const issues = drag.place(coreIssues)
  const shown = drag.place(coreShown)
  const [closed, setClosed] = useProjectStored<string[]>('queue.closed', [])
  const [unsortedOpen, setUnsortedOpen] = useProjectStored('queue.unsortedOpen', false)
  const list = useRef<HTMLDivElement>(null)

  // The core lists issues section by section and the issues no section matches last, in a nameless group;
  // a project with no sections has only that group, shown as a plain list.
  const sorted = issues.some((issue) => issue.section)
  const isClosed = useCallback((name: string) => (name ? closed.includes(name) : sorted && !unsortedOpen), [closed, sorted, unsortedOpen])

  const sections = useMemo(() => {
    const groups: { name: string; issues: Issue[]; open: boolean }[] = []
    for (const issue of shown) {
      const group = groups.find((g) => g.name === issue.section)
      if (group) group.issues.push(issue)
      else groups.push({ name: issue.section, issues: [issue], open: filtering || !isClosed(issue.section) })
    }
    return groups
  }, [shown, filtering, isClosed])

  const toggle = (name: string, open: boolean) => {
    if (!name) {
      setUnsortedOpen(open)
      return
    }
    setClosed(open ? closed.filter((n) => n !== name) : [...closed, name])
  }

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    const step = STEPS[e.key]
    if (!step || e.metaKey || e.ctrlKey || e.altKey || isTyping(e.target)) return
    e.preventDefault()
    moveFocus(e.currentTarget, step)
  }

  useDragEdgeScroll(list, drag.dragging !== null)
  const loading = issuesLoading && issues.length === 0

  const body = () => {
    if (loading) return <SkeletonRows />
    if (issues.length === 0) return <Notice title="No open issues" hint="This repo has no open issues." centered />
    if (filtering && shown.length === 0) return <Notice title="No issues match" hint="Clear the filter to see all of them." centered />
    return sections.map((section) => (
      <IssueSection key={section.name} name={section.name} className="issue-section" drag={drag}>
        {section.name ? (
          <SectionHeader name={section.name} count={section.issues.length} open={section.open} locked={filtering} onToggle={() => toggle(section.name, isClosed(section.name))} />
        ) : (
          sorted && <IssueUnsortedHeader count={section.issues.length} open={section.open} locked={filtering} onToggle={() => toggle('', isClosed(''))} />
        )}
        <Collapse open={section.open}>
          <div className="flex flex-col gap-0.5 pb-1">
            {section.issues.map((issue) => (
              <IssueRow key={issue.number} issue={issue} drag={drag} onRead={() => onRead(issue.number)} />
            ))}
          </div>
        </Collapse>
      </IssueSection>
    ))
  }

  return (
    <div ref={list} className="min-h-0 flex-1 overflow-y-auto pb-2" onKeyDown={onKeyDown}>
      {body()}
    </div>
  )
}
