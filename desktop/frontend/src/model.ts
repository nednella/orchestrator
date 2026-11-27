export function shortModel(model: string): string {
  return model.replace(/^claude-/, '')
}

// The models the picker offers, strongest first, by alias, with the full id each stands for. Config and labels
// also take aliases with no row here, such as best and opusplan. fast and ultracode say whether Claude gives the model
// fast mode and ultracode, as its model list reports them: ultracode is for the models that take the xhigh effort.
export const CLAUDE_MODELS: Record<string, { name: string; short: string; id: string; fast: boolean; ultracode: boolean }> = {
  fable: { name: 'Claude Fable 5.2', short: 'Fable 5.2', id: 'claude-fable-5-2', fast: true, ultracode: true },
  opus: { name: 'Claude Opus 5.5', short: 'Opus 5.5', id: 'claude-opus-5-5', fast: true, ultracode: true },
  sonnet: { name: 'Claude Sonnet 5.5', short: 'Sonnet 5.5', id: 'claude-sonnet-5-5', fast: false, ultracode: true },
  haiku: { name: 'Claude Haiku 5.5', short: 'Haiku 5.5', id: 'claude-haiku-5-5', fast: false, ultracode: true },
}

export const EFFORTS: Record<string, string> = { low: 'Low', medium: 'Medium', high: 'High', xhigh: 'Extra High', max: 'Max', ultracode: 'Ultracode' }

// What an effort means, for the ones whose name does not say.
export const EFFORT_HINTS: Record<string, string> = { ultracode: 'xhigh effort plus multi-agent workflow orchestration' }

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

// Whether Claude gives the model, an alias or a full id, fast mode.
export const supportsFast = (model: string) => CLAUDE_MODELS[modelAlias(model)]?.fast ?? false

// The effort ids the model takes, weakest first: ultracode only for a model that has it.
export const effortsOf = (model: string) => Object.keys(EFFORTS).filter((id) => id !== 'ultracode' || (CLAUDE_MODELS[modelAlias(model)]?.ultracode ?? false))

// The fast mode and effort a thread keeps when its model becomes model: fast mode goes off for a model with none, and
// ultracode for the xhigh it runs at.
export function optionsAfterModel(model: string, fast: boolean, effort: string): { fast: boolean; effort: string } {
  return { fast: fast && supportsFast(model), effort: effort === 'ultracode' && !effortsOf(model).includes(effort) ? 'xhigh' : effort }
}
