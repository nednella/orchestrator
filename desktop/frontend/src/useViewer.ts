import { useEffect, useState } from 'react'
import { api } from './api'

let known: string | null = null

// useViewer is the GitHub login the lists call "me", or null while it is read or when it cannot be.
export function useViewer(): string | null {
  const [viewer, setViewer] = useState(known)
  useEffect(() => {
    if (known !== null) return
    let live = true
    api.viewer().then(
      (login) => {
        known = login
        if (live) setViewer(login)
      },
      () => {},
    )
    return () => {
      live = false
    }
  }, [])
  return viewer
}
