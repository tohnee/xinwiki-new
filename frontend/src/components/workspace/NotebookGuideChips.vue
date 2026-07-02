<script setup lang="ts">
// NotebookGuideChips renders the NotebookLM-style suggested-questions block
// in the chat empty state. It fetches suggestions from the backend when a
// knowledge base is selected, falls back to a default question list when
// the KB has no indexed content, and emits a `select` event with the chosen
// question text so the parent (WorkspaceChat) can drop it into the input.
//
// Pure helpers + view-model shaping live in notebookGuide.ts; this file
// owns only the network fetch + the rendering loop.

import { ref, watch, onMounted } from 'vue'
import { LightbulbIcon } from 'tdesign-icons-vue-next'
import { getKBSuggestedQuestions } from '@/api/kb-suggestions'
import {
  formatSuggestionChips,
  pickFallbackQuestions,
  shouldShowSuggestions,
  MAX_SUGGESTION_CHIPS,
  type SuggestionChip,
} from './notebookGuide'

const props = defineProps<{
  /** Active knowledge base ID. When undefined, the block is hidden. */
  knowledgeBaseId?: string
  /** Optional knowledge filter (e.g. a selected document subset). */
  knowledgeIds?: string[]
  /** Max chips to render. Defaults to MAX_SUGGESTION_CHIPS. */
  max?: number
}>()

const emit = defineEmits<{
  (e: 'select', question: string): void
}>()

const chips = ref<SuggestionChip[]>([])
const loading = ref(false)
const loaded = ref(false)

const maxChips = () =>
  Number.isFinite(props.max as number) && (props.max as number) > 0
    ? Math.floor(props.max as number)
    : MAX_SUGGESTION_CHIPS

const visible = () => shouldShowSuggestions(chips.value, !!props.knowledgeBaseId)

/**
 * fetchSuggestions loads KB-backed suggestions. On any error or empty result
 * it falls back to the default question list so the empty state is never
 * bare. Network errors are swallowed: the fallback guarantees a renderable
 * chip set without a try/catch at every call site.
 */
async function fetchSuggestions() {
  if (!props.knowledgeBaseId) {
    chips.value = []
    loaded.value = false
    return
  }
  loading.value = true
  try {
    const raw = await getKBSuggestedQuestions(props.knowledgeBaseId, {
      knowledgeIds: props.knowledgeIds,
      limit: maxChips(),
    })
    const formatted = formatSuggestionChips(raw, maxChips())
    chips.value = formatted.length > 0 ? formatted : pickFallbackQuestions(maxChips())
  } catch {
    // Network / 5xx / auth: fall back rather than surfacing an error in
    // the empty state. The chat input itself still works.
    chips.value = pickFallbackQuestions(maxChips())
  } finally {
    loading.value = false
    loaded.value = true
  }
}

const onSelect = (chip: SuggestionChip) => {
  emit('select', chip.question)
}

onMounted(fetchSuggestions)

// Re-fetch when the KB changes (user picks another KB in the header).
watch(
  () => props.knowledgeBaseId,
  () => fetchSuggestions(),
)
</script>

<template>
  <div v-if="visible()" class="notebook-guide" :class="{ loading }">
    <div class="guide-header">
      <LightbulbIcon class="guide-icon" />
      <span class="guide-title">建议问题</span>
      <span v-if="loading" class="guide-hint">加载中…</span>
    </div>
    <div class="guide-chips">
      <button
        v-for="(chip, i) in chips"
        :key="i"
        class="guide-chip"
        :title="chip.sourceLabel ? `来自 ${chip.sourceLabel}` : '默认建议'"
        @click="onSelect(chip)"
      >
        <span class="chip-question">{{ chip.question }}</span>
        <span v-if="chip.sourceLabel" class="chip-source">{{ chip.sourceLabel }}</span>
      </button>
    </div>
  </div>
</template>

<style lang="less" scoped>
.notebook-guide {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 14px;
  max-width: 640px;
  margin: 0 auto;
  padding: 8px 0 4px;

  &.loading {
    opacity: 0.7;
  }
}

.guide-header {
  display: flex;
  align-items: center;
  gap: 6px;
  color: #86868b;
  font-size: 13px;
  user-select: none;
}
.guide-icon {
  color: #ff9500;
}
.guide-title {
  font-weight: 500;
  letter-spacing: 0.02em;
}
.guide-hint {
  color: #c7c7cc;
  font-size: 12px;
  margin-left: 4px;
}

.guide-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: center;
}

.guide-chip {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 8px 14px;
  background: rgba(0, 122, 255, 0.06);
  border: 1px solid rgba(0, 122, 255, 0.15);
  border-radius: 16px;
  color: #007aff;
  font-size: 13px;
  cursor: pointer;
  transition: all 0.15s ease;
  text-align: left;

  &:hover {
    background: rgba(0, 122, 255, 0.12);
    transform: translateY(-1px);
    border-color: rgba(0, 122, 255, 0.28);
  }

  &:active {
    transform: translateY(0);
  }
}

.chip-question {
  line-height: 1.4;
}

.chip-source {
  flex-shrink: 0;
  padding: 1px 6px;
  background: rgba(0, 122, 255, 0.1);
  border-radius: 8px;
  font-size: 10px;
  font-weight: 600;
  color: #007aff;
  letter-spacing: 0.02em;
  text-transform: uppercase;
}
</style>
