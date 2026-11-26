import { useState } from 'react'
import { useOrchestrator } from '../OrchestratorContext'
import { api } from '../api'
import type { Server, Session } from '../types'
import { Icon } from './Icon'
import { ServerMeta } from './ServerMeta'

type ServersTabProps = { session: Session; servers: Server[]; refresh(): Promise<void> }

// ServersTab lists the ports the thread's servers listen on. A row opens its address in the default browser; its
// button stops the server, with no confirm: the agent started it, and it stops anyway when the thread sleeps.
export function ServersTab({ session, servers, refresh }: ServersTabProps) {
  const { report } = useOrchestrator('report')
  const [stopping, setStopping] = useState<number | null>(null)

  const stop = (pid: number) =>
    report(async () => {
      setStopping(pid)
      try {
        await api.stopServer(session.id, pid)
        await refresh()
      } finally {
        setStopping(null)
      }
    })

  if (servers.length === 0) {
    return (
      <div className="grid h-full place-items-center p-6 text-center">
        <p className="max-w-xs text-soft">No servers running. A server the agent starts shows here, and stops when the thread sleeps.</p>
      </div>
    )
  }

  return (
    <ul className="absolute inset-0 overflow-y-auto py-1">
      {servers.map((server) => (
        <li key={server.port} className="flex items-center gap-1 pr-2 hover:bg-hover">
          <button
            className="flex min-w-0 flex-1 flex-col items-start gap-0.5 py-2 pl-4 text-left"
            title={`Open http://localhost:${server.port}\n${server.command}`}
            onClick={() => report(() => api.openServer(server.port))}
          >
            <span className="mono text-body text-ink">localhost:{server.port}</span>
            <span className="w-full truncate text-small text-dim">
              <ServerMeta server={server} />
            </span>
          </button>
          <button
            className="btn btn-ghost h-7 w-7 flex-none justify-center px-0"
            title="Stop this server"
            aria-label={`Stop the server on port ${server.port}`}
            disabled={stopping === server.pid}
            onClick={() => stop(server.pid)}
          >
            <Icon name="stop" size={14} />
          </button>
        </li>
      ))}
    </ul>
  )
}
