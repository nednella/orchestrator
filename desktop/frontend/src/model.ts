export function shortModel(model: string): string {
  return model.replace(/^(claude|gemini)-/, '')
}

// The models the picker offers, strongest first, by alias, with the full id each stands for. Config and labels
// also take aliases with no row here, such as best and opusplan.
export const CLAUDE_MODELS: Record<string, { name: string; short: string; id: string }> = {
  fable: { name: 'Claude Fable 5.2', short: 'Fable 5.2', id: 'claude-fable-5-2' },
  opus: { name: 'Claude Opus 5.5', short: 'Opus 5.5', id: 'claude-opus-5-5' },
  sonnet: { name: 'Claude Sonnet 5.5', short: 'Sonnet 5.5', id: 'claude-sonnet-5-5' },
  haiku: { name: 'Claude Haiku 5.5', short: 'Haiku 5.5', id: 'claude-haiku-5-5' },
}

export const EFFORTS: Record<string, string> = { low: 'Low', medium: 'Medium', high: 'High', xhigh: 'Extra High', max: 'Max' }

// The model's short name: the alias's, else the id as the agent reports it.
// The alias row for a model Claude's settings name, such as claude-opus-5-5[1m]; a model with no row comes back as it is.
export function modelAlias(model: string): string {
  const id = model.replace(/\[1m\]$/, '').replace(/-\d{8}$/, '')
  if (id in CLAUDE_MODELS) return id
  return Object.keys(CLAUDE_MODELS).find((alias) => CLAUDE_MODELS[alias].id === id) ?? model
}

export const modelShortName = (model: string) => CLAUDE_MODELS[model]?.short ?? shortModel(model)

// The short name of a model given as an alias or as a full id; '' for none.
export const modelName = (model: string) => (model ? modelShortName(modelAlias(model)) : '')
