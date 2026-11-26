import { useCallback, useEffect, useRef, useState } from 'react'
import { api } from '../api'
import { useAgentos, useAgentosSelector } from '../AgentosContext'
import type { PanelViewProps } from '../panelViews'
import { pullRows, pullsShown } from '../pullRows'
import type { PullsRead } from '../pullRows'
import type { RepoPull } from '../types'
import { Ago } from './Ago'
import { Icon } from './Icon'
import { Loading } from './Loading'
import { Notice } from './Notice'
import { PRTab } from './PRTab'

type PullRowProps = { pull: RepoPull; threadTitle: string | undefined; onOpen(): void }

function PullRow({ pull, threadTitle, onOpen }: PullRowProps) {
  const meta = [pull.draft ? 'draft' : '', pull.branch, threadTitle ?? 'no thread'].filter(Boolean).join(' · ')
  return (
    <button className="flex w-full min-w-0 items-start gap-2 px-4 py-2 text-left hover:bg-hover" onClick={onOpen}>
      <span className="mt-0.5 flex-none" style={{ color: `var(--pr-${pull.draft ? 'draft' : 'open'})` }}>
        <Icon name={pull.draft ? 'git-pull-request-draft' : 'git-pull-request'} size={14} />
      </span>
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-body">
          <span className="mono text-soft">#{pull.number}</span> {pull.title}
        </span>
        <span className="truncate text-label text-dim">{meta}</span>
      </span>
    </button>
  )
}

function BackButton({ onBack }: { onBack(): void }) {
  return (
    <button className="btn btn-ghost -ml-1.5 h-6 w-fit gap-1 px-1.5" title="Back to the pull requests" onClick={onBack}>
      <Icon name="chevron-left" size={12} />
      Pull requests
    </button>
  )
}

type PullDetailProps = { pull: RepoPull; onBack(): void }

function PullDetail({ pull, onBack }: PullDetailProps) {
  const { report } = useAgentos('report')
  return (
    <div className="flex h-full min-h-0 flex-col overflow-y-auto">
      <header className="flex flex-none flex-col gap-2 border-b border-line px-3 pt-2 pb-3">
        <BackButton onBack={onBack} />
        <h2 className="text-title font-semibold break-words">
          <span className="mono mr-1.5 text-soft">#{pull.number}</span>
          {pull.title}
        </h2>
        <p className="text-label text-dim">
          {pull.draft ? 'Draft · ' : ''}
          {pull.branch} · {pull.author} · updated <Ago at={pull.updatedAt} />
        </p>
      </header>
      <div className="px-4 py-3">
        <button className="btn" onClick={() => report(() => api.openURL(pull.url))}>
          <Icon name="external" size={12} />
          Open on GitHub
        </button>
      </div>
    </div>
  )
}

// The last list read of each project, so the view shows it at once when it opens again.
const known = new Map<string, RepoPull[]>()

// PullRequestsView is the open thread's pull request and the repository's other open ones.
export function PullRequestsView({ session }: PanelViewProps) {
  const { projectSessions } = useAgentos('projectSessions')
  const project = useAgentosSelector((a) => a.project?.name)
  const repo = useAgentosSelector((a) => a.project?.repo ?? '')
  const key = project ?? ''
  const [found, setFound] = useState<PullsRead | null>(null)
  const [failed, setFailed] = useState<string | null>(null)
  const [reading, setReading] = useState(false)
  const [opened, setOpened] = useState<{ project: string; number: number } | null>(null)
  const current = useRef(key)
  current.current = key
  const detail = opened?.project === key ? opened.number : null
  const setDetail = (number: number | null) => setOpened(number === null ? null : { project: key, number })

  const read = useCallback(() => {
    setReading(true)
    api.pullRequests().then(
      (got) => {
        if (current.current !== key) return
        known.set(key, got.pulls)
        setFound({ project: key, pulls: got.pulls, error: got.error })
        setFailed(null)
      },
      () => current.current === key && setFailed('Could not read the pull requests.'),
    ).finally(() => {
      if (current.current === key) setReading(false)
    })
  }, [key])
  useEffect(() => {
    setFailed(null)
    if (project !== undefined && repo) read()
  }, [project, repo, read])
  const shown = pullsShown(found, key, known)
  const pulls = shown.pulls
  const error = failed ?? shown.error

  if (project !== undefined && !repo) return <Notice title="No repo linked" hint={`${project} has no GitHub repo, so it has no pull requests.`} centered />

  const open = pulls?.find((p) => p.number === detail)
  if (open) {
    const thread = projectSessions.find((s) => s.id === open.thread)
    const back = () => setDetail(null)
    if (!thread) return <PullDetail pull={open} onBack={back} />
    return (
      <div className="flex h-full min-h-0 flex-col">
        <header className="flex-none border-b border-line px-3 py-2">
          <BackButton onBack={back} />
        </header>
        <div className="min-h-0 flex-1">
          <PRTab key={thread.id} session={thread} />
        </div>
      </div>
    )
  }

  const { mine, rest } = pullRows(pulls ?? [], session?.pr ? session : null)
  const titleOf = (p: RepoPull) => projectSessions.find((s) => s.id === p.thread)?.title

  return (
    <div className="flex h-full min-h-0 flex-col overflow-y-auto">
      {session?.pr && (
        <section aria-label="This thread" className="flex-none border-b border-line">
          <h3 className="px-4 pt-3 text-label font-medium text-dim">This thread</h3>
          <PRTab key={session.id} session={session} />
        </section>
      )}
      <section aria-label="Open" className="flex min-h-0 flex-col pb-2">
        <div className="flex items-center px-4 pt-3 pb-1">
          <h3 className="text-label font-medium text-dim">Open</h3>
          <button className="btn btn-ghost ml-auto h-6 w-6 justify-center px-0" title="Read the pull requests again" aria-label="Refresh" disabled={reading} onClick={read}>
            <span className={`flex ${reading ? 'animate-spin' : ''}`}>
              <Icon name="refresh" size={12} />
            </span>
          </button>
        </div>
        {error && (
          <p role="alert" className="px-4 pb-1 text-small text-danger">
            {error}
          </p>
        )}
        {pulls === null ? (
          error ? null : <Loading />
        ) : rest.length === 0 && !mine ? (
          <Notice title="No open pull requests" hint={`${repo || 'This project'} has none right now.`} />
        ) : (
          rest.map((p) => <PullRow key={p.number} pull={p} threadTitle={titleOf(p)} onOpen={() => setDetail(p.number)} />)
        )}
      </section>
    </div>
  )
}
