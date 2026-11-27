import { CalendarArrowDown, CalendarArrowUp, Clock, Eye, EyeOff, GitPullRequestDraft, Layers, PenLine, Tag, User } from 'lucide-react'
import type { ListBarProps } from './components/ListBar'
import { useLayout } from './LayoutContext'
import { facetOptions } from './listOptions'
import { PROVIDERS } from './listProviders'
import { useOrchestrator } from './OrchestratorContext'
import { DEFAULT_PULL_PREFS, fixPullPrefs, listPulls, normalPullPrefs, pullFacets, pullFilterCount } from './pullList'
import type { PullPrefs } from './pullList'
import type { RepoPull } from './types'
import { useProjectStored } from './useProjectStored'
import { useSavedViews } from './useSavedViews'
import { useViewer } from './useViewer'

const NO_LOGIN = 'Your GitHub login could not be read.'

type PullListBarOptions = { pulls: RepoPull[]; loading: boolean; onRefresh(): void }

// usePullListBar is the pull request list's bar, and the pull requests it leaves in the list.
export function usePullListBar({ pulls, loading, onRefresh }: PullListBarOptions): { bar: ListBarProps; shown: RepoPull[]; filtering: boolean } {
  const { focus } = useOrchestrator('focus')
  const { panelExpanded } = useLayout()
  const [stored, setStored] = useProjectStored('pulls.prefs', DEFAULT_PULL_PREFS)
  const prefs = fixPullPrefs(stored)
  const viewer = useViewer()
  const unavailable = viewer === null ? NO_LOGIN : undefined
  const facets = pullFacets(pulls)

  const change = (patch: Partial<PullPrefs>) => setStored({ ...prefs, ...patch })
  const views = useSavedViews({
    kind: 'PR',
    storeAs: 'pulls.views',
    current: prefs,
    fix: fixPullPrefs,
    normal: normalPullPrefs,
    apply: setStored,
    suggested: prefs.involvement === 'reviewing' ? 'PRs Requesting My Review' : '',
  })

  const bar: ListBarProps = {
    noun: 'pull requests',
    search: {
      value: prefs.q,
      label: 'Search pull requests',
      placeholder: 'Search pull requests, or label:bug',
      onChange: (q) => change({ q }),
      onLeave: () => focus(panelExpanded ? 'right' : 'thread'),
    },
    sort: {
      value: prefs.sort,
      options: [
        { value: 'updated', label: 'Recently updated', icon: <Clock size={14} /> },
        { value: 'created', label: 'Newest', icon: <CalendarArrowDown size={14} /> },
        { value: 'oldest', label: 'Oldest', icon: <CalendarArrowUp size={14} /> },
      ],
      onChange: (sort) => change({ sort: sort as PullPrefs['sort'] }),
    },
    filters: {
      count: pullFilterCount(prefs),
      rows: [
        {
          kind: 'radio',
          id: 'involvement',
          label: 'Involvement',
          icon: <Layers size={14} />,
          value: prefs.involvement,
          options: [
            { value: 'all', label: 'All', icon: <Layers size={14} /> },
            { value: 'reviewing', label: 'Reviewing', icon: <Eye size={14} />, unavailable },
            { value: 'authored', label: 'Authored', icon: <PenLine size={14} />, unavailable },
          ],
          onChange: (involvement) => change({ involvement: involvement as PullPrefs['involvement'] }),
        },
        { kind: 'separator', id: 'rule' },
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
          id: 'draft',
          label: 'Draft',
          icon: <GitPullRequestDraft size={14} />,
          value: prefs.draft,
          options: [
            { value: 'all', label: 'All', icon: <Layers size={14} /> },
            { value: 'only', label: 'Drafts only', icon: <GitPullRequestDraft size={14} /> },
            { value: 'hide', label: 'Hide drafts', icon: <EyeOff size={14} /> },
          ],
          onChange: (draft) => change({ draft: draft as PullPrefs['draft'] }),
        },
      ],
    },
    views,
    providers: PROVIDERS,
    host: prefs.host,
    onHost: (host) => change({ host }),
    loading,
    onRefresh,
  }
  return { bar, shown: listPulls(pulls, prefs, viewer ?? ''), filtering: prefs.q.trim() !== '' || pullFilterCount(prefs) > 0 }
}
