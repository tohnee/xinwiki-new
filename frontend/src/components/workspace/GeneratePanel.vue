<script setup lang="ts">
import { computed } from 'vue'
import {
  StarFilledIcon,
  SendIcon,
  CheckCircleFilledIcon,
  CloseCircleFilledIcon,
  DownloadIcon,
  RefreshIcon,
} from 'tdesign-icons-vue-next'
import type { GenerationType, Citation } from './useGeneration'
import type { Artifact, GenerationStatus } from '@/api/artifact'
import { sanitizeMarkdownHTML } from '@/utils/security'

const props = defineProps<{
  generateInput: string
  generationType: string
  isGenerating: boolean
  generatedContent: string
  generatedCitations: Citation[]
  generationTypes: GenerationType[]
  generationStatus?: GenerationStatus
  currentArtifact?: Artifact | null
  generationError?: string
  isDownloadable?: boolean
  artifactDownloadUrl?: string
}>()

const emit = defineEmits<{
  (e: 'update:generateInput', value: string): void
  (e: 'update:generationType', value: string): void
  (e: 'generate'): void
  (e: 'reset'): void
}>()

const renderedContent = computed(() => {
  if (!props.generatedContent) return ''
  try {
    const md = props.generatedContent
    return sanitizeMarkdownHTML(simpleMarkdownToHtml(md))
  } catch {
    return escapeHtml(props.generatedContent)
  }
})

const currentTypeName = () => props.generationTypes.find(t => t.id === props.generationType)?.name

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

function simpleMarkdownToHtml(md: string): string {
  let html = escapeHtml(md)
  html = html.replace(/^### (.*?)$/gm, '<h4>$1</h4>')
  html = html.replace(/^## (.*?)$/gm, '<h3>$1</h3>')
  html = html.replace(/^# (.*?)$/gm, '<h2>$1</h2>')
  html = html.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
  html = html.replace(/\*(.+?)\*/g, '<em>$1</em>')
  html = html.replace(/`([^`]+)`/g, '<code>$1</code>')
  html = html.replace(/\n/g, '<br>')
  return html
}

const copyContent = async () => {
  try {
    await window.navigator.clipboard.writeText(props.generatedContent)
  } catch {
    const ta = document.createElement('textarea')
    ta.value = props.generatedContent
    document.body.appendChild(ta)
    ta.select()
    document.execCommand('copy')
    document.body.removeChild(ta)
  }
}

const statusText = computed(() => {
  switch (props.generationStatus) {
    case 'generating': return '正在生成中...'
    case 'ready': return '生成完成'
    case 'failed': return '生成失败'
    default: return ''
  }
})
</script>

<template>
  <div class="panel-content">
    <div class="generate-types">
      <button
        v-for="type in generationTypes"
        :key="type.id"
        class="type-card"
        :class="{ selected: generationType === type.id, disabled: isGenerating }"
        :disabled="isGenerating"
        @click="emit('update:generationType', type.id)"
      >
        <component :is="type.icon" class="type-icon" />
        <span class="type-name">{{ type.name }}</span>
      </button>
    </div>

    <div class="generate-input-container">
      <textarea
        :value="generateInput"
        @input="emit('update:generateInput', ($event.target as HTMLTextAreaElement).value)"
        placeholder="输入指令，例如：总结这篇文档的核心要点..."
        class="generate-input"
        rows="3"
        :disabled="isGenerating"
        @keydown.meta.enter="emit('generate')"
        @keydown.ctrl.enter="emit('generate')"
      />
      <button
        class="generate-button"
        :disabled="!generateInput.trim() || isGenerating"
        @click="emit('generate')"
      >
        <SendIcon v-if="!isGenerating" />
        <div v-else class="loading-spinner small" />
      </button>
    </div>

    <div v-if="isGenerating" class="status-bar generating">
      <div class="loading-spinner tiny" />
      <span>{{ statusText }}</span>
    </div>

    <div v-else-if="generationStatus === 'failed'" class="status-bar failed">
      <CloseCircleFilledIcon class="status-icon error" />
      <span>{{ generationError || '生成失败' }}</span>
      <button class="status-action" @click="emit('generate')">
        <RefreshIcon size="small" />重试
      </button>
    </div>

    <div v-else-if="generationStatus === 'ready' && currentArtifact" class="status-bar ready">
      <CheckCircleFilledIcon class="status-icon success" />
      <span>{{ statusText }}</span>
      <button class="status-action" @click="emit('reset')">新建</button>
    </div>

    <div v-if="generatedContent || generatedCitations.length" class="generated-content">
      <div class="content-header-bar">
        <span class="content-title">{{ currentTypeName() }}</span>
        <div class="content-actions">
          <button
            v-if="generatedContent"
            class="action-button"
            @click="copyContent"
          >
            复制
          </button>
          <a
            v-if="isDownloadable && artifactDownloadUrl"
            class="action-button"
            :href="artifactDownloadUrl"
            target="_blank"
            rel="noopener"
          >
            <DownloadIcon size="small" style="vertical-align: -2px; margin-right: 2px;" />下载
          </a>
        </div>
      </div>
      <div v-if="generatedContent" class="content-body markdown-body" v-html="renderedContent" />
      <div v-else-if="isDownloadable" class="download-placeholder">
        <DownloadIcon class="download-icon" size="32" />
        <p>文件已生成，点击上方下载按钮获取</p>
      </div>

      <div v-if="generatedCitations.length" class="citations-section">
        <div class="citations-title">引用来源</div>
        <div v-for="citation in generatedCitations" :key="citation.id" class="citation-item">
          <div class="citation-number">{{ citation.id }}</div>
          <div class="citation-content">
            <div class="citation-title">{{ citation.title }}</div>
            <div class="citation-excerpt">{{ citation.excerpt }}</div>
          </div>
        </div>
      </div>
    </div>

    <div v-else-if="!isGenerating && generationStatus !== 'failed'" class="empty-state">
      <StarFilledIcon class="empty-icon" />
      <h4 class="empty-title">AI 智能生成</h4>
      <p class="empty-description">
        NotebookLM 风格的智能生成面板，支持一键生成总结、简报、FAQ、思维导图等多种内容
      </p>
    </div>
  </div>
</template>

<style lang="less" scoped>
.panel-content {
  padding: 20px 16px;
}

.generate-types {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 8px;
  margin-bottom: 16px;
}

.type-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 12px 8px;
  background: rgba(0, 0, 0, 0.02);
  border: 1px solid transparent;
  border-radius: 12px;
  cursor: pointer;
  transition: all 0.2s cubic-bezier(0.25, 0.1, 0.25, 1);

  &:hover:not(.disabled) {
    background: rgba(0, 122, 255, 0.04);
    border-color: rgba(0, 122, 255, 0.2);
    transform: translateY(-1px);
  }

  &.selected {
    background: rgba(0, 122, 255, 0.08);
    border-color: #007aff;
  }

  &.disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }
}

.type-icon {
  font-size: 22px;
  color: #007aff;
}

.type-name {
  font-size: 12px;
  font-weight: 500;
  color: #1d1d1f;
  text-align: center;
}

.generate-input-container {
  position: relative;
  margin-bottom: 12px;
}

.generate-input {
  width: 100%;
  padding: 12px 48px 12px 16px;
  border: 1px solid rgba(0, 0, 0, 0.1);
  border-radius: 12px;
  font-size: 14px;
  font-family: inherit;
  resize: none;
  background: rgba(255, 255, 255, 0.8);
  transition: all 0.2s ease;
  outline: none;

  &:focus {
    border-color: #007aff;
    box-shadow: 0 0 0 4px rgba(0, 122, 255, 0.1);
    background: white;
  }

  &:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  &::placeholder {
    color: #86868b;
  }
}

.generate-button {
  position: absolute;
  right: 8px;
  bottom: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border: none;
  background: linear-gradient(135deg, #007aff 0%, #5856d6 100%);
  color: white;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.15s ease;

  &:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  &:not(:disabled):hover {
    transform: scale(1.05);
  }
}

.loading-spinner {
  width: 16px;
  height: 16px;
  border: 2px solid rgba(255, 255, 255, 0.3);
  border-top-color: white;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;

  &.small { width: 14px; height: 14px; }
  &.tiny { width: 12px; height: 12px; border-width: 2px; }
}

@keyframes spin { to { transform: rotate(360deg); } }

.status-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-radius: 8px;
  font-size: 13px;
  margin-bottom: 12px;

  &.generating {
    color: #007aff;
    background: rgba(0, 122, 255, 0.08);
  }

  &.ready {
    color: #34c759;
    background: rgba(52, 199, 89, 0.08);
  }

  &.failed {
    color: #ff3b30;
    background: rgba(255, 59, 48, 0.08);
  }

  .status-icon {
    flex-shrink: 0;
    &.success { color: #34c759; }
    &.error { color: #ff3b30; }
  }

  span { flex: 1; }
}

.status-action {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 8px;
  font-size: 12px;
  font-weight: 500;
  color: #007aff;
  background: rgba(0, 122, 255, 0.1);
  border: none;
  border-radius: 6px;
  cursor: pointer;

  &:hover {
    background: rgba(0, 122, 255, 0.18);
  }
}

.generated-content {
  background: white;
  border-radius: 12px;
  border: 1px solid rgba(0, 0, 0, 0.06);
  overflow: hidden;
}

.content-header-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  border-bottom: 1px solid rgba(0, 0, 0, 0.06);
  background: rgba(0, 0, 0, 0.02);
}

.content-title {
  font-size: 14px;
  font-weight: 600;
  color: #1d1d1f;
}

.content-actions {
  display: flex;
  gap: 8px;
}

.action-button {
  display: inline-flex;
  align-items: center;
  padding: 4px 10px;
  font-size: 12px;
  font-weight: 500;
  color: #007aff;
  background: rgba(0, 122, 255, 0.08);
  border: none;
  border-radius: 6px;
  cursor: pointer;
  text-decoration: none;
  transition: all 0.15s ease;

  &:hover {
    background: rgba(0, 122, 255, 0.15);
  }
}

.content-body {
  padding: 16px;
  font-size: 14px;
  line-height: 1.6;
  color: #1d1d1f;

  :deep(h4) { margin: 0 0 12px 0; font-size: 16px; font-weight: 600; color: #1d1d1f; }
  :deep(h3) { margin: 16px 0 10px 0; font-size: 17px; font-weight: 600; color: #1d1d1f; }
  :deep(h2) { margin: 18px 0 10px 0; font-size: 18px; font-weight: 600; color: #1d1d1f; }
  :deep(strong) { font-weight: 600; }
  :deep(em) { font-style: italic; }
  :deep(code) {
    background: #f5f5f7;
    padding: 2px 5px;
    border-radius: 4px;
    font-family: -apple-system, 'SF Mono', Monaco, monospace;
    font-size: 0.9em;
  }
}

.download-placeholder {
  padding: 40px 20px;
  text-align: center;
  color: #86868b;

  .download-icon {
    color: #007aff;
    margin-bottom: 12px;
  }

  p {
    margin: 0;
    font-size: 14px;
  }
}

.citations-section {
  padding: 12px 16px;
  border-top: 1px solid rgba(0, 0, 0, 0.06);
  background: rgba(0, 0, 0, 0.01);
}

.citations-title {
  font-size: 12px;
  font-weight: 600;
  color: #86868b;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  margin-bottom: 8px;
}

.citation-item {
  display: flex;
  gap: 10px;
  padding: 8px 0;

  &:not(:last-child) {
    border-bottom: 1px solid rgba(0, 0, 0, 0.04);
  }
}

.citation-number {
  flex-shrink: 0;
  width: 20px;
  height: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 11px;
  font-weight: 600;
  color: #007aff;
  background: rgba(0, 122, 255, 0.1);
  border-radius: 50%;
}

.citation-content {
  flex: 1;
  min-width: 0;
}

.citation-title {
  font-size: 13px;
  font-weight: 500;
  color: #1d1d1f;
  margin-bottom: 2px;
}

.citation-excerpt {
  font-size: 12px;
  color: #86868b;
  line-height: 1.4;
  overflow: hidden;
  text-overflow: ellipsis;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  padding: 40px 20px;
}

.empty-icon {
  font-size: 48px;
  color: #c7c7cc;
  margin-bottom: 16px;
}

.empty-title {
  font-size: 17px;
  font-weight: 600;
  color: #1d1d1f;
  margin: 0 0 8px 0;
}

.empty-description {
  font-size: 14px;
  color: #86868b;
  margin: 0;
  line-height: 1.5;
}
</style>
