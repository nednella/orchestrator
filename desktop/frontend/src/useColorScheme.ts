import { useSyncExternalStore } from 'react'

const scheme = (): 'dark' | 'light' => (getComputedStyle(document.documentElement).colorScheme === 'light' ? 'light' : 'dark')

function subscribe(onChange: () => void) {
  const observer = new MutationObserver(onChange)
  observer.observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] })
  return () => observer.disconnect()
}

// useColorScheme is whether the theme in effect is a light or a dark one, kept as the theme changes.
export function useColorScheme(): 'dark' | 'light' {
  return useSyncExternalStore(subscribe, scheme)
}
