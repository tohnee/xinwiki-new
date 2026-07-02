/**
 * kbStore - shared knowledge-base state for the Workspace surface.
 *
 * P2 fix: Workspace.vue, XinWikiWorkspace.vue and WorkspaceSidebar.vue
 * each used to call `listKnowledgeBases({ creator: 'all' })` on mount,
 * producing three redundant network requests and three independent
 * copies of the same list. This module owns the canonical KB list +
 * active-KB id so all three components share one source of truth.
 *
 * Pure helpers live here so they can be unit-tested without a Vue
 * runtime; the reactive `useKbStore` composable in `useKbStore.ts`
 * wraps them.
 */

export interface KbListItem {
  id: string
  name: string
}

/**
 * Pick the initial active KB id.
 *
 * Resolution order:
 *   1. The KB already marked active by the auth store (covers deep
 *      links from the KB list page where the user explicitly picked
 *      a KB before entering the workspace).
 *   2. The first KB in the list (sensible default for first-time
 *      workspace entry).
 *   3. Empty string (no KB available - callers should fall back to
 *      a "no KB" state).
 *
 * The function never throws: bad inputs (non-array, undefined id)
 * degrade gracefully to ''.
 */
export const pickInitialKbId = (
  currentKbId: string | undefined | null,
  kbList: readonly KbListItem[] | null | undefined,
): string => {
  if (currentKbId && typeof currentKbId === 'string' && currentKbId.length > 0) {
    return currentKbId
  }
  if (Array.isArray(kbList) && kbList.length > 0 && kbList[0]?.id) {
    return kbList[0].id
  }
  return ''
}

/**
 * Resolve the display name for a KB id. Returns the fallback when the
 * id is empty or the KB is not in the list - the Workspace header
 * renders this directly, so we must never surface `undefined`.
 */
export const resolveKbName = (
  kbId: string | undefined | null,
  kbList: readonly KbListItem[] | null | undefined,
  fallback = '选择知识库',
): string => {
  if (!kbId || !Array.isArray(kbList)) return fallback
  const hit = kbList.find(k => k.id === kbId)
  return hit?.name || fallback
}

/**
 * Normalise the raw `listKnowledgeBases` API response into a flat
 * `KbListItem[]`. The response shape has drifted across versions
 * (`data` array vs `knowledge_bases` array vs bare array), so we
 * tolerate all three.
 */
export const normalizeKbList = (raw: any): KbListItem[] => {
  if (!raw) return []
  const list = raw?.data || raw?.knowledge_bases || raw?.items || raw
  if (!Array.isArray(list)) return []
  return list
    .filter((kb: any) => kb && typeof kb === 'object' && kb.id)
    .map((kb: any) => ({
      id: String(kb.id),
      name: String(kb.name ?? kb.title ?? '(未命名)'),
    }))
}
