// FooterMode is how the composer's footer lays its controls out: wide shows their words, compact turns the mode switch
// and the issue chip into icons.
export type FooterMode = 'wide' | 'compact'

const compactUnder = 620

// footerMode is the footer's layout for the width of the composer's form, in px.
export function footerMode(formWidth: number): FooterMode {
  return formWidth < compactUnder ? 'compact' : 'wide'
}
