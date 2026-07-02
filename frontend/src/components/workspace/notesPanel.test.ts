import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  formatNotesCount,
  truncate,
  displayTitle,
  displayExcerpt,
  hasSource,
  validateNoteTitle,
  validateNoteContent,
  buildCreateFromExcerpt,
  MAX_NOTE_TITLE_LEN,
  MAX_NOTE_CONTENT_LEN,
} from './notesPanel.ts'
import type { UserNote } from '@/api/user-notes.ts'

function makeNote(over: Partial<UserNote> = {}): UserNote {
  return {
    id: 'n1',
    user_id: 'u1',
    tenant_id: 1,
    title: 'default title',
    content: 'default content',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...over,
  }
}

test('formatNotesCount: 暂无笔记 when empty', () => {
  assert.equal(formatNotesCount([]), '暂无笔记')
})

test('formatNotesCount: counts notes when non-empty', () => {
  assert.equal(formatNotesCount([makeNote()]), '1 条笔记')
  assert.equal(formatNotesCount([makeNote(), makeNote(), makeNote()]), '3 条笔记')
})

test('truncate: returns empty string for empty input', () => {
  assert.equal(truncate('', 10), '')
  assert.equal(truncate(undefined as unknown as string, 10), '')
})

test('truncate: returns input unchanged when shorter than max', () => {
  assert.equal(truncate('hello', 10), 'hello')
})

test('truncate: appends ellipsis when truncating', () => {
  assert.equal(truncate('abcdefghij', 5), 'abcde…')
})

test('truncate: counts code points, not UTF-16 units (emoji + CJK safe)', () => {
  // '🚀' is 2 UTF-16 units but 1 code point. Truncating at 3 code points
  // must keep the emoji + 2 chars, then ellipsis.
  assert.equal(truncate('🚀ab文字', 3), '🚀ab…')
})

test('displayTitle: returns truncated title when present', () => {
  const n = makeNote({ title: 'A'.repeat(80) })
  assert.equal(displayTitle(n).length, 61) // 60 chars + '…'
})

test('displayTitle: falls back to content when title is empty', () => {
  const n = makeNote({ title: '', content: 'some content here' })
  assert.equal(displayTitle(n), 'some content here')
})

test('displayTitle: falls back to "无标题" when both title and content empty', () => {
  const n = makeNote({ title: '   ', content: '' })
  assert.equal(displayTitle(n), '无标题')
})

test('displayExcerpt: prefers source_excerpt over content', () => {
  const n = makeNote({ source_excerpt: 'pinned snippet', content: 'body text' })
  assert.equal(displayExcerpt(n), 'pinned snippet')
})

test('displayExcerpt: falls back to content when no source_excerpt', () => {
  const n = makeNote({ content: 'body text' })
  assert.equal(displayExcerpt(n), 'body text')
})

test('displayExcerpt: returns empty string when nothing to show', () => {
  const n = makeNote({ content: '', source_excerpt: '' })
  assert.equal(displayExcerpt(n), '')
})

test('hasSource: true when source_ref_id present', () => {
  assert.equal(hasSource(makeNote({ source_ref_id: 'chunk-1' })), true)
})

test('hasSource: true when source_url present', () => {
  assert.equal(hasSource(makeNote({ source_url: 'https://x' })), true)
})

test('hasSource: false for hand-written note', () => {
  assert.equal(hasSource(makeNote()), false)
})

test('validateNoteTitle: rejects empty/whitespace title', () => {
  assert.equal(validateNoteTitle(''), '标题不能为空')
  assert.equal(validateNoteTitle('   '), '标题不能为空')
  assert.equal(validateNoteTitle('\n\t'), '标题不能为空')
})

test('validateNoteTitle: accepts normal title', () => {
  assert.equal(validateNoteTitle('hello'), '')
  assert.equal(validateNoteTitle(' 中文标题 '), '')
})

test('validateNoteTitle: rejects title exceeding cap (rune-count aware)', () => {
  const long = 'a'.repeat(MAX_NOTE_TITLE_LEN + 1)
  assert.equal(validateNoteTitle(long), `标题不能超过 ${MAX_NOTE_TITLE_LEN} 个字符`)
})

test('validateNoteContent: accepts empty content', () => {
  assert.equal(validateNoteContent(''), '')
})

test('validateNoteContent: rejects content exceeding cap', () => {
  const big = 'x'.repeat(MAX_NOTE_CONTENT_LEN + 1)
  assert.equal(validateNoteContent(big), `内容不能超过 ${MAX_NOTE_CONTENT_LEN} 个字符`)
})

test('buildCreateFromExcerpt: uses provided title when given', () => {
  const p = buildCreateFromExcerpt({
    title: 'my title',
    excerpt: 'snippet',
    sourceRefId: 'c1',
    sourceTitle: 'Source Doc',
    sourceUrl: 'https://x',
    sessionId: 's1',
  })
  assert.equal(p.title, 'my title')
  assert.equal(p.source_excerpt, 'snippet')
  assert.equal(p.source_ref_id, 'c1')
  assert.equal(p.source_title, 'Source Doc')
  assert.equal(p.source_url, 'https://x')
  assert.equal(p.session_id, 's1')
  assert.equal(p.content, '')
})

test('buildCreateFromExcerpt: defaults title to sourceTitle when title omitted', () => {
  const p = buildCreateFromExcerpt({
    excerpt: 'snippet',
    sourceTitle: 'Source Doc',
  })
  assert.equal(p.title, 'Source Doc')
})

test('buildCreateFromExcerpt: falls back to 保存的引用 when both title and sourceTitle empty', () => {
  const p = buildCreateFromExcerpt({ excerpt: 'snippet' })
  assert.equal(p.title, '保存的引用')
})

test('buildCreateFromExcerpt: trims whitespace from title', () => {
  const p = buildCreateFromExcerpt({ title: '   spaced   ', excerpt: 'x' })
  assert.equal(p.title, 'spaced')
})
