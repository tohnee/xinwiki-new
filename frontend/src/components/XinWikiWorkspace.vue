<script setup lang="ts">
import { onMounted } from 'vue'
import WorkspaceHeader from './workspace/WorkspaceHeader.vue'
import WorkspaceSidebar from './workspace/WorkspaceSidebar.vue'
import WorkspaceContent from './workspace/WorkspaceContent.vue'
import WorkspaceRightPanel from './workspace/WorkspaceRightPanel.vue'
import { useWorkspaceLayout } from './workspace/useWorkspaceLayout'
import { useGeneration } from './workspace/useGeneration'
import { useKbStore } from './workspace/useKbStore'
import { ref } from 'vue'
import type { WikiPage } from '@/api/wiki'

const props = defineProps<{
  selectedPageId?: string
  /**
   * Breadcrumb trail for the content header. P1 fix: previously the
   * outer Workspace.vue computed this but never sent it down, so
   * WorkspaceContent always received `[]` and rendered an empty
   * breadcrumb. Now the parent owns it and we just forward it.
   */
  breadcrumb?: string[]
}>()

const emit = defineEmits<{
  (e: 'select-page', page: WikiPage | null): void
  (e: 'select-search-result', result: { id: string; title: string; type: string }): void
  /**
   * P1 fix: bubble up the sidebar's "新建" click so the parent can
   * route to the wiki page editor.
   */
  (e: 'create-page'): void
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

// P2 fix: KB list + active KB id are owned by the shared kbStore so
// Workspace.vue / XinWikiWorkspace.vue / WorkspaceSidebar.vue no longer
// each fire their own listKnowledgeBases() call on mount.
const { activeKbName, ensureLoaded } = useKbStore()

const searchQuery = ref('')

onMounted(() => {
  void ensureLoaded()
})

const handlePageSelect = (page: WikiPage | null) => {
  emit('select-page', page)
}

const handleSearchResult = (result: { id: string; title: string; type: string }) => {
  emit('select-search-result', result)
}

const handleCreatePage = () => {
  emit('create-page')
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
        :selected-page-id="selectedPageId"
        @toggle="toggleSidebar"
        @select-page="handlePageSelect"
        @create-page="handleCreatePage"
      />

      <WorkspaceContent :breadcrumb="props.breadcrumb ?? ['XinWiki', '智能问答']">
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
