<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import {
  AddIcon,
  StickyNoteIcon,
  DeleteIcon,
  LinkIcon,
} from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'
import {
  listNotesBySession,
  listNotes,
  createNote,
  deleteNote,
  type UserNote,
  type CreateNotePayload,
} from '@/api/user-notes'
import {
  formatNotesCount,
  displayTitle,
  displayExcerpt,
  hasSource,
  validateNoteTitle,
  validateNoteContent,
  buildCreateFromExcerpt,
} from './notesPanel'

// sessionId scopes the list to the current chat session. When undefined,
// the panel shows all the user's notes in the tenant - this is the
// fallback for the standalone "笔记" tab usage.
const props = defineProps<{
  sessionId?: string
}>()

// Emits:
// - `excerpt-saved` fires after a successful create so the parent
//   (WorkspaceChat) can close its "save to notes" affordance.
// - `error` lets the parent surface a toast if it owns the toast root.
const emit = defineEmits<{
  (e: 'excerpt-saved', note: UserNote): void
  (e: 'error', message: string): void
}>()

const notes = ref<UserNote[]>([])
const loading = ref(false)
const showCreateForm = ref(false)
const newTitle = ref('')
const newContent = ref('')
const creating = ref(false)
const titleError = ref('')
const contentError = ref('')

const countLabel = computed(() => formatNotesCount(notes.value))

async function refresh() {
  loading.value = true
  try {
    const api = props.sessionId ? listNotesBySession(props.sessionId) : listNotes()
    const res = await api
    notes.value = (res as any)?.data ?? []
  } catch (e: any) {
    const msg = e?.message || '加载笔记失败'
    MessagePlugin.error(msg)
    emit('error', msg)
  } finally {
    loading.value = false
  }
}

function openCreateForm() {
  showCreateForm.value = true
  newTitle.value = ''
  newContent.value = ''
  titleError.value = ''
  contentError.value = ''
}

function cancelCreate() {
  showCreateForm.value = false
}

async function submitCreate() {
  titleError.value = validateNoteTitle(newTitle.value)
  if (titleError.value) return
  contentError.value = validateNoteContent(newContent.value)
  if (contentError.value) return

  creating.value = true
  try {
    const payload: CreateNotePayload = {
      title: newTitle.value.trim(),
      content: newContent.value,
      session_id: props.sessionId,
    }
    const res = await createNote(payload)
    const note = (res as any)?.data as UserNote | undefined
    if (note) {
      notes.value.unshift(note)
      emit('excerpt-saved', note)
    }
    showCreateForm.value = false
    MessagePlugin.success('笔记已保存')
  } catch (e: any) {
    const msg = e?.message || '保存失败'
    MessagePlugin.error(msg)
    emit('error', msg)
  } finally {
    creating.value = false
  }
}

async function removeNote(id: string) {
  try {
    await deleteNote(id)
    notes.value = notes.value.filter(n => n.id !== id)
    MessagePlugin.success('已删除')
  } catch (e: any) {
    const msg = e?.message || '删除失败'
    MessagePlugin.error(msg)
    emit('error', msg)
  }
}

function openSource(note: UserNote) {
  if (note.source_url) {
    window.open(note.source_url, '_blank', 'noopener,noreferrer')
  }
}

// saveExcerpt is the imperative entry point the parent calls when the user
// picks "save to notes" from a chat citation. Exposed via defineExpose so
// WorkspaceChat / XinWikiWorkspace can call notesPanelRef.value?.saveExcerpt(...).
async function saveExcerpt(args: {
  excerpt: string
  sourceRefId?: string
  sourceTitle?: string
  sourceUrl?: string
  title?: string
}) {
  const payload = buildCreateFromExcerpt({
    ...args,
    sessionId: props.sessionId,
  })
  try {
    const res = await createNote(payload)
    const note = (res as any)?.data as UserNote | undefined
    if (note) {
      notes.value.unshift(note)
      emit('excerpt-saved', note)
      MessagePlugin.success('已保存到笔记')
    }
  } catch (e: any) {
    const msg = e?.message || '保存失败'
    MessagePlugin.error(msg)
    emit('error', msg)
  }
}

defineExpose({ saveExcerpt, refresh })

onMounted(() => {
  void refresh()
})
</script>

<template>
  <div class="panel-content">
    <div class="notes-header">
      <span class="notes-count">{{ countLabel }}</span>
      <button class="add-note-button" @click="openCreateForm">
        <AddIcon size="small" />
        新建笔记
      </button>
    </div>

    <!-- Create form: inline, collapses after submit. -->
    <div v-if="showCreateForm" class="note-create-form">
      <input
        v-model="newTitle"
        class="note-input note-title-input"
        placeholder="笔记标题"
        maxlength="255"
        @input="titleError = ''"
      />
      <p v-if="titleError" class="note-field-error">{{ titleError }}</p>
      <textarea
        v-model="newContent"
        class="note-input note-content-input"
        placeholder="正文（可选）"
        rows="4"
        @input="contentError = ''"
      />
      <p v-if="contentError" class="note-field-error">{{ contentError }}</p>
      <div class="note-form-actions">
        <button class="note-btn note-btn-cancel" @click="cancelCreate" :disabled="creating">
          取消
        </button>
        <button class="note-btn note-btn-submit" @click="submitCreate" :disabled="creating">
          {{ creating ? '保存中…' : '保存' }}
        </button>
      </div>
    </div>

    <div v-if="loading && notes.length === 0" class="notes-loading">
      <p>加载中…</p>
    </div>

    <div v-else-if="notes.length > 0" class="notes-list">
      <div v-for="note in notes" :key="note.id" class="note-item">
        <div class="note-item-main">
          <div class="note-item-title" :title="note.title">{{ displayTitle(note) }}</div>
          <div v-if="displayExcerpt(note)" class="note-item-excerpt">{{ displayExcerpt(note) }}</div>
          <div class="note-item-meta">
            <span v-if="hasSource(note)" class="note-source-link" @click="openSource(note)">
              <LinkIcon size="small" />
              <span>{{ note.source_title || '查看来源' }}</span>
            </span>
            <span class="note-updated">{{ note.updated_at?.slice(0, 10) }}</span>
          </div>
        </div>
        <button
          class="note-delete-btn"
          title="删除"
          @click="removeNote(note.id)"
        >
          <DeleteIcon size="small" />
        </button>
      </div>
    </div>

    <div v-else class="notes-empty">
      <StickyNoteIcon class="empty-icon" />
      <p class="empty-text">暂无笔记</p>
      <p class="empty-hint">可在对话中保存引用片段，或点击"新建笔记"手动添加</p>
    </div>
  </div>
</template>

<style lang="less" scoped>
.panel-content {
  padding: 20px 16px;
}

.notes-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}

.notes-count {
  font-size: 13px;
  font-weight: 500;
  color: #86868b;
}

.add-note-button {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 6px 12px;
  font-size: 13px;
  font-weight: 500;
  color: #007aff;
  background: rgba(0, 122, 255, 0.08);
  border: none;
  border-radius: 8px;
  cursor: pointer;

  &:hover {
    background: rgba(0, 122, 255, 0.15);
  }
}

.note-create-form {
  margin-bottom: 16px;
  padding: 12px;
  background: rgba(0, 0, 0, 0.02);
  border-radius: 10px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.note-input {
  width: 100%;
  padding: 8px 10px;
  font-size: 13px;
  border: 1px solid rgba(0, 0, 0, 0.1);
  border-radius: 8px;
  background: white;
  box-sizing: border-box;
  font-family: inherit;

  &:focus {
    outline: none;
    border-color: #007aff;
    box-shadow: 0 0 0 3px rgba(0, 122, 255, 0.1);
  }
}

.note-title-input {
  font-weight: 500;
}

.note-content-input {
  resize: vertical;
  min-height: 60px;
  font-family: inherit;
}

.note-field-error {
  margin: 0;
  font-size: 12px;
  color: #ff3b30;
}

.note-form-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.note-btn {
  padding: 6px 14px;
  font-size: 13px;
  border: none;
  border-radius: 8px;
  cursor: pointer;

  &:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
}

.note-btn-cancel {
  background: rgba(0, 0, 0, 0.05);
  color: #86868b;

  &:hover:not(:disabled) {
    background: rgba(0, 0, 0, 0.08);
  }
}

.note-btn-submit {
  background: #007aff;
  color: white;

  &:hover:not(:disabled) {
    background: #0066d6;
  }
}

.notes-loading {
  padding: 40px 16px;
  text-align: center;
  color: #86868b;
  font-size: 13px;
}

.notes-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.note-item {
  display: flex;
  gap: 8px;
  padding: 12px;
  background: rgba(0, 0, 0, 0.02);
  border-radius: 10px;
  transition: background 0.15s ease;

  &:hover {
    background: rgba(0, 0, 0, 0.04);
  }
}

.note-item-main {
  flex: 1;
  min-width: 0;
}

.note-item-title {
  font-size: 14px;
  font-weight: 500;
  color: #1d1d1f;
  margin-bottom: 4px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.note-item-excerpt {
  font-size: 12px;
  color: #86868b;
  line-height: 1.5;
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
  margin-bottom: 6px;
}

.note-item-meta {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 11px;
  color: #aeaeb2;
}

.note-source-link {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  color: #007aff;
  cursor: pointer;

  &:hover {
    text-decoration: underline;
  }
}

.note-updated {
  font-variant-numeric: tabular-nums;
}

.note-delete-btn {
  flex-shrink: 0;
  width: 28px;
  height: 28px;
  border: none;
  background: transparent;
  border-radius: 6px;
  color: #aeaeb2;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.15s ease;

  &:hover {
    background: rgba(255, 59, 48, 0.1);
    color: #ff3b30;
  }
}

.notes-empty {
  padding: 40px 16px;
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;

  .empty-icon {
    font-size: 32px;
    color: #c7c7cc;
    margin-bottom: 12px;
  }

  .empty-text {
    font-size: 14px;
    color: #86868b;
    margin: 0 0 4px 0;
    font-weight: 500;
  }

  .empty-hint {
    font-size: 12px;
    color: #aeaeb2;
    margin: 0;
    line-height: 1.5;
  }
}
</style>
