import { Zap } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useOrchestrator, useOrchestratorSelector } from '../OrchestratorContext'
import { ACCESS_NAME, showsAccessIcon } from '../access'
import { api } from '../api'
import { EFFORTS, modelAlias, modelShortName, supportsFast } from '../model'
import type { Access, ModelChoice } from '../types'
import { AccessIcon } from './AccessIcon'
import { ClaudeMark } from './ClaudeMark'
import { ComposerMenuChip } from './ComposerMenuChip'
import { ModelPicker } from './ModelPicker'

// model and effort are the user's picks, '' to leave them to the issue's labels, the action and Claude's settings.
// Without onModel the model is a running thread's, which the menu shows and does not change; the effort and fast mode
// are its own to change while the thread runs, with onEffort and onFast. accessOff is why the access cannot change
// now, '' when it can.
type ComposerModelChipProps = {
  model: string
  effort: string
  fast: boolean
  access: Access
  accessOff: string
  onModel?(model: string): void
  onEffort?(effort: string): void
  onFast?(fast: boolean): void
  onAccess(access: Access): void
}

export function ComposerModelChip({ model, effort, fast, access, accessOff, onModel, onEffort, onFast, onAccess }: ComposerModelChipProps) {
  const { report } = useOrchestrator('report')
  const projectKey = useOrchestratorSelector((a) => a.project?.key ?? '')
  const defaultAccess = useOrchestratorSelector((a) => a.settings.threadAccess)
  const [defaults, setDefaults] = useState<ModelChoice>({ model: '', effort: '' })
  const fixed = !onModel

  useEffect(() => {
    if (fixed) return
    let current = true
    report(async () => {
      const [base, picked] = await Promise.all([api.defaultModel(), model ? api.defaultModel(model) : null])
      if (current) setDefaults({ model: modelAlias(base.model), effort: (picked ?? base).effort })
    })
    return () => {
      current = false
    }
  }, [fixed, projectKey, model, report])

  const shownModel = model || defaults.model
  const shownEffort = effort || defaults.effort
  const modelName = shownModel ? modelShortName(shownModel) : ''
  const effortName = EFFORTS[shownEffort] ?? shownEffort
  const fastOn = fast && supportsFast(shownModel)
  const label = [modelName, effortName, fastOn && 'fast'].filter(Boolean).join(' ') || 'Default'

  return (
    <ComposerMenuChip
      label={`Model, effort and access: ${label}, ${ACCESS_NAME[access]}`}
      menu={() => <ModelPicker model={model} effort={effort} fast={fast} defaults={defaults} access={access} accessOff={accessOff} onModel={onModel} onEffort={onEffort} onFast={onFast} onAccess={onAccess} />}
    >
      {showsAccessIcon(access, defaultAccess) && (
        <span className="flex text-soft" title={`Access: ${ACCESS_NAME[access]}`}>
          <AccessIcon access={access} />
        </span>
      )}
      <ClaudeMark />
      {modelName || effortName ? (
        <>
          {modelName && <span className="min-w-0 truncate">{modelName}</span>}
          {effortName && <span className="composer-chip-detail flex-none">{effortName}</span>}
          {fastOn && <Zap size={13} fill="currentColor" aria-label="Fast mode on" role="img" style={{ color: 'var(--agent-claude)' }} />}
        </>
      ) : (
        <span>Default</span>
      )}
    </ComposerMenuChip>
  )
}
