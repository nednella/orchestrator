import assert from 'node:assert/strict'
import { test } from 'node:test'
import { EMPTY_CHAT } from './chatEvents.ts'
import { forgetChat, keepChat, keptChat } from './chatKept.ts'
import { forgetOpened, isOpened, setOpened } from './chatOpen.ts'
import { forgetPicks, keepPicks, keptPicks } from './chatQuestions.ts'
import { forgetScroll, keepScroll, keptScroll } from './chatScroll.ts'
import { filesKey, forgetPick, pickFile } from './filesView.ts'
import { attach, attachedTo, forgetSideChatState } from './sideChatStore.ts'

test('what the app keeps of a thread in memory goes with the thread, and only that thread', () => {
  for (const id of ['shop/gone', 'shop/stays']) {
    keepChat(id, EMPTY_CHAT)
    setOpened(id, 'call-1', true)
    keepPicks(id, 'call-1', [])
    keepScroll(id, { top: 10, measured: [] })
    pickFile(filesKey(id, 'shop'), 'a.go')
    attach(id, 'shop/side')
  }
  forgetChat('shop/gone')
  forgetOpened('shop/gone')
  forgetPicks('shop/gone')
  forgetScroll('shop/gone')
  forgetPick('shop/gone')
  forgetSideChatState('shop/gone')
  assert.equal(keptChat('shop/gone'), undefined)
  assert.equal(isOpened('shop/gone', 'call-1'), false)
  assert.equal(keptPicks('shop/gone', 'call-1'), undefined)
  assert.equal(keptScroll('shop/gone'), null)
  assert.deepEqual(attachedTo('shop/gone'), [])
  assert.equal(keptChat('shop/stays') !== undefined, true)
  assert.equal(isOpened('shop/stays', 'call-1'), true)
  assert.equal(keptPicks('shop/stays', 'call-1') !== undefined, true)
  assert.equal(keptScroll('shop/stays') !== null, true)
  assert.deepEqual(attachedTo('shop/stays'), ['shop/side'])
})
