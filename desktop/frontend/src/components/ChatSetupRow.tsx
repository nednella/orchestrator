import { useLayout } from '../LayoutContext'
import { activateTerminal } from '../projectTerminalOps'

// ChatSetupRow is the line of a new worktree's set-up command: quiet while it runs, and when it failed its exit code
// with a button that opens its terminal. The thread goes on either way.
type ChatSetupRowProps = { text: string; failed: boolean; shell: string }

export function ChatSetupRow({ text, failed, shell }: ChatSetupRowProps) {
  const layout = useLayout()
  return (
    <div className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1 px-1 py-1 text-small select-none">
      <span className={`min-w-0 break-words ${failed ? 'text-danger' : 'text-soft'}`}>{text}</span>
      {failed && shell !== '' && (
        <button
          className="btn h-6 flex-none px-2 text-label"
          onClick={() => {
            layout.showPanelView('terminal')
            activateTerminal(shell)
          }}
        >
          Open terminal
        </button>
      )}
    </div>
  )
}
