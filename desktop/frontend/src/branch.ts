const BRANCH_LENGTH = 40

// branchFromTitle names a thread's branch as the app does: the title in lower case, each run of characters other than
// a-z and 0-9 a `-`, at most 40 characters cut at a `-`; `thread` when nothing is left.
export function branchFromTitle(title: string): string {
  const name = title
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-|-$/g, '')
  let cut = name
  if (name.length > BRANCH_LENGTH) {
    const i = name.lastIndexOf('-', BRANCH_LENGTH)
    cut = i > 0 ? name.slice(0, i) : name.slice(0, BRANCH_LENGTH)
  }
  return cut || 'thread'
}

// freeBranch is name, or name with -2, -3… after it, the first that is not taken.
export function freeBranch(name: string, taken: (branch: string) => boolean): string {
  let branch = name
  for (let n = 2; taken(branch); n++) branch = `${name}-${n}`
  return branch
}
