// API client for per-user notes (NotebookLM-style "Notes" surface).
//
// The backend authoritatively scopes notes to the active (user, tenant)
// pair from the auth context, so these helpers never send user_id or
// tenant_id. A tenant switch automatically re-points reads at the right
// namespace on the server side.
//
// Wire contract: internal/handler/note.go (5 endpoints under /user/notes).

import { get, post, put, del } from '@/utils/request'

export interface UserNote {
  id: string
  user_id: string
  tenant_id: number
  /** Optional: the chat session this note was saved from. Empty for hand-written notes. */
  session_id?: string
  title: string
  content: string
  /** The cited excerpt the user pinned from chat. Empty for hand-written notes. */
  source_excerpt?: string
  /** Backend id of the cited source (chunk id, wiki page id). */
  source_ref_id?: string
  /** Denormalised title of the cited source, captured at save time. */
  source_title?: string
  /** Denormalised URL of the cited source. */
  source_url?: string
  created_at: string
  updated_at: string
}

export interface CreateNotePayload {
  title: string
  content?: string
  session_id?: string
  source_excerpt?: string
  source_ref_id?: string
  source_title?: string
  source_url?: string
}

export interface UpdateNotePayload {
  title: string
  content: string
}

/** List the current user's notes in the active tenant. Newest first. */
export function listNotes() {
  return get<{ success: boolean; data: UserNote[] }>('/api/v1/user/notes')
}

/** List notes saved from a specific chat session. */
export function listNotesBySession(sessionId: string) {
  return get<{ success: boolean; data: UserNote[] }>(
    `/api/v1/user/notes?session_id=${encodeURIComponent(sessionId)}`,
  )
}

export function getNote(id: string) {
  return get<{ success: boolean; data: UserNote }>(
    `/api/v1/user/notes/${encodeURIComponent(id)}`,
  )
}

export function createNote(payload: CreateNotePayload) {
  return post('/api/v1/user/notes', payload)
}

export function updateNote(id: string, payload: UpdateNotePayload) {
  return put(`/api/v1/user/notes/${encodeURIComponent(id)}`, payload)
}

export function deleteNote(id: string) {
  return del(`/api/v1/user/notes/${encodeURIComponent(id)}`)
}
