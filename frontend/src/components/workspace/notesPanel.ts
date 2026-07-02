// Pure helpers for the Notes panel. Kept side-effect-free so they can be
// unit-tested with node:test without mounting any Vue component.
//
// The Vue component (NotesPanel.vue) imports these via the named exports
// below; tests live in notesPanel.test.ts.

import type { UserNote } from '@/api/user-notes'

/** Maximum title length the backend accepts (see internal/application/service/note.go). */
export const MAX_NOTE_TITLE_LEN = 255
/** Maximum content length the backend accepts. */
export const MAX_NOTE_CONTENT_LEN = 65535

/**
 * formatNotesCount returns the count label shown in the panel header.
 * Returns "暂无笔记" for an empty list so the empty state isn't ambiguous.
 */
export function formatNotesCount(notes: UserNote[]): string {
  const n = notes.length
  if (n === 0) return '暂无笔记'
  return `${n} 条笔记`
}

/**
 * truncate clamps a string to `max` characters and appends an ellipsis when
 * truncation actually happened. Returns the input unchanged for empty / short
 * strings. Used for both note titles and excerpts in the list view.
 */
export function truncate(text: string, max: number): string {
  if (!text) return ''
  // Array.from counts code points (not UTF-16 units), so emojis and CJK
  // characters are counted as one each - matches the backend's
  // utf8.RuneCountInString validation.
  const chars = Array.from(text)
  if (chars.length <= max) return text
  return chars.slice(0, max).join('') + '…'
}

/**
 * displayTitle returns the title to show in the list. Falls back to a
 * truncated content snippet when the title is empty - this matches
 * NotebookLM's behaviour where a note saved from a citation may have no
 * user-typed title.
 */
export function displayTitle(note: UserNote): string {
  const t = (note.title ?? '').trim()
  if (t) return truncate(t, 60)
  const c = (note.content ?? '').trim()
  if (c) return truncate(c, 60)
  return '无标题'
}

/**
 * displayExcerpt returns the secondary line shown under the title. Prefers
 * source_excerpt (the pinned citation snippet), then falls back to content.
 */
export function displayExcerpt(note: UserNote): string {
  const s = (note.source_excerpt ?? '').trim()
  if (s) return truncate(s, 120)
  const c = (note.content ?? '').trim()
  if (c) return truncate(c, 120)
  return ''
}

/**
 * hasSource returns true when the note was saved from a chat citation
 * (as opposed to hand-written). Used by the UI to show a "查看来源" link.
 */
export function hasSource(note: UserNote): boolean {
  return !!(note.source_ref_id || note.source_url || note.source_title)
}

/**
 * validateNoteTitle mirrors the server-side validation in
 * internal/application/service/note.go so the form can fail fast without
 * a round-trip. Returns an error message string, or '' when valid.
 */
export function validateNoteTitle(title: string): string {
  const t = (title ?? '').trim()
  if (!t) return '标题不能为空'
  if (Array.from(t).length > MAX_NOTE_TITLE_LEN) {
    return `标题不能超过 ${MAX_NOTE_TITLE_LEN} 个字符`
  }
  return ''
}

/**
 * validateNoteContent mirrors the server-side content length cap. Empty
 * content is allowed (a note can be just a pinned excerpt).
 */
export function validateNoteContent(content: string): string {
  if (Array.from(content ?? '').length > MAX_NOTE_CONTENT_LEN) {
    return `内容不能超过 ${MAX_NOTE_CONTENT_LEN} 个字符`
  }
  return ''
}

/**
 * buildCreateFromExcerpt constructs a CreateNotePayload from a pinned chat
 * citation. The title defaults to the source title so the user has
 * something to edit; the excerpt is preserved verbatim in source_excerpt.
 */
export function buildCreateFromExcerpt(args: {
  title?: string
  excerpt: string
  sourceRefId?: string
  sourceTitle?: string
  sourceUrl?: string
  sessionId?: string
}): {
  title: string
  content: string
  source_excerpt: string
  source_ref_id: string
  source_title: string
  source_url: string
  session_id: string
} {
  const title = (args.title ?? args.sourceTitle ?? '').trim() || '保存的引用'
  return {
    title,
    content: '',
    source_excerpt: args.excerpt ?? '',
    source_ref_id: args.sourceRefId ?? '',
    source_title: args.sourceTitle ?? '',
    source_url: args.sourceUrl ?? '',
    session_id: args.sessionId ?? '',
  }
}
