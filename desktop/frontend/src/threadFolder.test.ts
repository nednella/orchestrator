import assert from 'node:assert/strict'
import { test } from 'node:test'
import { threadFolder } from './threadFolder.ts'

test('the project folder reads as Project folder', () => {
  assert.equal(threadFolder('', '/code/app'), 'Project folder')
  assert.equal(threadFolder('/code/app', '/code/app'), 'Project folder')
})

test('a folder inside the project is relative, one outside is in full', () => {
  assert.equal(threadFolder('/code/app/trees/fix', '/code/app'), 'trees/fix')
  assert.equal(threadFolder('/code/app-other/x', '/code/app'), '/code/app-other/x')
  assert.equal(threadFolder('/elsewhere/x', '/code/app'), '/elsewhere/x')
})
