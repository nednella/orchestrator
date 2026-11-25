// short keeps three figures at most, as a figure card shows them: "412M", "4.4M".
export function tokenCount(n: number, short = false): string {
  if (n < 1000) return String(n)
  if (short) {
    const [size, unit] = n < 999_500 ? [1000, 'k'] : [1_000_000, 'M']
    return `${Number((n / size).toPrecision(3))}${unit}`
  }
  if (n < 1_000_000) return `${(n / 1000).toFixed(n < 10_000 ? 1 : 0)}k`
  return `${(n / 1_000_000).toFixed(n < 10_000_000 ? 2 : 1)}M`
}

export function dollars(cost: number): string {
  if (cost > 0 && cost < 0.01) return '<$0.01'
  return `$${cost.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`
}
