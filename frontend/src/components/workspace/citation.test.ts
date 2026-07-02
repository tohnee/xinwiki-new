import { test } from 'node:test'
import assert from 'node:assert/strict'
import { mapReferencePayload, injectInlineCitations, findReferenceByNum, type Reference } from './citation.ts'

test('mapReferencePayload: uses knowledge_title (not title) for the label', () => {
  const raw = { id: 'chunk-1', knowledge_title: 'RAG 概览', content: '片段内容' }
  const r = mapReferencePayload(raw, 0)
  assert.equal(r.title, 'RAG 概览')
  assert.equal(r.num, 1)
  assert.equal(r.id, 'chunk-1')
  assert.equal(r.excerpt, '片段内容')
})

test('mapReferencePayload: falls back to 来源 N when no title fields present', () => {
  const r = mapReferencePayload({ id: 'x' }, 2)
  assert.equal(r.title, '来源 3')
  assert.equal(r.num, 3)
})

test('mapReferencePayload: extracts URL from metadata.url when top-level url is absent', () => {
  const raw = { id: 'c1', knowledge_title: 'T', metadata: { url: 'https://example.com/page' } }
  const r = mapReferencePayload(raw, 0)
  assert.equal(r.url, 'https://example.com/page')
})

test('mapReferencePayload: prefers top-level url over metadata.url', () => {
  const raw = { id: 'c1', url: 'https://top.example.com', metadata: { url: 'https://meta.example.com' } }
  const r = mapReferencePayload(raw, 0)
  assert.equal(r.url, 'https://top.example.com')
})

test('mapReferencePayload: preserves start_at/end_at numeric offsets', () => {
  const raw = { id: 'c1', knowledge_title: 'T', start_at: 120, end_at: 280 }
  const r = mapReferencePayload(raw, 0)
  assert.equal(r.startAt, 120)
  assert.equal(r.endAt, 280)
})

test('mapReferencePayload: leaves startAt undefined when backend sends string', () => {
  const r = mapReferencePayload({ id: 'c1', start_at: 'abc' }, 0)
  assert.equal(r.startAt, undefined)
})

test('injectInlineCitations: converts [1] to <cite> when 1 is in references', () => {
  const refs: Reference[] = [{ id: 'a', num: 1, title: 'A', excerpt: '' }]
  const out = injectInlineCitations('see [1] for details', refs)
  assert.equal(out, 'see <cite class="inline-citation" data-ref-num="1">1</cite> for details')
})

test('injectInlineCitations: leaves [n] alone when n is not in references', () => {
  const refs: Reference[] = [{ id: 'a', num: 1, title: 'A', excerpt: '' }]
  const out = injectInlineCitations('see [2] for details', refs)
  assert.equal(out, 'see [2] for details')
})

test('injectInlineCitations: returns input unchanged when references is empty', () => {
  const out = injectInlineCitations('see [1]', [])
  assert.equal(out, 'see [1]')
})

test('injectInlineCitations: handles multiple citations in one string', () => {
  const refs: Reference[] = [
    { id: 'a', num: 1, title: 'A', excerpt: '' },
    { id: 'b', num: 2, title: 'B', excerpt: '' },
  ]
  const out = injectInlineCitations('[1] and [2] agree, [3] does not', refs)
  assert.ok(out.includes('data-ref-num="1"'))
  assert.ok(out.includes('data-ref-num="2"'))
  assert.ok(!out.includes('data-ref-num="3"'))
})

test('injectInlineCitations: handles empty html', () => {
  const refs: Reference[] = [{ id: 'a', num: 1, title: 'A', excerpt: '' }]
  assert.equal(injectInlineCitations('', refs), '')
})

test('findReferenceByNum: returns matching reference', () => {
  const refs: Reference[] = [
    { id: 'a', num: 1, title: 'A', excerpt: '' },
    { id: 'b', num: 2, title: 'B', excerpt: '' },
  ]
  const r = findReferenceByNum(refs, 2)
  assert.equal(r?.id, 'b')
})

test('findReferenceByNum: returns undefined for out-of-range num', () => {
  const refs: Reference[] = [{ id: 'a', num: 1, title: 'A', excerpt: '' }]
  assert.equal(findReferenceByNum(refs, 99), undefined)
})
