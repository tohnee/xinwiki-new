<script setup lang="ts">
import { computed } from 'vue'
import {
  StarFilledIcon,
  FileTxtIcon,
  TipsIcon,
  StickyNoteIcon,
  CloseIcon,
} from 'tdesign-icons-vue-next'
import GeneratePanel from './GeneratePanel.vue'
import SourcesPanel from './SourcesPanel.vue'
import ThinkingPanel from './ThinkingPanel.vue'
import NotesPanel from './NotesPanel.vue'
import type { GenerationType, Citation, ThinkingStep } from './useGeneration'
import type { Artifact, GenerationStatus } from '@/api/artifact'
import {
  resolvePanelSurfaceState,
  mobileFabAriaLabel,
  mobileDrawerCloseAriaLabel,
} from './mobilePanel'

type TabId = 'generate' | 'sources' | 'thinking' | 'notes'

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
  /** Optional: chat session id, forwarded to NotesPanel so notes scope to the current chat. */
  sessionId?: string
}>()

const emit = defineEmits<{
  (e: 'toggle'): void
  (e: 'update:activeTab', value: TabId): void
  (e: 'update:generateInput', value: string): void
  (e: 'update:generationType', value: string): void
  (e: 'generate'): void
  (e: 'reset'): void
}>()

// P2 fix: the panel used to gate everything on `!isMobile`, which made
// the right panel completely unreachable on phones. Compute which
// surfaces should render via the shared, unit-tested helper so the
// branch logic lives in one place.
const panelState = computed(() =>
  resolvePanelSurfaceState(props.isMobile, props.visible),
)
</script>

<template>
  <!-- Desktop side panel (always-visible when expanded) -->
  <aside v-if="panelState.showDesktopPanel" class="right-panel">
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
        <button
          class="panel-tab"
          :class="{ active: activeTab === 'notes' }"
          @click="emit('update:activeTab', 'notes')"
        >
          <StickyNoteIcon />
          <span class="tab-label">笔记</span>
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
      <SourcesPanel v-else-if="activeTab === 'sources'" :citations="generatedCitations" />
      <ThinkingPanel
        v-else-if="activeTab === 'thinking'"
        :sample-thinking-steps="sampleThinkingSteps"
      />
      <NotesPanel v-else-if="activeTab === 'notes'" :session-id="sessionId" />
    </div>
  </aside>

  <!-- Desktop collapse handle (when panel is hidden) -->
  <button
    v-if="panelState.showDesktopHandle"
    class="panel-collapse-handle"
    @click="emit('toggle')"
    title="打开生成面板"
  >
    <StarFilledIcon />
  </button>

  <!-- P2 fix: mobile drawer overlay. Previously the right panel was
       completely unreachable on phones because both v-if branches
       excluded isMobile. The drawer slides in from the right and dims
       the background; tapping the backdrop or the close button emits
       `toggle` to collapse it. -->
  <Teleport to="body">
    <div v-if="panelState.showMobileDrawer" class="mobile-drawer-root">
      <div class="mobile-drawer-backdrop" @click="emit('toggle')" />
      <aside class="mobile-drawer" role="dialog" aria-modal="true">
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
            <button
              class="panel-tab"
              :class="{ active: activeTab === 'notes' }"
              @click="emit('update:activeTab', 'notes')"
            >
              <StickyNoteIcon />
              <span class="tab-label">笔记</span>
            </button>
          </div>
          <button
            class="icon-button panel-close"
            @click="emit('toggle')"
            :aria-label="mobileDrawerCloseAriaLabel"
            title="关闭生成面板"
          >
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
          <SourcesPanel v-else-if="activeTab === 'sources'" :citations="generatedCitations" />
          <ThinkingPanel
            v-else-if="activeTab === 'thinking'"
            :sample-thinking-steps="sampleThinkingSteps"
          />
          <NotesPanel v-else-if="activeTab === 'notes'" :session-id="sessionId" />
        </div>
      </aside>
    </div>
  </Teleport>

  <!-- P2 fix: mobile floating action button. Visible whenever the
       drawer is closed on a phone; tapping it emits `toggle` to slide
       the drawer in. -->
  <button
    v-if="panelState.showMobileFab"
    class="mobile-panel-fab"
    @click="emit('toggle')"
    :aria-label="mobileFabAriaLabel"
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

/* P2 fix: mobile FAB lives inside the component subtree (not teleported),
   so scoped styles apply normally. */
.mobile-panel-fab {
  position: fixed;
  right: 16px;
  bottom: 16px;
  width: 48px;
  height: 48px;
  border-radius: 50%;
  border: none;
  background: #007aff;
  color: white;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 4px 16px rgba(0, 122, 255, 0.35);
  cursor: pointer;
  z-index: 1000;
  transition: transform 0.15s ease, box-shadow 0.15s ease;

  &:hover {
    transform: scale(1.05);
    box-shadow: 0 6px 20px rgba(0, 122, 255, 0.45);
  }

  &:active {
    transform: scale(0.95);
  }
}
</style>

<!-- P2 fix: the mobile drawer is Teleported to <body>, so its styles
     must NOT be scoped - scoped styles only apply to elements rendered
     inside the component's own subtree. This block duplicates the
     panel-header/tabs/scroll rules for the teleported drawer. -->
<style lang="less">
.mobile-drawer-root {
  position: fixed;
  inset: 0;
  z-index: 1100;
}

.mobile-drawer-backdrop {
  position: absolute;
  inset: 0;
  background: rgba(0, 0, 0, 0.4);
  animation: mobileFadeIn 0.2s ease;
}

.mobile-drawer {
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  width: 88vw;
  max-width: 380px;
  display: flex;
  flex-direction: column;
  background: rgba(255, 255, 255, 0.96);
  backdrop-filter: saturate(180%) blur(20px);
  -webkit-backdrop-filter: saturate(180%) blur(20px);
  box-shadow: -4px 0 24px rgba(0, 0, 0, 0.12);
  animation: mobileSlideIn 0.25s cubic-bezier(0.25, 0.1, 0.25, 1);
}

@keyframes mobileFadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

@keyframes mobileSlideIn {
  from { transform: translateX(100%); }
  to { transform: translateX(0); }
}

.mobile-drawer .panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  border-bottom: 1px solid rgba(0, 0, 0, 0.06);
  flex-shrink: 0;
}

.mobile-drawer .panel-tabs {
  display: flex;
  gap: 4px;
  background: rgba(0, 0, 0, 0.04);
  padding: 3px;
  border-radius: 8px;
}

.mobile-drawer .panel-tab {
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
}

.mobile-drawer .panel-tab.active {
  background: white;
  color: #007aff;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
}

.mobile-drawer .icon-button {
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
}

.mobile-drawer .panel-close {
  width: 28px;
  height: 28px;
}

.mobile-drawer .panel-scroll {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  -webkit-overflow-scrolling: touch;
}

.mobile-drawer .panel-scroll::-webkit-scrollbar {
  width: 6px;
}

.mobile-drawer .panel-scroll::-webkit-scrollbar-thumb {
  background: rgba(0, 0, 0, 0.15);
  border-radius: 3px;
}
</style>
