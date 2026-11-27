import assert from 'node:assert/strict'
import { test } from 'node:test'
import { createChipScope } from './chipScope.ts'

test('two chips of one name show a folder, until one goes', () => {
  const scope = createChipScope()
  let heard = 0
  scope.subscribe(() => heard++)
  const first = scope.add('package.json')
  assert.equal(scope.names().get('package.json'), 'package.json')
  const second = scope.add('app-frontend/package.json')
  assert.equal(scope.names().get('package.json'), 'package.json')
  assert.equal(scope.names().get('app-frontend/package.json'), 'app-frontend/package.json')
  const before = scope.names()
  const third = scope.add('/tmp/package.json')
  assert.equal(scope.names() !== before, true, 'a new chip gives a new answer')
  assert.equal(scope.names().get('/tmp/package.json'), 'tmp/package.json')
  third()
  second()
  assert.equal(scope.names().get('package.json'), 'package.json')
  assert.equal(heard, 5)
  first()
})

test('a path shown by two chips is one path', () => {
  const scope = createChipScope()
  const a = scope.add('src/a.ts')
  scope.add('src/a.ts')
  assert.equal(scope.names().get('src/a.ts'), 'a.ts')
  a()
  assert.equal(scope.names().get('src/a.ts'), 'a.ts')
})
