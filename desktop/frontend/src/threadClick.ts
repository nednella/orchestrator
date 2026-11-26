// clickCloses is whether a click on an open thread's row closes it. The second click of a double-click, which opened
// the row with its first, does not.
export function clickCloses(open: boolean, detail: number): boolean {
  return open && detail <= 1
}
