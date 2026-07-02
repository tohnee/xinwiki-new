import assert from 'node:assert/strict'
import test from 'node:test'

import { formatSourcesCount, truncateExcerpt } from './sourcesPanel.ts'
import type { Citation } from './useGeneration.ts'

const makeCitations = (n: number): Citation[] =>
  Array.from({ length: n }, (_, i) => ({
    id: String(i),
    title: `文档 ${i}`,
    excerpt: `片段 ${i}`,
  }))

test('formatSourcesCount shows 暂无知识源 when no citations (P1: no more fake 12)', () => {
  assert.equal(formatSourcesCount([]), '暂无知识源')
})

test('formatSourcesCount shows the real citation count', () => {
  assert.equal(formatSourcesCount(makeCitations(3)), '3 个知识源')
  assert.equal(formatSourcesCount(makeCitations(7)), '7 个知识源')
})

test('truncateExcerpt returns short excerpts unchanged', () => {
  assert.equal(truncateExcerpt('short text'), 'short text')
  assert.equal(truncateExcerpt(''), '')
})

test('truncateExcerpt caps long excerpts and adds ellipsis', () => {
  const long = 'x'.repeat(120)
  const out = truncateExcerpt(long, 80)
  assert.equal(out.length, 83)
  assert.ok(out.endsWith('...'))
})

test('truncateExcerpt respects the max argument', () => {
  assert.equal(truncateExcerpt('abcdef', 3), 'abc...')
})
