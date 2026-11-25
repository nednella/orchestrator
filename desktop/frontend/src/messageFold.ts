// A long message of the owner's folds to foldHeight, its last fadeHeight fading out; both in rem.
export const foldHeight = 11
export const fadeHeight = 1.75

// folds says whether a message body of that height in px folds. One that fits in the folded height and its fade shows
// whole, since folding it would hide no more than the fade covers.
export function folds(height: number, remPx: number): boolean {
  return height > (foldHeight + fadeHeight) * remPx
}
