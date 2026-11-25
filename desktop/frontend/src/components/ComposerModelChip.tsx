import { useEffect, useState } from 'react'
import { useAgentos, useAgentosSelector } from '../AgentosContext'
import { api } from '../api'
import { EFFORTS, modelAlias, modelShortName } from '../model'
import type { ModelChoice } from '../types'
import { ClaudeMark } from './ClaudeMark'
import { ComposerMenuChip } from './ComposerMenuChip'
import { ModelPicker } from './ModelPicker'

// model and effort are the user's picks, '' to leave them to the issue's labels, the action and Claude's settings.
type ComposerModelChipProps = {
  model: string
  effort: string
  onModel(model: string): void
  onEffort(effort: string): void
}

export function ComposerModelChip({ model, effort, onModel, onEffort }: ComposerModelChipProps) {
  const { report } = useAgentos('report')
  const projectKey = useAgentosSelector((a) => a.project?.key ?? '')
  const [defaults, setDefaults] = useState<ModelChoice>({ model: '', effort: '' })

  useEffect(() => {
    let current = true
    report(async () => {
      const [base, picked] = await Promise.all([api.defaultModel(), model ? api.defaultModel(model) : null])
      if (current) setDefaults({ model: modelAlias(base.model), effort: (picked ?? base).effort })
    })
    return () => {
      current = false
    }
  }, [projectKey, model, report])

  const shownModel = model || defaults.model
  const shownEffort = effort || defaults.effort
  const modelName = shownModel ? modelShortName(shownModel) : ''
  const effortName = EFFORTS[shownEffort] ?? shownEffort
  const label = [modelName, effortName].filter(Boolean).join(' ') || 'Default'

  return (
    <ComposerMenuChip
      label={`Model and reasoning: ${label}`}
      menu={(close) => <ModelPicker model={model} effort={effort} defaults={defaults} onModel={onModel} onEffort={onEffort} onDone={close} />}
    >
      <ClaudeMark />
      {modelName || effortName ? (
        <>
          {modelName && <span className="max-w-24 truncate">{modelName}</span>}
          {effortName && <span className="composer-chip-detail max-w-24 truncate">{effortName}</span>}
        </>
      ) : (
        <span>Default</span>
      )}
    </ComposerMenuChip>
  )
}
