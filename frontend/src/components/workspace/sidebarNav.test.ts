import assert from 'node:assert/strict'
import test from 'node:test'

import { resolveNavAction, navHasContent, DEFAULT_NAV_ACTION } from './sidebarNav.ts'

test('chat nav clears the current page so the chat view shows', () => {
  const action = resolveNavAction('chat')
  assert.equal(action.activeNav, 'chat')
  assert.equal(action.selectPage, 'clear')
  assert.equal(action.createPage, false)
})

test('knowledge nav keeps the current page and switches to knowledge list', () => {
  const action = resolveNavAction('knowledge')
  assert.equal(action.activeNav, 'knowledge')
  assert.equal(action.selectPage, null)
  assert.equal(action.createPage, false)
})

test('favorites and history nav do not emit select-page (P1: no dead empty panel)', () => {
  const fav = resolveNavAction('favorites')
  const hist = resolveNavAction('history')
  assert.equal(fav.activeNav, 'favorites')
  assert.equal(hist.activeNav, 'history')
  assert.equal(fav.selectPage, null)
  assert.equal(hist.selectPage, null)
  assert.equal(fav.createPage, false)
  assert.equal(hist.createPage, false)
  // They are flagged as no-content so the template shows an empty-state.
  assert.equal(navHasContent('favorites'), false)
  assert.equal(navHasContent('history'), false)
})

test('create action (新建 button) emits create-page so the parent opens the editor (P1)', () => {
  const action = resolveNavAction('create')
  assert.equal(action.createPage, true)
  assert.equal(action.activeNav, 'knowledge')
  assert.equal(action.selectPage, null)
})

test('unknown nav id falls back to the default action', () => {
  assert.deepEqual(resolveNavAction('unknown'), DEFAULT_NAV_ACTION)
})

test('navHasContent is true only for chat and knowledge', () => {
  assert.equal(navHasContent('chat'), true)
  assert.equal(navHasContent('knowledge'), true)
  assert.equal(navHasContent('favorites'), false)
  assert.equal(navHasContent('history'), false)
})
