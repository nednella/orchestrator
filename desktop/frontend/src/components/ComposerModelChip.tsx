import { useEffect, useState } from 'react'
import { useOrchestrator, useOrchestratorSelector } from '../OrchestratorContext'
import { ACCESS_NAME, showsAccessIcon } from '../access'
import { api } from '../api'
import { EFFORTS, modelAlias, modelShortName } from '../model'
import type { Access, ModelChoice } from '../types'
import { AccessIcon } from './AccessIcon'
import { ClaudeMark } from './ClaudeMark'
import { ComposerMenuChip } from './ComposerMenuChip'
import { ModelPicker } from './ModelPicker'

// model and effort are the user's picks, '' to leave them to the issue's labels, the action and Claude's settings.
// Without onModel they are a running thread's, which the menu shows and does not change. accessOff is why the access
// cannot change now, '' when it can.
type ComposerModelChipProps = {
  model: string
  effort: string
  access: Access
  accessOff: string
  onModel?(model: string): void
  onEffort?(effort: string): void
  onAccess(access: Access): void
}

export function ComposerModelChip({ model, effort, access, accessOff, onModel, onEffort, onAccess }: ComposerModelChipProps) {
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
  const label = [modelName, effortName].filter(Boolean).join(' ') || 'Default'

  return (
    <ComposerMenuChip
      label={`Model, effort and access: ${label}, ${ACCESS_NAME[access]}`}
      menu={() => <ModelPicker model={model} effort={effort} defaults={defaults} access={access} accessOff={accessOff} onModel={onModel} onEffort={onEffort} onAccess={onAccess} />}
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
        </>
      ) : (
        <span>Default</span>
      )}
    </ComposerMenuChip>
  )
}
