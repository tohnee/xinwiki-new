/**
 * Pure helpers for SourcesPanel.
 *
 * Extracted so the source-count label and the empty-state text can be
 * unit-tested without mounting Vue. SourcesPanel imports these and
 * feeds its props through them.
 */

import type { Citation } from './useGeneration'

/**
 * Build the "N 个知识源" label for the sources panel header.
 *
 * P1 bug: previously the panel always showed "12 个知识源" with a
 * hard-coded mock list, regardless of what was actually retrieved.
 * Now the count reflects the real citations passed in.
 */
export const formatSourcesCount = (citations: Citation[]): string => {
  const n = citations.length
  if (n === 0) return '暂无知识源'
  return `${n} 个知识源`
}

/**
 * Short preview of an excerpt, capped at `max` chars with an ellipsis.
 * Avoids layout blow-ups when a citation excerpt is very long.
 */
export const truncateExcerpt = (excerpt: string, max = 80): string => {
  if (!excerpt) return ''
  if (excerpt.length <= max) return excerpt
  return excerpt.slice(0, max) + '...'
}
