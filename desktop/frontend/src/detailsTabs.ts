import type { SessionView } from './AgentosContext'

const order: SessionView[] = ['pr', 'evidence', 'browser', 'agents']

// shownView is the tab a thread's details show for the one chosen: PR while the Agents tab is hidden.
export function shownView(chosen: SessionView, withAgents: boolean): SessionView {
  return chosen === 'agents' && !withAgents ? 'pr' : chosen
}

// nextView is the tab delta steps from the one shown, among the tabs shown, round the ends.
export function nextView(chosen: SessionView, delta: -1 | 1, withAgents: boolean): SessionView {
  const views = order.filter((v) => v !== 'agents' || withAgents)
  const at = views.indexOf(shownView(chosen, withAgents))
  return views[(at + delta + views.length) % views.length]
}
