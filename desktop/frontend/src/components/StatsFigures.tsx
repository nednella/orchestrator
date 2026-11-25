import { Clock, Coins, GitMerge, Layers, ListChecks, MessageSquare } from 'lucide-react'
import { dollars, tokenCount } from '../spend'
import { shortDay } from '../statsChart'
import { aDay, change, counted, periodBefore, workTime } from '../statsFigures'
import type { Ledger } from '../types'
import { StatsFigure } from './StatsFigure'

type StatsFiguresProps = { ledger: Ledger }

export function StatsFigures({ ledger }: StatsFiguresProps) {
  const { totals, previous, usage } = ledger
  const tasks = usage.tasks
  const period = periodBefore(ledger.days)
  const priced = `Estimated at list prices of ${shortDay(usage.pricesAsOf)}`
  return (
    <dl aria-label="Figures" className="grid grid-cols-3 gap-2 @max-[40rem]/page:grid-cols-2">
      <StatsFigure icon={Clock} label="Worked" value={workTime(totals.workMs)} sub={`${workTime(totals.sessionMs)} open`} change={change(totals.workMs, previous.workMs)} period={period} />
      <StatsFigure
        icon={ListChecks}
        label="Tasks"
        value={totals.sessions.toLocaleString()}
        sub={`${counted(totals.issueSessions, 'issue', 'issues')} · ${counted(totals.chats, 'chat', 'chats')}`}
        change={change(totals.sessions, previous.sessions)}
        period={period}
      />
      <StatsFigure icon={MessageSquare} label="Prompts" value={totals.prompts.toLocaleString()} sub={`${aDay(totals.prompts, ledger.perDay.length)} a day`} change={change(totals.prompts, previous.prompts)} period={period} />
      <StatsFigure icon={GitMerge} label="PRs merged" value={totals.prsMerged.toLocaleString()} sub={`${counted(totals.issuesClosed, 'issue', 'issues')} closed`} />
      <StatsFigure icon={Layers} label="Tokens" value={tokenCount(totals.tokens, true)} sub={tasks.sessions ? `${tokenCount(Math.round(tasks.tokens / tasks.sessions), true)} a task` : 'no tasks yet'} />
      <StatsFigure
        icon={Coins}
        label="Cost"
        value={`~${dollars(totals.cost)}`}
        sub={tasks.sessions ? `~${dollars(tasks.cost / tasks.sessions)} a task` : 'no tasks yet'}
        change={change(totals.cost, previous.cost)}
        period={period}
        title={priced}
      />
    </dl>
  )
}
