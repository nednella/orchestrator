import { useMemo, useSyncExternalStore } from 'react'
import { previewTokens } from './questionPreview'

const read = (): string => {
  const style = getComputedStyle(document.documentElement)
  return JSON.stringify(Object.fromEntries(['color-scheme', ...previewTokens].map((name) => [name, style.getPropertyValue(name).trim() || (name === 'color-scheme' ? style.colorScheme : '')])))
}

function subscribe(onChange: () => void) {
  const observer = new MutationObserver(onChange)
  observer.observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme', 'style'] })
  return () => observer.disconnect()
}

// usePreviewTokens is the values of the theme tokens a preview's page may use, kept as the theme or its accent changes.
export function usePreviewTokens(): Record<string, string> {
  const values = useSyncExternalStore(subscribe, read)
  return useMemo(() => JSON.parse(values) as Record<string, string>, [values])
}
