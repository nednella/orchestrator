import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import { completeFolder, directoryOf, enter, folderName, isBrowsePath, matchFolders, parentOf, planBrowse, projectPath, repositoryOf, tailOf } from './projectPath.ts'

const folders = (...names: string[]) => names.map((name) => ({ name, path: `/x/${name}` }))

test('folder names follow the cases both sides share', () => {
  const cases: { name: string; folder: string }[] = JSON.parse(readFileSync(new URL('../../projects/testdata/folder-names.json', import.meta.url), 'utf8'))
  for (const c of cases) assert.equal(folderName(c.name), c.folder, JSON.stringify(c.name))
})

test('a project path joins the root and the folder name', () => {
  assert.equal(projectPath('/Users/me/code/', 'My App'), '/Users/me/code/my-app')
  assert.equal(repositoryOf('me', '/Users/me/code/my-app'), 'me/my-app')
})

test('paths the browser lists', () => {
  for (const p of ['/a', '~/a', './a', '../a']) assert.equal(isBrowsePath(p), true, p)
  for (const p of ['', 'a', '~', 'new']) assert.equal(isBrowsePath(p), false, p)
})

test('directory, tail and parent', () => {
  assert.equal(directoryOf('~/code/ap'), '~/code/')
  assert.equal(directoryOf('~/code/'), '~/code/')
  assert.equal(tailOf('~/code/ap'), 'ap')
  assert.equal(tailOf('~/code/'), '')
  assert.equal(parentOf('~/code/'), '~/')
  assert.equal(parentOf('~/'), null)
  assert.equal(parentOf('/Users/me/'), '/Users/')
  assert.equal(parentOf('/Users/'), '/')
  assert.equal(parentOf('/'), null)
  assert.equal(parentOf('./a/'), './')
  assert.equal(enter('~/co', 'code'), '~/code/')
})

test('folders match by name, hide dot folders and find the exact one', () => {
  const all = folders('.git', 'app', 'my-app', 'zebra', 'Pappy')
  assert.deepEqual(matchFolders(all, '').visible.map((e) => e.name), ['app', 'my-app', 'zebra', 'Pappy'])
  assert.deepEqual(matchFolders(all, '.').visible.map((e) => e.name), ['.git'])
  const found = matchFolders(all, 'app')
  assert.deepEqual(found.visible.map((e) => e.name), ['app', 'my-app', 'Pappy'])
  assert.equal(found.exact?.name, 'app')
  assert.equal(matchFolders(all, 'ap').exact, null)
  assert.deepEqual(matchFolders(all, 'mpp').visible.map((e) => e.name), ['my-app'])
})

test('tab goes into a single match and extends several to what they share', () => {
  assert.deepEqual(completeFolder(folders('alpha', 'beta'), 'al'), { name: 'alpha', whole: true })
  assert.deepEqual(completeFolder(folders('alpha', 'alpine', 'beta'), 'a'), { name: 'alp', whole: false })
  assert.equal(completeFolder(folders('alpha', 'alpine'), 'alp'), null)
  assert.equal(completeFolder(folders('alpha'), 'z'), null)
})

const listing = { dir: '/home/me/code/', parent: '/home/me/', exists: true, entries: [{ name: 'app', path: '/home/me/code/app' }, { name: 'api', path: '/home/me/code/api' }] }

test('a path that ends in a slash adds the folder listed', () => {
  const plan = planBrowse('~/code/', listing)
  assert.equal(plan.directory, '~/code/')
  assert.equal(plan.parent, '~/')
  assert.equal(plan.path, '/home/me/code/')
  assert.equal(plan.creates, false)
  assert.deepEqual(plan.visible.map((e) => e.name), ['app', 'api'])
})

test('a slash after a folder that is not there makes it', () => {
  const plan = planBrowse('~/nowhere/', { dir: '/home/me/nowhere/', parent: '/home/me/', exists: false, entries: [] })
  assert.equal(plan.creates, true)
})

test('a name that is a folder adds that folder, any other name makes one', () => {
  assert.deepEqual([planBrowse('~/code/app', listing).path, planBrowse('~/code/app', listing).creates], ['/home/me/code/app', false])
  assert.deepEqual([planBrowse('~/code/new', listing).path, planBrowse('~/code/new', listing).creates], ['~/code/new', true])
  assert.equal(planBrowse('~/code/ap', listing).visible.length, 2)
})
