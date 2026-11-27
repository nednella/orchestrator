const GB = 2 ** 30

export function gigabytes(bytes: number, digits = 1): string {
  return `${(bytes / GB).toFixed(digits)} GB`
}

export const memoryNote = 'Memory as Activity Monitor counts it, added up over the processes: an estimate.'
