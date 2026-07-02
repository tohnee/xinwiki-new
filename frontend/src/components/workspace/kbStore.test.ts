import assert from 'node:assert/strict'
import test from 'node:test'

import {
  pickInitialKbId,
  resolveKbName,
  normalizeKbList,
  type KbListItem,
} from './kbStore.ts'

test('pickInitialKbId prefers the auth store current KB id', () => {
  const list: KbListItem[] = [{ id: 'kb-1', name: 'A' }, { id: 'kb-2', name: 'B' }]
  assert.equal(pickInitialKbId('kb-2', list), 'kb-2')
})

test('pickInitialKbId falls back to the first KB when no current id', () => {
  const list: KbListItem[] = [{ id: 'kb-1', name: 'A' }]
  assert.equal(pickInitialKbId(undefined, list), 'kb-1')
  assert.equal(pickInitialKbId('', list), 'kb-1')
  assert.equal(pickInitialKbId(null, list), 'kb-1')
})

test('pickInitialKbId returns empty string when no KB is available', () => {
  assert.equal(pickInitialKbId(undefined, []), '')
  assert.equal(pickInitialKbId(undefined, null), '')
  assert.equal(pickInitialKbId(undefined, undefined), '')
})

test('pickInitialKbId ignores a non-string current id (defensive)', () => {
  const list: KbListItem[] = [{ id: 'kb-1', name: 'A' }]
  // @ts-expect-error intentionally bad input
  assert.equal(pickInitialKbId(123, list), 'kb-1')
})

test('resolveKbName returns the KB name for a known id', () => {
  const list: KbListItem[] = [{ id: 'kb-1', name: '产品文档' }, { id: 'kb-2', name: '研发文档' }]
  assert.equal(resolveKbName('kb-2', list), '研发文档')
})

test('resolveKbName returns the fallback for an unknown id', () => {
  const list: KbListItem[] = [{ id: 'kb-1', name: 'A' }]
  assert.equal(resolveKbName('kb-x', list), '选择知识库')
})

test('resolveKbName returns the fallback for empty / missing id', () => {
  const list: KbListItem[] = [{ id: 'kb-1', name: 'A' }]
  assert.equal(resolveKbName('', list), '选择知识库')
  assert.equal(resolveKbName(undefined, list), '选择知识库')
  assert.equal(resolveKbName(null, list), '选择知识库')
})

test('resolveKbName tolerates a missing list', () => {
  assert.equal(resolveKbName('kb-1', null), '选择知识库')
  assert.equal(resolveKbName('kb-1', undefined), '选择知识库')
})

test('resolveKbName uses the custom fallback when supplied', () => {
  const list: KbListItem[] = [{ id: 'kb-1', name: 'A' }]
  assert.equal(resolveKbName('kb-x', list, '未选择'), '未选择')
})

test('normalizeKbList handles the data-array response shape', () => {
  const raw = { data: [{ id: 1, name: 'A' }, { id: 2, name: 'B' }] }
  assert.deepEqual(normalizeKbList(raw), [
    { id: '1', name: 'A' },
    { id: '2', name: 'B' },
  ])
})

test('normalizeKbList handles the knowledge_bases-array response shape', () => {
  const raw = { knowledge_bases: [{ id: 'kb-1', name: 'A' }] }
  assert.deepEqual(normalizeKbList(raw), [{ id: 'kb-1', name: 'A' }])
})

test('normalizeKbList handles a bare array response', () => {
  const raw = [{ id: 'kb-1', name: 'A' }]
  assert.deepEqual(normalizeKbList(raw), [{ id: 'kb-1', name: 'A' }])
})

test('normalizeKbList falls back to title when name is missing', () => {
  const raw = [{ id: 'kb-1', title: 'A' }]
  assert.deepEqual(normalizeKbList(raw), [{ id: 'kb-1', name: 'A' }])
})

test('normalizeKbList filters out entries without an id', () => {
  const raw = [{ id: 'kb-1', name: 'A' }, { name: 'no id' }, null, { id: '', name: 'empty id' }]
  assert.deepEqual(normalizeKbList(raw), [{ id: 'kb-1', name: 'A' }])
})

test('normalizeKbList returns [] for null / undefined / non-array', () => {
  assert.deepEqual(normalizeKbList(null), [])
  assert.deepEqual(normalizeKbList(undefined), [])
  assert.deepEqual(normalizeKbList({}), [])
  assert.deepEqual(normalizeKbList('not-an-array'), [])
})
