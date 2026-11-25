import type { SessionView } from './AgentosContext'

const order: SessionView[] = ['pr', 'evidence', 'browser', 'agents', 'servers']

// shownView is the tab a thread's details show for the one chosen: PR while the Agents tab is hidden.
export function shownView(chosen: SessionView, withAgents: boolean): SessionView {
  return chosen === 'agents' && !withAgents ? 'pr' : chosen
}

// nextView is the tab delta steps from the one shown, among the tabs shown, round the ends. A chosen Servers tab shows
// also once the thread has no server left.
export function nextView(chosen: SessionView, delta: -1 | 1, withAgents: boolean, withServers: boolean): SessionView {
  const view = shownView(chosen, withAgents)
  const views = order.filter((v) => (v !== 'agents' || withAgents) && (v !== 'servers' || withServers || view === 'servers'))
  const at = views.indexOf(view)
  return views[(at + delta + views.length) % views.length]
}
