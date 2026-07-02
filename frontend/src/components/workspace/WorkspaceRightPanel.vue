<script setup lang="ts">
import {
  StarFilledIcon,
  FileTxtIcon,
  TipsIcon,
  CloseIcon,
} from 'tdesign-icons-vue-next'
import GeneratePanel from './GeneratePanel.vue'
import SourcesPanel from './SourcesPanel.vue'
import ThinkingPanel from './ThinkingPanel.vue'
import type { GenerationType, Citation, ThinkingStep } from './useGeneration'
import type { Artifact, GenerationStatus } from '@/api/artifact'

type TabId = 'generate' | 'sources' | 'thinking'

const props = defineProps<{
  visible: boolean
  activeTab: TabId
  isMobile: boolean
  isGenerating: boolean
  generatedContent: string
  generatedCitations: Citation[]
  sampleThinkingSteps: ThinkingStep[]
  generateInput: string
  generationType: string
  generationTypes: GenerationType[]
  generationStatus?: GenerationStatus
  currentArtifact?: Artifact | null
  generationError?: string
  isDownloadable?: boolean
  artifactDownloadUrl?: string
}>()

const emit = defineEmits<{
  (e: 'toggle'): void
  (e: 'update:activeTab', value: TabId): void
  (e: 'update:generateInput', value: string): void
  (e: 'update:generationType', value: string): void
  (e: 'generate'): void
  (e: 'reset'): void
}>()
</script>

<template>
  <aside v-if="visible && !isMobile" class="right-panel">
    <div class="panel-header">
      <div class="panel-tabs">
        <button
          class="panel-tab"
          :class="{ active: activeTab === 'generate' }"
          @click="emit('update:activeTab', 'generate')"
        >
          <StarFilledIcon />
          <span class="tab-label">生成</span>
        </button>
        <button
          class="panel-tab"
          :class="{ active: activeTab === 'sources' }"
          @click="emit('update:activeTab', 'sources')"
        >
          <FileTxtIcon />
          <span class="tab-label">来源</span>
        </button>
        <button
          class="panel-tab"
          :class="{ active: activeTab === 'thinking' }"
          @click="emit('update:activeTab', 'thinking')"
        >
          <TipsIcon />
          <span class="tab-label">思维链</span>
        </button>
      </div>
      <button class="icon-button panel-close" @click="emit('toggle')" title="关闭面板">
        <CloseIcon size="small" />
      </button>
    </div>

    <div class="panel-scroll">
      <GeneratePanel
        v-if="activeTab === 'generate'"
        :generate-input="generateInput"
        :generation-type="generationType"
        :is-generating="isGenerating"
        :generated-content="generatedContent"
        :generated-citations="generatedCitations"
        :generation-types="generationTypes"
        :generation-status="generationStatus"
        :current-artifact="currentArtifact"
        :generation-error="generationError"
        :is-downloadable="isDownloadable"
        :artifact-download-url="artifactDownloadUrl"
        @update:generate-input="emit('update:generateInput', $event)"
        @update:generation-type="emit('update:generationType', $event)"
        @generate="emit('generate')"
        @reset="emit('reset')"
      />
      <SourcesPanel v-else-if="activeTab === 'sources'" />
      <ThinkingPanel
        v-else-if="activeTab === 'thinking'"
        :sample-thinking-steps="sampleThinkingSteps"
      />
    </div>
  </aside>

  <button
    v-if="!visible && !isMobile"
    class="panel-collapse-handle"
    @click="emit('toggle')"
    title="打开生成面板"
  >
    <StarFilledIcon />
  </button>
</template>

<style lang="less" scoped>
.right-panel {
  width: 380px;
  display: flex;
  flex-direction: column;
  background: rgba(255, 255, 255, 0.8);
  backdrop-filter: saturate(180%) blur(20px);
  -webkit-backdrop-filter: saturate(180%) blur(20px);
  border-left: 1px solid rgba(0, 0, 0, 0.06);
  flex-shrink: 0;
  animation: slideIn 0.3s cubic-bezier(0.25, 0.1, 0.25, 1);
}

@keyframes slideIn {
  from {
    opacity: 0;
    transform: translateX(20px);
  }
  to {
    opacity: 1;
    transform: translateX(0);
  }
}

.panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  border-bottom: 1px solid rgba(0, 0, 0, 0.06);
  flex-shrink: 0;
}

.panel-tabs {
  display: flex;
  gap: 4px;
  background: rgba(0, 0, 0, 0.04);
  padding: 3px;
  border-radius: 8px;
}

.panel-tab {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border: none;
  background: transparent;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 500;
  color: #86868b;
  cursor: pointer;
  transition: all 0.15s ease;

  &:hover {
    color: #1d1d1f;
  }

  &.active {
    background: white;
    color: #007aff;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
  }
}

.icon-button {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border: none;
  background: transparent;
  border-radius: 8px;
  cursor: pointer;
  color: #1d1d1f;
  transition: all 0.15s ease;

  &:hover {
    background: rgba(0, 0, 0, 0.06);
  }

  &:active {
    transform: scale(0.95);
  }
}

.panel-close {
  width: 28px;
  height: 28px;
}

.panel-scroll {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;

  &::-webkit-scrollbar {
    width: 6px;
  }

  &::-webkit-scrollbar-track {
    background: transparent;
  }

  &::-webkit-scrollbar-thumb {
    background: rgba(0, 0, 0, 0.15);
    border-radius: 3px;
  }
}

.panel-collapse-handle {
  width: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.8);
  border: 1px solid rgba(0, 0, 0, 0.06);
  border-left: none;
  border-radius: 0 8px 8px 0;
  color: #86868b;
  cursor: pointer;
  writing-mode: vertical-rl;
  padding: 12px 0;
  transition: all 0.2s ease;

  &:hover {
    color: #007aff;
    background: white;
  }
}

@media (max-width: 1280px) {
  .right-panel {
    width: 340px;
  }
}
</style>
