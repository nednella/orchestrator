// The tests run on Node's own test runner. These are the parts of its types they use.
declare module 'node:test' {
  export function test(name: string, fn: () => void | Promise<void>): void
}

declare module 'node:assert/strict' {
  const assert: {
    equal(actual: unknown, expected: unknown, message?: string): void
    deepEqual(actual: unknown, expected: unknown, message?: string): void
    throws(fn: () => unknown, expected?: RegExp, message?: string): void
  }
  export default assert
}

declare module 'node:fs' {
  export function readFileSync(path: URL, encoding: 'utf8'): string
}
