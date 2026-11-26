import { serverMeta } from '../contextCard'
import { duration, useClock } from '../time'
import type { Server } from '../types'

type ServerMetaProps = { server: Server }

// ServerMeta is the line under a server's address, with how long it has run kept up to date.
export function ServerMeta({ server }: ServerMetaProps) {
  return <>{useClock((now) => serverMeta(server.command, server.started ? duration(now - server.started) : ''))}</>
}
