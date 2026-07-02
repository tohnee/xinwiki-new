<script setup lang="ts">
import {
  AddIcon,
  FileTxtIcon,
} from 'tdesign-icons-vue-next'
</script>

<template>
  <div class="panel-content">
    <div class="sources-header">
      <span class="sources-count">12 个知识源</span>
      <button class="add-source-button">
        <AddIcon size="small" />
        添加来源
      </button>
    </div>
    <div class="sources-list">
      <div v-for="n in 5" :key="n" class="source-item">
        <FileTxtIcon class="source-icon" />
        <div class="source-info">
          <div class="source-title">文档 {{ n }}</div>
          <div class="source-meta">{{ 8 + n }} 个分块</div>
        </div>
      </div>
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
}
</style>
