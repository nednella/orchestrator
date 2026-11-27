// The model menu that is open, if any, takes the number shortcuts: ⌘1 picks the first model it lists.
let open: ((number: number) => void) | null = null

export function registerModelJump(jump: (number: number) => void): () => void {
  open = jump
  return () => {
    if (open === jump) open = null
  }
}

export function jumpToModel(number: number) {
  open?.(number)
}
