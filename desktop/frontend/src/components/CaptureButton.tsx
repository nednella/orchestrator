import { useState } from 'react'
import { useOrchestrator } from '../OrchestratorContext'
import { api } from '../api'
import { Camera } from 'lucide-react'
import { ChatActionButton } from './ChatActionButton'
import { InlineInput } from './InlineInput'

type CaptureButtonProps = { id: string }

export function CaptureButton({ id }: CaptureButtonProps) {
  const { report, pushToast } = useOrchestrator('report', 'pushToast')
  const [capturing, setCapturing] = useState(false)

  if (capturing) {
    return (
      <InlineInput
        placeholder="Caption (optional)"
        action="Capture"
        blur="cancel"
        className="w-64 max-w-full"
        onSubmit={(caption) => {
          setCapturing(false)
          report(async () => {
            await api.browserScreenshot(id, caption.trim())
            pushToast({ tone: 'success', text: 'Captured to Media' })
          })
        }}
        onCancel={() => setCapturing(false)}
      />
    )
  }
  return (
    <ChatActionButton label="Capture the page to Media" onClick={() => setCapturing(true)}>
      <Camera size={14} aria-hidden="true" />
      <span className="hidden sm:inline">Capture</span>
    </ChatActionButton>
  )
}
