import assert from 'node:assert/strict'
import { test } from 'node:test'
import type { Agentos } from '../AgentosContext.tsx'
import type { Layout } from '../LayoutContext.tsx'
import type { Session } from '../types.ts'
import { OPEN_IN_EDITOR_LABEL, sessionActions } from './sessions.ts'

const thread = { id: 'shop/aaaaaaaa', n: 1, title: 'Fix cart' } as Session

function openAction(current: Session | undefined, opened: string[]) {
  const a = { openInEditor: async (id: string) => void opened.push(id) } as unknown as Agentos
  return sessionActions({ a, layout: {} as Layout, current }).find((x) => x.id === 'open-in-editor')
}

test('the registry has Open in VS Code, which opens the selected thread by its id', async () => {
  const opened: string[] = []
  const action = openAction(thread, opened)
  assert.equal(action?.label, OPEN_IN_EDITOR_LABEL)
  await action?.run([])
  assert.deepEqual(opened, [thread.id])
})

test('Open in VS Code with no thread selected rejects with a sentence and opens nothing', async () => {
  const opened: string[] = []
  const failure = await Promise.resolve(openAction(undefined, opened)?.run([])).then(
    () => '',
    (e: unknown) => String(e),
  )
  assert.equal(failure, 'Select a thread first')
  assert.deepEqual(opened, [])
})
