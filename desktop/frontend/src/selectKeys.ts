// moveOption is the option a key moves to from option i of count, or -1 when the key moves nowhere.
export function moveOption(key: string, i: number, count: number): number {
  switch (key) {
    case 'ArrowDown':
      return Math.min(i + 1, count - 1)
    case 'ArrowUp':
      return Math.max(i - 1, 0)
    case 'Home':
      return 0
    case 'End':
      return count - 1
    default:
      return -1
  }
}
