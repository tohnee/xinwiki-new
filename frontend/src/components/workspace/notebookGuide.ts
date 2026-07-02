// Pure helpers for the NotebookLM-style "Notebook Guide" suggested-questions
// surface. Kept side-effect-free so they can be unit-tested with node:test
// without mounting any Vue component or hitting the network.
//
// The Vue layer (WorkspaceChat.vue empty state) imports these via the named
// exports below; tests live in notebookGuide.test.ts.

import type { SuggestedQuestion } from '@/api/kb-suggestions'

/**
 * Source codes returned by the backend (see internal/types/custom_agent.go
 * SuggestedQuestion.Source). Mapped to localized labels for chip rendering.
 */
export const SOURCE_LABELS: Record<string, string> = {
  faq: 'FAQ',
  document: '文档',
  wiki: 'Wiki',
  agent_config: '智能体',
}

/**
 * Default fallback questions shown when the KB has no indexed content yet
 * (or the suggestion endpoint is unavailable). Mirrors NotebookLM, which
 * always shows *something* in the empty state rather than a blank panel.
 */
export const DEFAULT_SUGGESTED_QUESTIONS: readonly string[] = [
  '帮我总结一下这个知识库的核心内容',
  '有哪些关键概念需要重点关注？',
  '请列出主要的知识点和它们之间的关系',
]

/** Maximum chips to render in the empty state. Keeps the panel tidy. */
export const MAX_SUGGESTION_CHIPS = 6

/**
 * sourceLabel returns a human-readable label for a suggestion source code.
 * Falls back to the raw code when unknown so new sources still render
 * (defensive: the backend may add sources before the frontend ships an
 * updated label map).
 */
export function sourceLabel(source: string | undefined | null): string {
  if (!source) return '其他'
  return SOURCE_LABELS[source] ?? source
}

/**
 * normalizeQuestion trims and collapses internal whitespace so that
 * "What is  RAG?" and "What is RAG?" compare equal during dedup.
 */
export function normalizeQuestion(q: string | undefined | null): string {
  return (q ?? '').trim().replace(/\s+/g, ' ')
}

/**
 * dedupSuggestions removes duplicate questions by normalized text. The first
 * occurrence wins, preserving the round-robin diversity produced by the
 * backend. Empty/whitespace-only questions are also dropped here so the
 * rendering layer never has to guard against blank chips.
 */
export function dedupSuggestions(questions: readonly SuggestedQuestion[]): SuggestedQuestion[] {
  const seen = new Set<string>()
  const out: SuggestedQuestion[] = []
  for (const q of questions) {
    const norm = normalizeQuestion(q.question)
    if (!norm) continue
    if (seen.has(norm)) continue
    seen.add(norm)
    out.push({ ...q, question: norm })
  }
  return out
}

/**
 * limitSuggestions truncates a suggestion list to at most `max` entries.
 * Falls back to MAX_SUGGESTION_CHIPS when max is not a positive integer so
 * callers can pass `limit` straight from a query param without sanitizing.
 */
export function limitSuggestions(
  questions: readonly SuggestedQuestion[],
  max: number = MAX_SUGGESTION_CHIPS,
): SuggestedQuestion[] {
  const cap = Number.isFinite(max) && max > 0 ? Math.floor(max) : MAX_SUGGESTION_CHIPS
  if (questions.length <= cap) return [...questions]
  return questions.slice(0, cap)
}

/**
 * SuggestionChip is the view-model rendered by the empty-state chips. The
 * question is normalized; sourceLabel is precomputed so the template can
 * render a chip with a single object access.
 */
export interface SuggestionChip {
  question: string
  source: string
  sourceLabel: string
}

/**
 * formatSuggestionChips turns a raw SuggestedQuestion[] into the chip
 * view-model list. It runs dedup + limit in one pass and precomputes the
 * source label. This is the single entry point the Vue layer should call:
 * it never throws and always returns a (possibly empty) array.
 */
export function formatSuggestionChips(
  questions: readonly SuggestedQuestion[] | null | undefined,
  max: number = MAX_SUGGESTION_CHIPS,
): SuggestionChip[] {
  if (!Array.isArray(questions) || questions.length === 0) return []
  const cleaned = dedupSuggestions(questions)
  const capped = limitSuggestions(cleaned, max)
  return capped.map(q => ({
    question: q.question,
    source: q.source ?? '',
    sourceLabel: sourceLabel(q.source),
  }))
}

/**
 * pickFallbackQuestions returns the default question list when the backend
 * returned nothing. Returns the raw strings (not chips) because the
 * fallbacks have no source - the caller wraps them as label-less chips.
 *
 * `max` mirrors formatSuggestionChips' cap so the fallback list never
 * overflows the panel even if a caller shrinks the budget.
 */
export function pickFallbackQuestions(
  max: number = MAX_SUGGESTION_CHIPS,
): SuggestionChip[] {
  const cap = Number.isFinite(max) && max > 0 ? Math.floor(max) : MAX_SUGGESTION_CHIPS
  return DEFAULT_SUGGESTED_QUESTIONS
    .slice(0, cap)
    .map(q => ({ question: q, source: '', sourceLabel: '' }))
}

/**
 * shouldShowSuggestions decides whether the chip block should render at all.
 * NotebookLM hides the block entirely when there is nothing to show (rather
 * than rendering an empty "Suggested questions" header that implies the
 * feature is broken).
 *
 * `hasKb` is forwarded so the caller can suppress suggestions until a KB is
 * selected - showing "summarize this knowledge base" with no KB selected
 * would be misleading.
 */
export function shouldShowSuggestions(
  chips: readonly SuggestionChip[],
  hasKb: boolean,
): boolean {
  if (!hasKb) return false
  return chips.length > 0
}
