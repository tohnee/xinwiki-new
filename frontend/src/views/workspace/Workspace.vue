<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import XinWikiWorkspace from '@/components/XinWikiWorkspace.vue'
import WorkspaceChat from '@/components/workspace/WorkspaceChat.vue'
import { TimeIcon, BookIcon, ChatIcon, ArrowLeftIcon } from 'tdesign-icons-vue-next'
import { getWikiPage, type WikiPage } from '@/api/wiki'
import { marked } from 'marked'
import markedKatex from 'marked-katex-extension'
import { sanitizeMarkdownHTML } from '@/utils/security'
import { useAuthStore } from '@/stores/auth'
import { buildBreadcrumb } from '@/components/workspace/breadcrumb'
import { useKbStore } from '@/components/workspace/useKbStore'

marked.use(markedKatex({ throwOnError: false, nonStandard: true }))

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

const currentPage = ref<WikiPage | null>(null)
const pageContent = ref<string>('')
const viewMode = ref<'chat' | 'page'>('chat')
const pageContentRef = ref<HTMLElement | null>(null)
let lastLoadToken = 0

// P2 fix: KB list + active KB id are owned by the shared kbStore.
// Workspace.vue used to call listKnowledgeBases() on mount; now it
// reads the cached activeKbId and treats that as the workspace KB.
const { activeKbId: sharedKbId, ensureLoaded } = useKbStore()
const defaultKbId = computed(() => sharedKbId.value)

const selectedPageId = computed(() => {
  if (currentPage.value) {
    return currentPage.value.slug || currentPage.value.id
  }
  if (route.params.pageId) {
    return route.params.pageId as string
  }
  return undefined
})

const loadPageById = async (pageSlugOrId: string) => {
  const kbId = authStore.currentKnowledgeBase?.id || defaultKbId.value
  if (!kbId) {
    viewMode.value = 'chat'
    return
  }

  const token = ++lastLoadToken
  try {
    const res = await getWikiPage(kbId, pageSlugOrId) as any
    if (token !== lastLoadToken) return
    const page = res?.data as WikiPage | undefined
    if (page) {
      currentPage.value = page
      viewMode.value = 'page'
      try {
        const html = marked.parse(page.content || '') as string
        pageContent.value = sanitizeMarkdownHTML(html)
        await nextTick()
        setupInternalLinks()
      } catch {
        pageContent.value = page.content || ''
      }
    } else {
      currentPage.value = null
      viewMode.value = 'chat'
    }
  } catch (e) {
    if (token !== lastLoadToken) return
    console.warn('[workspace] load page failed', e)
    viewMode.value = 'chat'
  }
}

const setupInternalLinks = () => {
  if (!pageContentRef.value) return
  const links = pageContentRef.value.querySelectorAll('a[href]')
  links.forEach(link => {
    const href = link.getAttribute('href')
    if (!href) return
    if (href.startsWith('/wiki/') || href.startsWith('./') || (!href.startsWith('http') && !href.startsWith('#'))) {
      link.addEventListener('click', (e) => {
        e.preventDefault()
        const slug = href.replace(/^\.?\/?wiki\//, '').replace(/^\.\//, '')
        if (slug) {
          handleSelectPage({ slug, id: slug, title: slug } as WikiPage)
        }
      })
    }
  })
}

const handleSelectPage = async (page: WikiPage | null) => {
  if (!page) {
    currentPage.value = null
    viewMode.value = 'chat'
    if (route.params.pageId) {
      router.replace({ name: 'workspace' })
    }
    return
  }

  const slug = page.slug || page.id
  if (route.params.pageId !== slug) {
    router.replace({ name: 'workspacePage', params: { pageId: slug } })
  }
  await loadPageById(slug)
}

const handleBackToChat = () => {
  currentPage.value = null
  viewMode.value = 'chat'
  router.replace({ name: 'workspace' })
}

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

const breadcrumb = computed(() => {
  // Use the shared, unit-tested helper. P1 fix: previously the
  // component computed a breadcrumb but never passed it down to
  // WorkspaceContent, so the header always rendered an empty trail.
  return buildBreadcrumb({
    mode: viewMode.value,
    pageTitle: currentPage.value?.title,
    kbName: authStore.currentKnowledgeBase?.name,
  })
})

// P1 fix: Header search results used to be a dead link - XinWikiWorkspace
// emitted `select-search-result` but Workspace.vue never listened. Treat a
// search result the same way as a sidebar page pick: load the wiki page.
const handleSelectSearchResult = (result: { id: string; title: string; type: string }) => {
  handleSelectPage({ slug: result.id, id: result.id, title: result.title } as WikiPage)
}

// P1 fix: the sidebar "新建" button used to be a dead shell. Route to the
// knowledge base detail page where the file/upload UI lives, so the user
// can actually create a new page instead of clicking a no-op button.
const handleCreatePage = () => {
  const kbId = authStore.currentKnowledgeBase?.id || defaultKbId.value
  if (kbId) {
    router.push(`/platform/knowledge-bases/${kbId}`)
  } else {
    router.push('/platform/knowledge-bases')
  }
}

watch(() => route.params.pageId, async (pageId) => {
  if (pageId && typeof pageId === 'string') {
    if (!defaultKbId.value) {
      await ensureLoaded()
    }
    await loadPageById(pageId)
  } else {
    currentPage.value = null
    viewMode.value = 'chat'
  }
}, { immediate: false })

onMounted(async () => {
  await ensureLoaded()
  const pageId = route.params.pageId as string | undefined
  if (pageId && defaultKbId.value) {
    await loadPageById(pageId)
  }
})
</script>

<template>
  <XinWikiWorkspace
    :selected-page-id="selectedPageId"
    :breadcrumb="breadcrumb"
    @select-page="handleSelectPage"
    @select-search-result="handleSelectSearchResult"
    @create-page="handleCreatePage"
  >
    <template #header-actions>
      <div class="view-toggle" v-if="currentPage">
        <button
          class="toggle-btn"
          @click="handleBackToChat"
          title="返回问答"
        >
          <ArrowLeftIcon size="small" />
        </button>
        <button
          class="toggle-btn"
          :class="{ active: viewMode === 'chat' }"
          @click="viewMode = 'chat'"
          title="智能问答"
        >
          <ChatIcon size="small" />
          <span>问答</span>
        </button>
        <button
          class="toggle-btn"
          :class="{ active: viewMode === 'page' }"
          @click="viewMode = 'page'"
          title="页面内容"
        >
          <BookIcon size="small" />
          <span>页面</span>
        </button>
      </div>
    </template>

    <div class="workspace-content">
      <WorkspaceChat
        v-if="viewMode === 'chat'"
        :key="'chat-' + defaultKbId"
        :knowledge-base-ids="defaultKbId ? [defaultKbId] : undefined"
      />

      <div v-else-if="viewMode === 'page' && currentPage" class="wiki-page-viewer">
        <div class="page-header">
          <button class="back-btn" @click="handleBackToChat" title="返回问答">
            <ArrowLeftIcon size="small" />
            返回
          </button>
          <h1 class="page-title">{{ currentPage.title }}</h1>
          <div class="page-meta">
            <span class="meta-item">
              <TimeIcon size="small" />
              {{ formatDate(currentPage.updated_at) }}
            </span>
            <span v-if="currentPage.summary" class="page-summary">{{ currentPage.summary }}</span>
          </div>
        </div>
        <div ref="pageContentRef" class="page-content">
          <div class="markdown-body" v-html="pageContent" />
        </div>
      </div>
    </div>
  </XinWikiWorkspace>
</template>

<style scoped>
.workspace-content {
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.view-toggle {
  display: flex;
  gap: 4px;
  background: rgba(0, 0, 0, 0.04);
  padding: 3px;
  border-radius: 8px;
}
.toggle-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 5px 12px;
  border: none;
  background: transparent;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 500;
  color: #86868b;
  cursor: pointer;
  transition: all 0.15s ease;

  &.active {
    background: white;
    color: #007aff;
    box-shadow: 0 1px 4px rgba(0, 0, 0, 0.06);
  }
  &:hover:not(.active) { color: #1d1d1f; }
}

.wiki-page-viewer {
  padding: 8px 0;
  overflow-y: auto;
  height: 100%;
}

.back-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 6px 12px;
  margin-bottom: 16px;
  font-size: 13px;
  color: #007aff;
  background: rgba(0, 122, 255, 0.08);
  border: none;
  border-radius: 8px;
  cursor: pointer;
  &:hover { background: rgba(0, 122, 255, 0.15); }
}

.page-header {
  margin-bottom: 24px;
  padding-bottom: 16px;
  border-bottom: 1px solid rgba(0, 0, 0, 0.06);
}

.page-title {
  font-size: 28px;
  font-weight: 700;
  color: #1d1d1f;
  margin: 0 0 10px 0;
  letter-spacing: -0.02em;
}

.page-meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 16px;
  color: #86868b;
  font-size: 13px;
}

.meta-item {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.page-summary {
  color: #6c6c70;
  font-size: 14px;
}

.page-content {
  font-size: 15px;
  line-height: 1.75;
  color: #1d1d1f;
  padding-bottom: 40px;
}

.markdown-body :deep(h1),
.markdown-body :deep(h2),
.markdown-body :deep(h3) {
  margin-top: 28px;
  margin-bottom: 12px;
  font-weight: 600;
}
.markdown-body :deep(p) { margin-bottom: 14px; }
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
  margin: 16px 0;
}
.markdown-body :deep(ul), .markdown-body :deep(ol) {
  padding-left: 24px;
  margin-bottom: 14px;
}
.markdown-body :deep(blockquote) {
  border-left: 3px solid #007aff;
  padding-left: 16px;
  color: #6c6c70;
  margin: 16px 0;
}
.markdown-body :deep(a) {
  color: #007aff;
  text-decoration: none;
  &:hover { text-decoration: underline; }
}

@media (max-width: 768px) {
  .page-title { font-size: 22px; }
}
</style>
