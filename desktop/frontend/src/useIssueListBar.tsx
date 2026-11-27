import { CalendarArrowDown, Clock, Layers, PenLine, Users, UserCheck, Tag, User } from 'lucide-react'
import type { ListBarProps } from './components/ListBar'
import { listIssues, issueFacets, issueFilterCount, DEFAULT_ISSUE_PREFS, fixIssuePrefs, normalIssuePrefs } from './issueList'
import type { IssuePrefs } from './issueList'
import { isFiltering } from './issueFilter'
import { useLayout } from './LayoutContext'
import { facetOptions } from './listOptions'
import { PROVIDERS } from './listProviders'
import { useOrchestrator } from './OrchestratorContext'
import type { Issue } from './types'
import { useProjectStored } from './useProjectStored'
import { useSavedViews } from './useSavedViews'
import { useViewer } from './useViewer'

const NO_LOGIN = 'Your GitHub login could not be read.'

// useIssueListBar is the issue list's bar, and the issues it leaves in the list.
export function useIssueListBar(issues: Issue[]): { bar: ListBarProps; shown: Issue[]; filtering: boolean } {
  const { issueFilter, setIssueFilter, issuesLoading, refreshIssues, focus, report } = useOrchestrator('issueFilter', 'setIssueFilter', 'issuesLoading', 'refreshIssues', 'focus', 'report')
  const { panelExpanded } = useLayout()
  const [stored, setStored] = useProjectStored('issues.prefs', DEFAULT_ISSUE_PREFS)
  const prefs: IssuePrefs = { ...fixIssuePrefs(stored), q: issueFilter }
  const viewer = useViewer()

  const change = (patch: Partial<IssuePrefs>) => {
    if (patch.q !== undefined) setIssueFilter(patch.q)
    setStored({ ...prefs, ...patch })
  }
  const facets = issueFacets(issues)
  const unavailable = viewer === null ? NO_LOGIN : undefined

  const views = useSavedViews({
    kind: 'issue',
    storeAs: 'issues.views',
    current: prefs,
    fix: fixIssuePrefs,
    normal: normalIssuePrefs,
    apply: (p) => {
      setIssueFilter(p.q)
      setStored(p)
    },
    suggested: prefs.involvement === 'assigned' ? 'Issues Assigned to Me' : '',
  })

  const shown = listIssues(issues, prefs, viewer ?? '')
  const bar: ListBarProps = {
    noun: 'issues',
    search: {
      value: issueFilter,
      label: 'Search issues',
      placeholder: 'Search issues',
      title: 'Search by text, @author, assignee:, label:, type:, section: and is:running; a leading - leaves matches out',
      focusOn: 'issue-filter',
      onChange: (q) => change({ q }),
      onLeave: () => focus(panelExpanded ? 'right' : 'thread'),
    },
    sort: {
      value: prefs.sort,
      options: [
        { value: 'updated', label: 'Recently updated', icon: <Clock size={14} /> },
        { value: 'created', label: 'Newest', icon: <CalendarArrowDown size={14} /> },
      ],
      onChange: (sort) => change({ sort: sort as IssuePrefs['sort'] }),
    },
    filters: {
      count: issueFilterCount(prefs),
      rows: [
        {
          kind: 'radio',
          id: 'involvement',
          label: 'Involvement',
          icon: <Users size={14} />,
          value: prefs.involvement,
          options: [
            { value: 'all', label: 'Everyone', icon: <Users size={14} /> },
            { value: 'assigned', label: 'Assigned to me', icon: <UserCheck size={14} />, unavailable },
            { value: 'opened', label: 'Opened by me', icon: <PenLine size={14} />, unavailable },
          ],
          onChange: (involvement) => change({ involvement: involvement as IssuePrefs['involvement'] }),
        },
        {
          kind: 'radio',
          id: 'type',
          label: 'Type',
          icon: <Layers size={14} />,
          value: prefs.type,
          options: [{ value: '', label: 'Any type', icon: <Layers size={14} /> }, ...facetOptions(facets.types, prefs.type ? [prefs.type] : [])],
          onChange: (type) => change({ type }),
        },
        {
          kind: 'radio',
          id: 'author',
          label: 'Author',
          icon: <User size={14} />,
          value: prefs.author,
          searchLabel: 'Search authors',
          emptyText: 'No authors found',
          options: [{ value: '', label: 'Anyone', icon: <Layers size={14} /> }, ...facetOptions(facets.authors, prefs.author ? [prefs.author] : [])],
          onChange: (author) => change({ author }),
        },
        { kind: 'separator', id: 'rule' },
        {
          kind: 'check',
          id: 'labels',
          label: 'Labels',
          icon: <Tag size={14} />,
          value: prefs.labels,
          searchLabel: 'Search labels',
          emptyText: 'No labels found',
          options: facetOptions(facets.labels, prefs.labels),
          onChange: (labels) => change({ labels }),
        },
        {
          kind: 'radio',
          id: 'assignee',
          label: 'Assignee',
          icon: <User size={14} />,
          value: prefs.assignee,
          searchLabel: 'Search people',
          emptyText: 'No people found',
          options: [{ value: '', label: 'Anyone', icon: <Layers size={14} /> }, ...facetOptions(facets.assignees, prefs.assignee ? [prefs.assignee] : [])],
          onChange: (assignee) => change({ assignee }),
        },
      ],
    },
    views,
    providers: PROVIDERS,
    host: prefs.host,
    onHost: (host) => change({ host }),
    loading: issuesLoading,
    onRefresh: () => report(refreshIssues),
  }
  return { bar, shown, filtering: isFiltering(issueFilter) || issueFilterCount(prefs) > 0 }
}
