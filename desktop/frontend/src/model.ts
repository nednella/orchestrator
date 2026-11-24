export function shortModel(model: string): string {
  return model.replace(/^(claude|gemini)-/, '')
}

// Claude's model aliases as the picker names them; `more` ones show under "Show more".
export const CLAUDE_MODELS: Record<string, { name: string; short: string; more?: true }> = {
  opus: { name: 'Claude Opus 5.5', short: 'Opus 5.5' },
  sonnet: { name: 'Claude Sonnet 5.5', short: 'Sonnet 5.5' },
  fable: { name: 'Claude Fable 5.2', short: 'Fable 5.2' },
  haiku: { name: 'Claude Haiku 5.5', short: 'Haiku 5.5' },
  best: { name: 'Best available', short: 'Best', more: true },
  opusplan: { name: 'Opus plan, Sonnet execute', short: 'Opus plan', more: true },
}

export const EFFORTS: Record<string, string> = { low: 'Low', medium: 'Medium', high: 'High', xhigh: 'Extra High', max: 'Max' }

// The model's short name: the alias's, else the id as the agent reports it.
export const modelShortName = (model: string) => CLAUDE_MODELS[model]?.short ?? shortModel(model)
