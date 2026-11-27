import { chipNames } from './filePaths.ts'

// createChipScope is the chips one message shows, so that two files of one name show a folder to tell them apart. A
// chip adds its file's path while it shows; names is the name each path shows, the same object until a chip comes or goes.
export function createChipScope() {
  const keys = new Map<symbol, string>()
  const listeners = new Set<() => void>()
  let names = new Map<string, string>()

  const changed = () => {
    names = chipNames([...new Set(keys.values())])
    listeners.forEach((listener) => listener())
  }

  return {
    add(path: string) {
      const id = Symbol(path)
      keys.set(id, path)
      changed()
      return () => {
        keys.delete(id)
        changed()
      }
    },
    names: () => names,
    subscribe(listener: () => void) {
      listeners.add(listener)
      return () => {
        listeners.delete(listener)
      }
    },
  }
}

export type ChipScope = ReturnType<typeof createChipScope>
