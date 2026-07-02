import assert from 'node:assert/strict'
import test from 'node:test'

import {
  MENU_PATHS,
  TOP_MENU_PATHS,
  LITE_HIDDEN_PATHS,
  buildMenuItems,
  isTopMenuItem,
} from './menuPaths.ts'

test('buildMenuItems returns one entry per MENU_PATHS entry, in order', () => {
  const items = buildMenuItems()
  assert.equal(items.length, MENU_PATHS.length)
  assert.deepEqual(
    items.map((i) => i.path),
    [...MENU_PATHS],
  )
})

test('workspace entry exists so the three-column workspace is navigable (P0)', () => {
  const items = buildMenuItems()
  const workspace = items.find((i) => i.path === 'workspace')
  assert.ok(workspace, 'workspace menu item must exist')
  assert.equal(workspace!.icon, 'workspace')
  assert.equal(workspace!.titleKey, 'menu.workspace')
})

test('workspace is in the top-menu whitelist (P0)', () => {
  assert.ok(TOP_MENU_PATHS.has('workspace'), 'workspace must be a top-menu item')
  assert.ok(isTopMenuItem('workspace'))
  // Bottom-menu items (settings, logout) must NOT be in the top whitelist.
  assert.ok(!isTopMenuItem('settings'))
  assert.ok(!isTopMenuItem('logout'))
})

test('every top-menu item has a corresponding buildMenuItems entry', () => {
  const itemPaths = new Set(buildMenuItems().map((i) => i.path))
  for (const p of TOP_MENU_PATHS) {
    assert.ok(itemPaths.has(p), `top-menu path ${p} missing from buildMenuItems`)
  }
})

test('lite hidden paths only hide logout and organizations', () => {
  assert.ok(LITE_HIDDEN_PATHS.has('logout'))
  assert.ok(LITE_HIDDEN_PATHS.has('organizations'))
  // workspace must remain visible in lite mode.
  assert.ok(!LITE_HIDDEN_PATHS.has('workspace'))
  assert.ok(!LITE_HIDDEN_PATHS.has('knowledge-bases'))
})

test('creatChat entry keeps its children slot for session list', () => {
  const chat = buildMenuItems().find((i) => i.path === 'creatChat')
  assert.ok(chat)
  assert.equal(chat!.childrenPath, 'chat')
  assert.ok(Array.isArray(chat!.children))
})
