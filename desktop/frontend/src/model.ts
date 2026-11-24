export function shortModel(model: string): string {
  return model.replace(/^(claude|gemini)-/, '')
}

// Claude's model aliases as the picker names them, with the full id each stands for; `more` ones show under "Show more".
export const CLAUDE_MODELS: Record<string, { name: string; short: string; id?: string; more?: true }> = {
  opus: { name: 'Claude Opus 5.5', short: 'Opus 5.5', id: 'claude-opus-5-5' },
  sonnet: { name: 'Claude Sonnet 5.5', short: 'Sonnet 5.5', id: 'claude-sonnet-5-5' },
  fable: { name: 'Claude Fable 5.2', short: 'Fable 5.2', id: 'claude-fable-5-2' },
  haiku: { name: 'Claude Haiku 5.5', short: 'Haiku 5.5', id: 'claude-haiku-5-5' },
  best: { name: 'Best available', short: 'Best', more: true },
  opusplan: { name: 'Opus plan, Sonnet execute', short: 'Opus plan', more: true },
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
