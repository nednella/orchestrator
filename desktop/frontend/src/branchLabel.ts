const TAIL_LENGTH = 8

// splitBranch cuts a branch name into the part that may be shortened by the width and the end that always shows,
// so a long name keeps its start and its end. A short name is one part. It counts characters, not UTF-16 units.
export function splitBranch(name: string): { head: string; tail: string } {
  const chars = Array.from(name)
  if (chars.length <= TAIL_LENGTH * 2) return { head: name, tail: '' }
  return { head: chars.slice(0, -TAIL_LENGTH).join(''), tail: chars.slice(-TAIL_LENGTH).join('') }
}
