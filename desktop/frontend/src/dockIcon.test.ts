import assert from 'node:assert/strict'
import { test } from 'node:test'
import { themeFollower } from './dockIcon.ts'
import type { ThemeName } from './types.ts'

test('the dock icon is shown once for each change of the shown theme', () => {
  const shown: ThemeName[] = []
  const follow = themeFollower((theme) => shown.push(theme))
  for (const theme of ['black', 'black', 'midnight', 'midnight', 'midnight', 'linen', 'linen-dark', 'linen-dark', 'black', 'black'] as const) follow(theme)
  assert.deepEqual(shown, ['black', 'midnight', 'linen', 'linen-dark', 'black'])
})
