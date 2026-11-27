import { useState } from 'react'
import { readStored, writeStored } from './storage'
import { toggled } from './modelMenu.ts'

const KEY = 'orchestrator.favoriteModels'

// The models the owner starred in the model menu, saved with the other interface state.
export function useFavoriteModels(): [string[], (id: string) => void] {
  const [favorites, setFavorites] = useState(() => readStored<string[]>(KEY, []))
  const toggle = (id: string) => {
    const next = toggled(favorites, id)
    writeStored(KEY, next)
    setFavorites(next)
  }
  return [favorites, toggle]
}
