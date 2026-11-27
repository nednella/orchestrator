import { useEffect } from 'react'
import { registerModelJump } from './modelJump'

// While the model menu is mounted, ⌘ and a number run jump with that number.
export function useModelJump(jump: (number: number) => void) {
  // jump closes over the menu's latest picks, so the registration follows every render.
  useEffect(() => registerModelJump(jump))
}
