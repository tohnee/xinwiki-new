<template>
  <XinWikiWorkspace>
    <div class="workspace-content">
      <div class="wiki-page-viewer">
        <div class="page-header">
          <h1 class="page-title">{{ currentPage?.title || '选择一个知识库页面开始' }}</h1>
          <div class="page-meta" v-if="currentPage">
            <span class="meta-item">
              <TimeIcon size="small" />
              {{ formatDate(currentPage.updated_at) }}
            </span>
            <span class="meta-item">
              <BookIcon size="small" />
              {{ currentPage.knowledge_base_name }}
            </span>
          </div>
        </div>
        <div class="page-content" v-if="currentPage">
          <div class="markdown-body" v-html="renderedContent"></div>
        </div>
        <div class="empty-page" v-else>
          <BookIcon class="empty-icon" size="48" />
          <p>从左侧选择一个知识库或页面开始浏览和提问</p>
        </div>
      </div>
    </div>
  </XinWikiWorkspace>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import XinWikiWorkspace from '@/components/XinWikiWorkspace.vue'
import { TimeIcon, BookIcon } from 'tdesign-icons-vue-next'

interface WikiPage {
  id: string
  title: string
  content: string
  knowledge_base_name: string
  updated_at: string
}

const currentPage = ref<WikiPage | null>(null)
const renderedContent = computed(() => {
  if (!currentPage.value) return ''
  return currentPage.value.content
})

const formatDate = (dateStr: string) => {
  if (!dateStr) return ''
  const date = new Date(dateStr)
  return date.toLocaleDateString('zh-CN', {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  })
}
</script>

<style scoped>
.workspace-content {
  padding: 32px 48px;
  max-width: 900px;
  margin: 0 auto;
}

.page-header {
  margin-bottom: 32px;
  padding-bottom: 20px;
  border-bottom: 1px solid rgba(0, 0, 0, 0.06);
}

.page-title {
  font-size: 32px;
  font-weight: 700;
  color: #1d1d1f;
  margin: 0 0 12px 0;
  letter-spacing: -0.02em;
}

.page-meta {
  display: flex;
  gap: 20px;
  color: #86868b;
  font-size: 13px;
}

.meta-item {
  display: flex;
  align-items: center;
  gap: 6px;
}

.page-content {
  font-size: 16px;
  line-height: 1.7;
  color: #1d1d1f;
}

.markdown-body :deep(h1),
.markdown-body :deep(h2),
.markdown-body :deep(h3) {
  margin-top: 32px;
  margin-bottom: 16px;
  font-weight: 600;
}

.markdown-body :deep(p) {
  margin-bottom: 16px;
}

.markdown-body :deep(code) {
  background: #f5f5f7;
  padding: 2px 6px;
  border-radius: 4px;
  font-family: -apple-system, BlinkMacSystemFont, 'SF Mono', Monaco, monospace;
  font-size: 0.9em;
}

.markdown-body :deep(pre) {
  background: #f5f5f7;
  padding: 16px;
  border-radius: 12px;
  overflow-x: auto;
  margin: 20px 0;
}

.empty-page {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 80px 20px;
  color: #86868b;
  text-align: center;
}

.empty-icon {
  margin-bottom: 16px;
  opacity: 0.5;
}

.empty-page p {
  font-size: 15px;
  margin: 0;
}

@media (max-width: 768px) {
  .workspace-content {
    padding: 20px;
  }

  .page-title {
    font-size: 24px;
  }
}
</style>
