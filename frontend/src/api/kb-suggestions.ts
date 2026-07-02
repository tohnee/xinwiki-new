// API client for the KB-level suggested-questions endpoint (NotebookLM
// "Notebook Guide" feature). Mirrors the backend types.SuggestedQuestion
// shape returned by GET /api/v1/knowledge-bases/:id/suggested-questions.

import { get } from '@/utils/request'

export interface SuggestedQuestion {
  question: string
  /** "faq" | "document" | "wiki" | "agent_config" */
  source: string
  knowledge_base_id?: string
}

export interface SuggestedQuestionsResponse {
  success: boolean
  data: {
    questions: SuggestedQuestion[]
  }
}

/**
 * Fetch NotebookLM-style suggested questions for a single knowledge base.
 * Returns an empty array on error so the caller can fall back to defaults
 * without try/catch noise.
 *
 * The underlying `get` helper returns the parsed Axios response payload
 * directly, so the response shape mirrors SuggestedQuestionsResponse.
 */
export async function getKBSuggestedQuestions(
  kbId: string,
  opts?: { knowledgeIds?: string[]; limit?: number },
): Promise<SuggestedQuestion[]> {
  const params: Record<string, string> = {}
  if (opts?.knowledgeIds && opts.knowledgeIds.length > 0) {
    params.knowledge_ids = opts.knowledgeIds.join(',')
  }
  if (opts?.limit && opts.limit > 0) {
    params.limit = String(opts.limit)
  }
  const res = await get<SuggestedQuestionsResponse>(
    `/api/v1/knowledge-bases/${encodeURIComponent(kbId)}/suggested-questions`,
    { params },
  )
  // The request helper may return either the raw Axios response or the
  // already-unwrapped data payload depending on interceptor config; the
  // optional chaining covers both without throwing when fields are absent.
  const data = (res as any)?.data ?? res
  return data?.data?.questions ?? []
}
