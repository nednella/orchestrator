import assert from 'node:assert/strict'
import { test } from 'node:test'
import { loadView } from './loadView.ts'

test('data wins over an old error', () => assert.equal(loadView(true, 'late'), 'ready'))
test('a failed read is an error, not a placeholder', () => assert.equal(loadView(false, 'no ledger'), 'error'))
test('no data and no error is still loading', () => assert.equal(loadView(false, ''), 'loading'))
