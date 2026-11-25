import { useState } from 'react'
import { InlineInput } from './InlineInput'

// The deny button opens a one-line reason first, which may be left empty.
type ChatAllowDenyProps = { allow: string; deny: string; reasonHint: string; busy: boolean; onAllow(): void; onDeny(reason: string): void }

export function ChatAllowDeny({ allow, deny, reasonHint, busy, onAllow, onDeny }: ChatAllowDenyProps) {
  const [denying, setDenying] = useState(false)

  if (denying) {
    return (
      <InlineInput
        placeholder={reasonHint}
        blur="cancel"
        action={deny}
        onSubmit={(reason) => {
          setDenying(false)
          onDeny(reason.trim())
        }}
        onCancel={() => setDenying(false)}
      />
    )
  }
  return (
    <div className="flex gap-2">
      <button className="btn btn-primary" disabled={busy} onClick={onAllow}>
        {allow}
      </button>
      <button className="btn" disabled={busy} onClick={() => setDenying(true)}>
        {deny}
      </button>
    </div>
  )
}
