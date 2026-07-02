<script setup lang="ts">
import {
  AddIcon,
  FileTxtIcon,
} from 'tdesign-icons-vue-next'
import type { Citation } from './useGeneration'
import { formatSourcesCount, truncateExcerpt } from './sourcesPanel'

// P1 fix: previously this panel rendered a hard-coded mock list
// ("文档 1..5", "8+n 个分块") with no real data behind it. Now it
// accepts the citations produced by useGeneration() and renders
// them. When no citations exist, it shows an empty-state instead
// of fake numbers.
defineProps<{
  citations?: Citation[]
}>()

const emit = defineEmits<{
  (e: 'add-source'): void
}>()
</script>

<template>
  <div class="panel-content">
    <div class="sources-header">
      <span class="sources-count">{{ formatSourcesCount(citations ?? []) }}</span>
      <button class="add-source-button" @click="emit('add-source')">
        <AddIcon size="small" />
        添加来源
      </button>
    </div>

    <div v-if="citations && citations.length > 0" class="sources-list">
      <div
        v-for="c in citations"
        :key="c.id"
        class="source-item"
        :title="c.title"
      >
        <FileTxtIcon class="source-icon" />
        <div class="source-info">
          <div class="source-title">{{ c.title }}</div>
          <div class="source-meta">{{ truncateExcerpt(c.excerpt) }}</div>
        </div>
      </div>
    </div>

    <div v-else class="sources-empty">
      <FileTxtIcon class="empty-icon" />
      <p class="empty-text">生成内容后，引用的知识源将显示在这里</p>
    </div>
  </div>
</template>

<style lang="less" scoped>
.panel-content {
  padding: 20px 16px;
}

.sources-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}

.sources-count {
  font-size: 13px;
  font-weight: 500;
  color: #86868b;
}

.add-source-button {
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

.sources-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.source-item {
  display: flex;
  gap: 10px;
  padding: 12px;
  background: rgba(0, 0, 0, 0.02);
  border-radius: 10px;
  cursor: pointer;
  transition: all 0.15s ease;

  &:hover {
    background: rgba(0, 0, 0, 0.04);
  }
}

.source-icon {
  font-size: 18px;
  color: #86868b;
  flex-shrink: 0;
  margin-top: 2px;
}

.source-info {
  flex: 1;
  min-width: 0;
}

.source-title {
  font-size: 14px;
  font-weight: 500;
  color: #1d1d1f;
  margin-bottom: 2px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.source-meta {
  font-size: 12px;
  color: #86868b;
  line-height: 1.4;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.sources-empty {
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
    font-size: 13px;
    color: #86868b;
    margin: 0;
    line-height: 1.5;
  }
}
</style>
