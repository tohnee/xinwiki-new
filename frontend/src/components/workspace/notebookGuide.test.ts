import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  sourceLabel,
  normalizeQuestion,
  dedupSuggestions,
  limitSuggestions,
  formatSuggestionChips,
  pickFallbackQuestions,
  shouldShowSuggestions,
  SOURCE_LABELS,
  DEFAULT_SUGGESTED_QUESTIONS,
  MAX_SUGGESTION_CHIPS,
  type SuggestionChip,
} from './notebookGuide.ts'
import type { SuggestedQuestion } from '@/api/kb-suggestions.ts'

function makeQ(question: string, source = 'faq'): SuggestedQuestion {
  return { question, source }
}

// --- sourceLabel ---------------------------------------------------------

test('sourceLabel: returns localized label for known sources', () => {
  assert.equal(sourceLabel('faq'), 'FAQ')
  assert.equal(sourceLabel('document'), '文档')
  assert.equal(sourceLabel('wiki'), 'Wiki')
  assert.equal(sourceLabel('agent_config'), '智能体')
})

test('sourceLabel: returns 其他 for empty/undefined/null', () => {
  assert.equal(sourceLabel(''), '其他')
  assert.equal(sourceLabel(undefined), '其他')
  assert.equal(sourceLabel(null), '其他')
})

test('sourceLabel: returns raw code for unknown source (defensive)', () => {
  // New backend sources should still render, not silently collapse.
  assert.equal(sourceLabel('new_source'), 'new_source')
})

test('SOURCE_LABELS: exposes the label map for direct lookup', () => {
  assert.equal(typeof SOURCE_LABELS, 'object')
  assert.equal(SOURCE_LABELS.faq, 'FAQ')
})

// --- normalizeQuestion ---------------------------------------------------

test('normalizeQuestion: trims leading/trailing whitespace', () => {
  assert.equal(normalizeQuestion('  hello  '), 'hello')
})

test('normalizeQuestion: collapses internal whitespace', () => {
  assert.equal(normalizeQuestion('What   is   RAG?'), 'What is RAG?')
  assert.equal(normalizeQuestion('a\tb\n c'), 'a b c')
})

test('normalizeQuestion: returns empty string for null/undefined', () => {
  assert.equal(normalizeQuestion(null), '')
  assert.equal(normalizeQuestion(undefined), '')
  assert.equal(normalizeQuestion(''), '')
})

// --- dedupSuggestions ----------------------------------------------------

test('dedupSuggestions: returns empty array for empty input', () => {
  assert.deepEqual(dedupSuggestions([]), [])
})

test('dedupSuggestions: keeps all unique questions', () => {
  const input = [makeQ('a', 'faq'), makeQ('b', 'wiki')]
  const out = dedupSuggestions(input)
  assert.equal(out.length, 2)
  assert.equal(out[0].question, 'a')
  assert.equal(out[1].question, 'b')
})

test('dedupSuggestions: drops duplicates by normalized text (first wins)', () => {
  const input = [
    makeQ('What is RAG?', 'faq'),
    makeQ('What is  RAG?', 'wiki'), // collapsed-equal to first
    makeQ('What is RAG?', 'document'), // exact dup
  ]
  const out = dedupSuggestions(input)
  assert.equal(out.length, 1)
  assert.equal(out[0].question, 'What is RAG?')
  assert.equal(out[0].source, 'faq') // first occurrence preserved
})

test('dedupSuggestions: drops empty/whitespace-only questions', () => {
  const input = [
    makeQ('', 'faq'),
    makeQ('   ', 'wiki'),
    makeQ('\t\n', 'document'),
    makeQ('real question', 'faq'),
  ]
  const out = dedupSuggestions(input)
  assert.equal(out.length, 1)
  assert.equal(out[0].question, 'real question')
})

test('dedupSuggestions: preserves other fields on the kept object', () => {
  const input = [makeQ('hello', 'wiki')]
  input[0].knowledge_base_id = 'kb-1'
  const out = dedupSuggestions(input)
  assert.equal(out[0].knowledge_base_id, 'kb-1')
})

test('dedupSuggestions: does not mutate the input array', () => {
  const input = [makeQ('a'), makeQ('a')]
  const snapshot = input.map(q => ({ ...q }))
  dedupSuggestions(input)
  assert.deepEqual(input, snapshot)
})

// --- limitSuggestions ----------------------------------------------------

test('limitSuggestions: returns all when under the cap', () => {
  const input = [makeQ('a'), makeQ('b')]
  const out = limitSuggestions(input, 5)
  assert.equal(out.length, 2)
})

test('limitSuggestions: truncates to cap when over', () => {
  const input = [makeQ('a'), makeQ('b'), makeQ('c'), makeQ('d')]
  const out = limitSuggestions(input, 2)
  assert.equal(out.length, 2)
  assert.equal(out[0].question, 'a')
  assert.equal(out[1].question, 'b')
})

test('limitSuggestions: defaults to MAX_SUGGESTION_CHIPS when max omitted', () => {
  const input: SuggestedQuestion[] = []
  for (let i = 0; i < MAX_SUGGESTION_CHIPS + 3; i++) input.push(makeQ(`q${i}`))
  const out = limitSuggestions(input)
  assert.equal(out.length, MAX_SUGGESTION_CHIPS)
})

test('limitSuggestions: falls back to default cap for invalid max', () => {
  const input = [makeQ('a'), makeQ('b'), makeQ('c'), makeQ('d'), makeQ('e'), makeQ('f'), makeQ('g')]
  // Negative, zero, NaN, non-finite all fall back to MAX_SUGGESTION_CHIPS.
  assert.equal(limitSuggestions(input, -1).length, MAX_SUGGESTION_CHIPS)
  assert.equal(limitSuggestions(input, 0).length, MAX_SUGGESTION_CHIPS)
  assert.equal(limitSuggestions(input, NaN).length, MAX_SUGGESTION_CHIPS)
  assert.equal(limitSuggestions(input, Infinity).length, MAX_SUGGESTION_CHIPS)
})

test('limitSuggestions: floors fractional max', () => {
  const input = [makeQ('a'), makeQ('b'), makeQ('c')]
  assert.equal(limitSuggestions(input, 2.9).length, 2)
})

test('limitSuggestions: returns a new array (no mutation)', () => {
  const input = [makeQ('a')]
  const out = limitSuggestions(input, 5)
  assert.notEqual(out, input)
  assert.equal(input.length, 1)
})

// --- formatSuggestionChips ----------------------------------------------

test('formatSuggestionChips: returns empty array for null/undefined/empty', () => {
  assert.deepEqual(formatSuggestionChips(null), [])
  assert.deepEqual(formatSuggestionChips(undefined), [])
  assert.deepEqual(formatSuggestionChips([]), [])
})

test('formatSuggestionChips: maps to chip view-models with sourceLabel', () => {
  const input = [makeQ('hello', 'faq'), makeQ('world', 'wiki')]
  const out = formatSuggestionChips(input)
  assert.equal(out.length, 2)
  assert.equal(out[0].question, 'hello')
  assert.equal(out[0].source, 'faq')
  assert.equal(out[0].sourceLabel, 'FAQ')
  assert.equal(out[1].sourceLabel, 'Wiki')
})

test('formatSuggestionChips: dedup + limit in one pass', () => {
  const input = [
    makeQ('q1', 'faq'),
    makeQ('q1', 'wiki'), // dup
    makeQ('q2', 'document'),
    makeQ('q3', 'wiki'),
    makeQ('q4', 'agent_config'),
  ]
  const out = formatSuggestionChips(input, 3)
  assert.equal(out.length, 3)
  assert.equal(out[0].question, 'q1')
  assert.equal(out[1].question, 'q2')
  assert.equal(out[2].question, 'q3')
})

test('formatSuggestionChips: empty/whitespace questions are dropped', () => {
  const input = [makeQ('   '), makeQ('', 'faq'), makeQ('real', 'wiki')]
  const out = formatSuggestionChips(input)
  assert.equal(out.length, 1)
  assert.equal(out[0].question, 'real')
})

test('formatSuggestionChips: unknown source falls back to raw code in sourceLabel', () => {
  const out = formatSuggestionChips([makeQ('q', 'unknown_src')])
  assert.equal(out[0].sourceLabel, 'unknown_src')
})

test('formatSuggestionChips: empty source renders 其他 label', () => {
  const out = formatSuggestionChips([{ question: 'q', source: '' }])
  assert.equal(out[0].sourceLabel, '其他')
})

test('formatSuggestionChips: default cap is MAX_SUGGESTION_CHIPS', () => {
  const input: SuggestedQuestion[] = []
  for (let i = 0; i < MAX_SUGGESTION_CHIPS + 2; i++) input.push(makeQ(`q${i}`))
  assert.equal(formatSuggestionChips(input).length, MAX_SUGGESTION_CHIPS)
})

// --- pickFallbackQuestions ----------------------------------------------

test('pickFallbackQuestions: returns DEFAULT_SUGGESTED_QUESTIONS as chips', () => {
  const out = pickFallbackQuestions()
  assert.equal(out.length, DEFAULT_SUGGESTED_QUESTIONS.length)
  out.forEach((chip, i) => {
    assert.equal(chip.question, DEFAULT_SUGGESTED_QUESTIONS[i])
    assert.equal(chip.source, '')
    assert.equal(chip.sourceLabel, '')
  })
})

test('pickFallbackQuestions: respects max cap', () => {
  const out = pickFallbackQuestions(2)
  assert.equal(out.length, 2)
  assert.equal(out[0].question, DEFAULT_SUGGESTED_QUESTIONS[0])
})

test('pickFallbackQuestions: falls back to default cap for invalid max', () => {
  const out = pickFallbackQuestions(-5)
  assert.equal(out.length, DEFAULT_SUGGESTED_QUESTIONS.length)
})

test('pickFallbackQuestions: never exceeds DEFAULT_SUGGESTED_QUESTIONS length', () => {
  // Even if max is huge, slice can't return more than the source array.
  const out = pickFallbackQuestions(999)
  assert.equal(out.length, DEFAULT_SUGGESTED_QUESTIONS.length)
})

// --- shouldShowSuggestions ----------------------------------------------

test('shouldShowSuggestions: false when no KB selected', () => {
  assert.equal(shouldShowSuggestions([{
    question: 'q',
    source: 'faq',
    sourceLabel: 'FAQ',
  }], false), false)
})

test('shouldShowSuggestions: false when chip list empty', () => {
  assert.equal(shouldShowSuggestions([], true), false)
})

test('shouldShowSuggestions: true only when KB present and chips non-empty', () => {
  const chips: SuggestionChip[] = [{
    question: 'q',
    source: 'faq',
    sourceLabel: 'FAQ',
  }]
  assert.equal(shouldShowSuggestions(chips, true), true)
})

test('shouldShowSuggestions: false when both KB missing and chips empty', () => {
  assert.equal(shouldShowSuggestions([], false), false)
})

// --- DEFAULT_SUGGESTED_QUESTIONS sanity ---------------------------------

test('DEFAULT_SUGGESTED_QUESTIONS: is a non-empty readonly list', () => {
  assert.ok(Array.isArray(DEFAULT_SUGGESTED_QUESTIONS))
  assert.ok(DEFAULT_SUGGESTED_QUESTIONS.length > 0)
  // Every entry is a non-empty trimmed string.
  for (const q of DEFAULT_SUGGESTED_QUESTIONS) {
    assert.equal(q, q.trim())
    assert.ok(q.length > 0)
  }
})

test('MAX_SUGGESTION_CHIPS: is a positive integer', () => {
  assert.ok(Number.isInteger(MAX_SUGGESTION_CHIPS))
  assert.ok(MAX_SUGGESTION_CHIPS > 0)
})
