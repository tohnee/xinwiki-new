/**
 * Pure helpers for inline citation rendering in WorkspaceChat.
 *
 * Extracted from the Vue component so the field mapping and [n] token
 * parsing can be unit-tested without mounting the component. The chat
 * panel imports these and feeds the SSE reference payload + rendered
 * markdown through them.
 */

export interface Reference {
  /** Stable chunk/source identifier from the backend (SearchResult.id). */
  id: string
  /** 1-based citation number the LLM was instructed to emit as [1], [2]. */
  num: number
  /** Human-readable label (knowledge title, wiki page title, web title). */
  title: string
  /** Source excerpt used in hover cards and the sources panel. */
  excerpt: string
  /** Optional routable URL (web source URL or wiki page path). */
  url?: string
  /** Backend knowledge ID, for "open in knowledge base" navigation. */
  knowledgeId?: string
  /** Source category: "knowledge_chunk" | "wiki_page" | "web_search". */
  sourceType?: string
  /** Char offset range in the source document, for excerpt highlighting. */
  startAt?: number
  endAt?: number
}

/**
 * Map a raw SSE reference payload (SearchResult-shaped) to the frontend
 * Reference type. The backend SearchResult has 26+ fields; this helper
 * picks the ones the UI needs and applies the correct fallback chain.
 *
 * P0 fix: previously the component mapped `r.title || r.name || r.doc_title`
 * which always fell through to "来源 N" because the backend field is
 * `knowledge_title`. Same bug for `url` (real value is `metadata.url`)
 * and `excerpt` (real value is `content`). This helper centralizes the
 * correct mapping so the bug cannot recur.
 */
export const mapReferencePayload = (raw: any, index: number): Reference => {
  const num = index + 1
  const id = String(raw?.id ?? raw?.knowledge_id ?? raw?.chunk_id ?? num)
  const title =
    raw?.knowledge_title ||
    raw?.title ||
    raw?.wiki_page_title ||
    raw?.name ||
    raw?.doc_title ||
    `来源 ${num}`
  const excerpt = raw?.excerpt || raw?.content || raw?.snippet || ''
  const url = raw?.url || raw?.metadata?.url || raw?.source_url || undefined
  const knowledgeId = raw?.knowledge_id || raw?.knowledgeBaseId || undefined
  const sourceType = raw?.chunk_type || raw?.source_type || raw?.type || 'knowledge_chunk'
  const startAt = typeof raw?.start_at === 'number' ? raw.start_at : undefined
  const endAt = typeof raw?.end_at === 'number' ? raw.end_at : undefined
  return { id, num, title, excerpt, url, knowledgeId, sourceType, startAt, endAt }
}

/**
 * Replace `[1]`, `[2]`, ... tokens in already-escaped HTML with clickable
 * `<cite>` chips. Only numbers that exist in `references` are converted;
 * other `[n]` tokens (e.g. array indices in code blocks) are left alone.
 *
 * The emitted markup is intentionally minimal and relies on the parent
 * component's CSS for styling. `data-ref-num` is the contract between
 * the rendered HTML and the click/hover handlers attached after v-html.
 */
export const injectInlineCitations = (html: string, references: Reference[]): string => {
  if (!html || references.length === 0) return html
  const validNums = new Set(references.map(r => r.num))
  // Match [n] where n is a positive integer, but not inside tag attributes
  // (cheap heuristic: the char before [ is not a letter/quote, avoiding
  // matches like data-x="[1]" while still catching prose "[1]").
  return html.replace(/\[(\d+)\]/g, (match, numStr) => {
    const n = parseInt(numStr, 10)
    if (!validNums.has(n)) return match
    return `<cite class="inline-citation" data-ref-num="${n}">${n}</cite>`
  })
}

/**
 * Find the reference whose num matches the data-ref-num attribute on a
 * clicked citation chip. Returns undefined when the number is out of range
 * (defensive against stale DOM after streaming updates).
 */
export const findReferenceByNum = (references: Reference[], num: number): Reference | undefined => {
  return references.find(r => r.num === num)
}
