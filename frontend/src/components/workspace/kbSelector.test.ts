import assert from 'node:assert/strict'
import test from 'node:test'

import {
  buildDropdownItems,
  chevronClass,
  selectorIsInteractive,
  selectorAriaLabel,
} from './kbSelector.ts'
import type { KbListItem } from './kbStore.ts'

test('buildDropdownItems marks the active KB', () => {
  const list: KbListItem[] = [
    { id: 'kb-1', name: 'A' },
    { id: 'kb-2', name: 'B' },
    { id: 'kb-3', name: 'C' },
  ]
  const items = buildDropdownItems(list, 'kb-2')
  assert.deepEqual(items, [
    { id: 'kb-1', name: 'A', isActive: false },
    { id: 'kb-2', name: 'B', isActive: true },
    { id: 'kb-3', name: 'C', isActive: false },
  ])
})

test('buildDropdownItems returns [] for empty / null / undefined list', () => {
  assert.deepEqual(buildDropdownItems([], 'kb-1'), [])
  assert.deepEqual(buildDropdownItems(null, 'kb-1'), [])
  assert.deepEqual(buildDropdownItems(undefined, 'kb-1'), [])
})

test('buildDropdownItems marks nothing active when activeKbId is empty', () => {
  const list: KbListItem[] = [{ id: 'kb-1', name: 'A' }]
  const items = buildDropdownItems(list, '')
  assert.equal(items[0].isActive, false)
})

test('chevronClass returns "open" suffix when the dropdown is open', () => {
  assert.equal(chevronClass(true), 'chevron-icon open')
  assert.equal(chevronClass(false), 'chevron-icon')
})

test('selectorIsInteractive is true when the list has at least one KB', () => {
  assert.equal(selectorIsInteractive([{ id: 'kb-1', name: 'A' }]), true)
})

test('selectorIsInteractive is false for empty / null / undefined list', () => {
  assert.equal(selectorIsInteractive([]), false)
  assert.equal(selectorIsInteractive(null), false)
  assert.equal(selectorIsInteractive(undefined), false)
})

test('selectorAriaLabel reflects open state and current KB name', () => {
  assert.equal(
    selectorAriaLabel(false, '产品文档'),
    '打开知识库选择器，当前：产品文档',
  )
  assert.equal(
    selectorAriaLabel(true, '产品文档'),
    '关闭知识库选择器，当前：产品文档',
  )
})

test('selectorAriaLabel falls back to the default label when name is missing', () => {
  assert.equal(
    selectorAriaLabel(false, undefined),
    '打开知识库选择器，当前：选择知识库',
  )
  assert.equal(
    selectorAriaLabel(false, ''),
    '打开知识库选择器，当前：选择知识库',
  )
})
