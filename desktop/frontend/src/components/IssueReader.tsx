import { useEffect, useRef, useState } from 'react'
import { api, errorMessage } from '../api'
import { useAgentos } from '../AgentosContext'
import { isTyping } from '../useListNav'
import { useMinimumLoading } from '../useMinimumLoading'
import type { IssueDetail } from '../types'
import { Ago } from './Ago'
import { Icon } from './Icon'
import { IssueBody } from './IssueBody'
import { IssueThread } from './IssueThread'
import { IssueWorkflow } from './IssueWorkflow'
import { Loading } from './Loading'
import { TypeMark } from './TypeMark'

const dateOf = (at: number) => new Date(at).toLocaleString()

type IssueReaderProps = { number: number; onBack(): void }

export function IssueReader({ number, onBack }: IssueReaderProps) {
  const { issues, projectSessions: sessions, report } = useAgentos('issues', 'projectSessions', 'report')
  const body = useRef<HTMLDivElement>(null)
  const [detail, setDetail] = useState<IssueDetail | null>(null)
  const [error, setError] = useState('')
  const loading = useMinimumLoading(!detail && !error)
  const issue = issues.find((i) => i.number === number)
  const session = sessions.find((s) => s.id === issue?.sessionId)

  useEffect(() => {
    api.issueDetail(number).then(setDetail, (err) => setError(errorMessage(err)))
  }, [number])

  useEffect(() => body.current?.focus(), [])

  return (
    <div
      className="flex h-full min-h-0 flex-col"
      onKeyDown={(e) => {
        if (e.key !== 'Escape' || isTyping(e.target)) return
        e.preventDefault()
        e.stopPropagation()
        onBack()
      }}
    >
      <header className="flex flex-none flex-col gap-2 border-b border-line px-3 pt-2 pb-3">
        <div className="flex items-center">
          <button className="btn btn-ghost -ml-1.5 h-6 gap-1 px-1.5" title="Back to the issues (Esc)" onClick={onBack}>
            <Icon name="chevron-left" size={12} />
            Issues
          </button>
          {issue && (
            <button className="btn btn-ghost ml-auto h-6 w-6 justify-center px-0" title="Open on GitHub" aria-label="Open on GitHub" onClick={() => report(() => api.openURL(issue.url))}>
              <Icon name="external" size={12} />
            </button>
          )}
        </div>
        <h2 className="text-title font-semibold break-words">
          <span className="mono mr-1.5 text-soft">#{number}</span>
          {issue?.title ?? 'No longer in the queue'}
        </h2>
        {issue && (
          <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-small text-dim">
            {issue.type && <TypeMark type={issue.type} named />}
            <span title={dateOf(issue.createdAt)}>
              {issue.author ? `${issue.author} opened this` : 'Opened'} <Ago at={issue.createdAt} />
            </span>
            {issue.labels.map((label) => (
              <span key={label} className="mono rounded-sm border border-line-strong px-1.5 text-label text-soft">
                {label}
              </span>
            ))}
          </div>
        )}
        {issue && (
          <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5 pt-0.5 text-small">
            {session ? <IssueThread session={session} /> : <IssueWorkflow issue={issue} />}
          </div>
        )}
      </header>
      <div ref={body} tabIndex={-1} className="min-h-0 flex-1 overflow-y-auto px-3 py-3 outline-none">
        {loading ? (
          <Loading />
        ) : (
          <div className="flex flex-col gap-6">
            {error && <p className="text-small text-danger">{error}</p>}
            {detail && (detail.bodyHTML ? <IssueBody html={detail.bodyHTML} /> : <p className="text-small text-dim">No description.</p>)}
            {detail && detail.comments.length > 0 && (
              <section className="flex flex-col gap-4 border-t border-line pt-4">
                <h3 className="label">{detail.comments.length === 1 ? '1 comment' : `${detail.comments.length} comments`}</h3>
                {detail.comments.map((comment) => (
                  <article key={comment.createdAt} className="flex flex-col gap-1.5">
                    <div className="flex items-baseline gap-2 text-small">
                      <span className="font-medium">{comment.author}</span>
                      <span className="text-dim" title={dateOf(comment.createdAt)}>
                        <Ago at={comment.createdAt} />
                      </span>
                    </div>
                    <IssueBody html={comment.bodyHTML} />
                  </article>
                ))}
              </section>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
