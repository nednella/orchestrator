// What a view that reads its data shows: a read that failed shows its error, not the placeholder for ever.
export function loadView(hasData: boolean, error: string): 'ready' | 'error' | 'loading' {
  if (hasData) return 'ready'
  return error ? 'error' : 'loading'
}
