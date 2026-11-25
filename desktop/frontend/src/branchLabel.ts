const TAIL_LENGTH = 8

// splitBranch cuts a branch name into the part that may be shortened by the width and the end that always shows,
// so a long name keeps its start and its end. A short name is one part.
export function splitBranch(name: string): { head: string; tail: string } {
  if (name.length <= TAIL_LENGTH * 2) return { head: name, tail: '' }
  return { head: name.slice(0, -TAIL_LENGTH), tail: name.slice(-TAIL_LENGTH) }
}
