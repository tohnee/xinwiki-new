<script setup lang="ts">
import { useSlots } from 'vue'
import WorkspaceHeader from './workspace/WorkspaceHeader.vue'
import WorkspaceSidebar from './workspace/WorkspaceSidebar.vue'
import WorkspaceContent from './workspace/WorkspaceContent.vue'
import WorkspaceRightPanel from './workspace/WorkspaceRightPanel.vue'
import { useWorkspaceLayout } from './workspace/useWorkspaceLayout'
import { useGeneration } from './workspace/useGeneration'
import { ref, computed } from 'vue'
import type { WikiPage } from '@/api/wiki'
import { listKnowledgeBases } from '@/api/knowledge-base'
import { useAuthStore } from '@/stores/auth'

const slots = useSlots()
const authStore = useAuthStore()

const emit = defineEmits<{
  (e: 'select-page', page: WikiPage | null): void
  (e: 'select-search-result', result: { id: string; title: string; type: string }): void
}>()

const {
  sidebarCollapsed,
  rightPanelVisible,
  rightPanelTab,
  isMobile,
  toggleSidebar,
  toggleRightPanel,
} = useWorkspaceLayout()

const {
  generateInput,
  isGenerating,
  generationType,
  generatedContent,
  generatedCitations,
  generationTypes,
  sampleThinkingSteps,
  generationStatus,
  currentArtifact,
  generationError,
  isDownloadable,
  artifactDownloadUrl,
  handleGenerate,
  resetGeneration,
} = useGeneration()

const searchQuery = ref('')
const kbList = ref<Array<{ id: string; name: string }>>([])
const activeKbId = ref<string>('')
const activeKbName = computed(() => {
  return kbList.value.find(k => k.id === activeKbId.value)?.name || '选择知识库'
})

const loadKBs = async () => {
  try {
    const res = await listKnowledgeBases({ creator: 'all' }) as any
    const list = res?.data || res?.knowledge_bases || []
    kbList.value = Array.isArray(list)
      ? list.map((kb: any) => ({ id: kb.id, name: kb.name }))
      : []
    const current = authStore.currentKnowledgeBase?.id
    activeKbId.value = current || (kbList.value[0]?.id || '')
  } catch (e) {
    console.warn('[workspace] load KBs failed', e)
  }
}
loadKBs()

const handlePageSelect = (page: WikiPage | null) => {
  emit('select-page', page)
}

const handleSearchResult = (result: { id: string; title: string; type: string }) => {
  emit('select-search-result', result)
}
</script>

<template>
  <div class="xinwiki-workspace">
    <WorkspaceHeader
      :sidebar-collapsed="sidebarCollapsed"
      :right-panel-visible="rightPanelVisible"
      :search-query="searchQuery"
      :kb-name="activeKbName"
      @toggle-sidebar="toggleSidebar"
      @toggle-right-panel="toggleRightPanel"
      @update:search-query="searchQuery = $event"
      @select-search-result="handleSearchResult"
    />

    <div class="workspace-body">
      <WorkspaceSidebar
        :collapsed="sidebarCollapsed"
        :is-mobile="isMobile"
        @toggle="toggleSidebar"
        @select-page="handlePageSelect"
      />

      <WorkspaceContent :show-welcome="!slots.default">
        <template #header-actions>
          <slot name="header-actions" />
        </template>
        <slot />
      </WorkspaceContent>

      <WorkspaceRightPanel
        :visible="rightPanelVisible"
        :active-tab="rightPanelTab"
        :is-mobile="isMobile"
        :is-generating="isGenerating"
        :generated-content="generatedContent"
        :generated-citations="generatedCitations"
        :sample-thinking-steps="sampleThinkingSteps"
        :generate-input="generateInput"
        :generation-type="generationType"
        :generation-types="generationTypes"
        :generation-status="generationStatus"
        :current-artifact="currentArtifact"
        :generation-error="generationError"
        :is-downloadable="isDownloadable"
        :artifact-download-url="artifactDownloadUrl"
        @toggle="toggleRightPanel"
        @update:active-tab="rightPanelTab = $event"
        @update:generate-input="generateInput = $event"
        @update:generation-type="generationType = ($event as any)"
        @generate="handleGenerate"
        @reset="resetGeneration"
      />
    </div>
  </div>
</template>

<style lang="less" scoped>
.xinwiki-workspace {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100vh;
  background: linear-gradient(180deg, #f5f7fa 0%, #e8edf5 100%);
  font-family: -apple-system, BlinkMacSystemFont, 'SF Pro Display', 'SF Pro Text', 'Helvetica Neue', Arial, sans-serif;
  overflow: hidden;
}

.workspace-body {
  flex: 1;
  display: flex;
  min-height: 0;
  overflow: hidden;
}
</style>
