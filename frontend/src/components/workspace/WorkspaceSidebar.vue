<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import {
  BookIcon,
  StarIcon,
  TimeIcon,
  AddIcon,
  FileTxtIcon,
  LoadingIcon,
  ChatIcon,
} from 'tdesign-icons-vue-next'
import { listKnowledgeBases } from '@/api/knowledge-base'
import { getWikiIndex, listWikiPages, type WikiPage, type WikiIndexResponse } from '@/api/wiki'
import { useAuthStore } from '@/stores/auth'

const props = defineProps<{
  collapsed: boolean
  isMobile: boolean
  selectedPageId?: string
}>()

const emit = defineEmits<{
  (e: 'toggle'): void
  (e: 'select-page', page: WikiPage | null): void
}>()

const authStore = useAuthStore()

const activeNav = ref<'chat' | 'knowledge' | 'favorites' | 'history'>('chat')
const kbList = ref<Array<{ id: string; name: string }>>([])
const activeKbId = ref<string>('')
const pages = ref<WikiPage[]>([])
const loadingPages = ref(false)
const expandedFolders = ref<Set<string>>(new Set(['root']))
const searchQuery = ref('')

const sidebarNavItems = computed(() => [
  { id: 'chat', name: '问答', icon: ChatIcon },
  { id: 'knowledge', name: '知识库', icon: BookIcon, active: activeNav.value === 'knowledge' },
  { id: 'favorites', name: '收藏夹', icon: StarIcon },
  { id: 'history', name: '历史记录', icon: TimeIcon },
])

const filteredPages = computed(() => {
  if (!searchQuery.value.trim()) return pages.value
  const q = searchQuery.value.toLowerCase()
  return pages.value.filter(p =>
    (p.title || '').toLowerCase().includes(q) ||
    (p.summary || '').toLowerCase().includes(q),
  )
})

const pageTree = computed(() => {
  const map = new Map<string, WikiPage[]>()
  map.set('', [])
  for (const p of filteredPages.value) {
    const parent = p.parent_slug || ''
    if (!map.has(parent)) map.set(parent, [])
    map.get(parent)!.push(p)
  }
  for (const arr of map.values()) {
    arr.sort((a, b) => (a.sort_order || 0) - (b.sort_order || 0) || a.title.localeCompare(b.title))
  }
  return map
})

const toggleFolder = (slug: string) => {
  if (expandedFolders.value.has(slug)) {
    expandedFolders.value.delete(slug)
  } else {
    expandedFolders.value.add(slug)
  }
}

const handleNavClick = (id: string) => {
  if (id === 'chat') {
    activeNav.value = 'chat'
    emit('select-page', null)
    return
  }
  activeNav.value = id as any
}

const handlePageClick = (page: WikiPage) => {
  emit('select-page', page)
}

const loadKBs = async () => {
  try {
    const res = await listKnowledgeBases({ creator: 'all' }) as any
    const list = res?.data || res?.knowledge_bases || []
    kbList.value = Array.isArray(list)
      ? list.map((kb: any) => ({ id: kb.id, name: kb.name }))
      : []
    if (!activeKbId.value) {
      const current = authStore.currentKnowledgeBase?.id
      activeKbId.value = current || (kbList.value[0]?.id || '')
    }
  } catch (e) {
    console.warn('[sidebar] load KBs failed', e)
  }
}

const loadPages = async () => {
  if (!activeKbId.value) {
    pages.value = []
    return
  }
  loadingPages.value = true
  try {
    const res = await listWikiPages(activeKbId.value, { page: 1, page_size: 200 }) as any
    pages.value = res?.pages || res?.data?.pages || []
  } catch (e) {
    try {
      const idxRes = await getWikiIndex(activeKbId.value, { limit: 200 }) as any
      const idx: WikiIndexResponse | undefined = idxRes?.data || idxRes
      const collected: WikiPage[] = []
      if (idx?.groups) {
        for (const g of idx.groups) {
          for (const it of (g.items || [])) {
            collected.push({
              id: it.slug,
              tenant_id: 0,
              knowledge_base_id: activeKbId.value,
              slug: it.slug,
              title: it.title,
              page_type: g.type,
              status: 'active',
              content: '',
              summary: it.summary || '',
              aliases: [],
              parent_slug: it.parent_slug,
              category_path: it.category_path,
              wiki_path: it.wiki_path,
              depth: it.depth,
              sort_order: it.sort_order,
              source_refs: [],
              in_links: [],
              out_links: [],
              page_metadata: {},
              version: 1,
              created_at: '',
              updated_at: '',
            })
          }
        }
      }
      pages.value = collected
    } catch (e2) {
      console.warn('[sidebar] load pages failed', e2)
      pages.value = []
    }
  } finally {
    loadingPages.value = false
  }
}

watch(activeKbId, () => {
  loadPages()
})

onMounted(async () => {
  await loadKBs()
  if (activeKbId.value) loadPages()
})
</script>

<template>
  <aside class="sidebar" :class="{ collapsed: props.collapsed, 'mobile-open': props.isMobile && !props.collapsed }">
    <div class="sidebar-scroll">
      <div v-if="!props.collapsed" class="sidebar-new-btn">
        <button class="new-button">
          <AddIcon />
          <span>新建</span>
        </button>
      </div>

      <nav class="sidebar-nav">
        <div class="nav-section">
          <div v-if="!props.collapsed" class="nav-section-header">
            <span class="section-title">导航</span>
          </div>
          <div
            v-for="navItem in sidebarNavItems"
            :key="navItem.id"
            class="nav-item"
            :class="{ active: activeNav === navItem.id }"
            @click="handleNavClick(navItem.id)"
          >
            <component :is="navItem.icon" class="nav-icon" />
            <span v-if="!props.collapsed" class="nav-label">{{ navItem.name }}</span>
          </div>
        </div>

        <div v-if="!props.collapsed" class="nav-section">
          <div class="nav-section-header">
            <span class="section-title">目录</span>
            <button class="section-action" @click="loadPages" title="刷新">
              <AddIcon size="small" />
            </button>
          </div>

          <div v-if="kbList.length > 1" class="kb-selector">
            <select v-model="activeKbId" class="kb-select">
              <option v-for="kb in kbList" :key="kb.id" :value="kb.id">{{ kb.name }}</option>
            </select>
          </div>

          <input
            v-model="searchQuery"
            type="text"
            class="tree-search"
            placeholder="搜索页面..."
          />

          <div v-if="loadingPages" class="tree-loading">
            <LoadingIcon class="spin" />
            <span>加载中...</span>
          </div>

          <div v-else class="tree-container">
            <div
              class="tree-item"
              :class="{ active: activeNav === 'chat' && !selectedPageId }"
              @click="handleNavClick('chat')"
            >
              <ChatIcon class="tree-icon" />
              <span class="tree-label">智能问答</span>
            </div>

            <template v-if="pageTree.get('')?.length">
              <div
                v-for="page in pageTree.get('')"
                :key="page.id"
                class="tree-item"
                :class="{ active: selectedPageId === page.id }"
                @click="handlePageClick(page)"
              >
                <FileTxtIcon v-if="page.page_type === 'page' || !page.page_type" class="tree-icon" />
                <BookIcon v-else class="tree-icon" />
                <span class="tree-label">{{ page.title }}</span>
              </div>
            </template>

            <div v-if="filteredPages.length === 0 && !loadingPages" class="tree-empty">
              暂无页面
            </div>
          </div>
        </div>
      </nav>
    </div>
  </aside>
</template>

<style lang="less" scoped>
.sidebar {
  width: 280px;
  display: flex;
  flex-direction: column;
  background: rgba(255, 255, 255, 0.6);
  backdrop-filter: saturate(180%) blur(20px);
  -webkit-backdrop-filter: saturate(180%) blur(20px);
  border-right: 1px solid rgba(0, 0, 0, 0.06);
  transition: width 0.3s cubic-bezier(0.25, 0.1, 0.25, 1);
  flex-shrink: 0;

  &.collapsed {
    width: 52px;
  }
}

.sidebar-scroll {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  padding: 12px;

  &::-webkit-scrollbar {
    width: 6px;
  }

  &::-webkit-scrollbar-track {
    background: transparent;
  }

  &::-webkit-scrollbar-thumb {
    background: rgba(0, 0, 0, 0.15);
    border-radius: 3px;

    &:hover {
      background: rgba(0, 0, 0, 0.25);
    }
  }
}

.sidebar-new-btn {
  margin-bottom: 16px;
}

.new-button {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 10px 16px;
  background: linear-gradient(135deg, #007aff 0%, #5856d6 100%);
  color: white;
  border: none;
  border-radius: 10px;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s cubic-bezier(0.25, 0.1, 0.25, 1);
  box-shadow: 0 4px 12px rgba(0, 122, 255, 0.25);

  &:hover {
    transform: translateY(-1px);
    box-shadow: 0 6px 16px rgba(0, 122, 255, 0.3);
  }

  &:active {
    transform: translateY(0);
  }
}

.sidebar-nav {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.nav-section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 6px;
  padding: 0 4px;
}

.section-title {
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: #86868b;
}

.section-action {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border: none;
  background: transparent;
  border-radius: 6px;
  color: #86868b;
  cursor: pointer;

  &:hover {
    background: rgba(0, 0, 0, 0.06);
    color: #1d1d1f;
  }
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 10px;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.15s ease;
  margin-bottom: 2px;

  .nav-icon {
    font-size: 18px;
    color: #86868b;
    flex-shrink: 0;
  }

  .nav-label {
    flex: 1;
    font-size: 14px;
    color: #1d1d1f;
    font-weight: 500;
  }

  .nav-badge {
    font-size: 12px;
    font-weight: 600;
    color: #007aff;
    background: rgba(0, 122, 255, 0.1);
    padding: 2px 7px;
    border-radius: 10px;
  }

  &:hover {
    background: rgba(0, 0, 0, 0.04);

    .nav-icon {
      color: #1d1d1f;
    }
  }

  &.active {
    background: rgba(0, 122, 255, 0.1);

    .nav-icon {
      color: #007aff;
    }

    .nav-label {
      color: #007aff;
      font-weight: 600;
    }
  }
}

.kb-selector {
  margin-bottom: 8px;
}
.kb-select {
  width: 100%;
  padding: 6px 8px;
  border: 1px solid rgba(0, 0, 0, 0.08);
  border-radius: 8px;
  font-size: 13px;
  background: white;
  color: #1d1d1f;
  outline: none;
  &:focus { border-color: #007aff; }
}

.tree-search {
  width: 100%;
  padding: 6px 10px;
  margin-bottom: 8px;
  border: 1px solid rgba(0, 0, 0, 0.08);
  border-radius: 8px;
  font-size: 13px;
  background: rgba(255, 255, 255, 0.8);
  outline: none;
  box-sizing: border-box;
  &:focus { border-color: #007aff; background: white; }
  &::placeholder { color: #86868b; }
}

.tree-loading {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px;
  font-size: 13px;
  color: #86868b;
  .spin { animation: spin 0.8s linear infinite; }
}

@keyframes spin { to { transform: rotate(360deg); } }

.collapsed {
  .nav-item {
    justify-content: center;
    padding: 8px;
  }

  .new-button {
    padding: 10px;
    span { display: none; }
  }
}

.tree-container {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.tree-empty {
  padding: 12px;
  text-align: center;
  font-size: 12px;
  color: #86868b;
}

.tree-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 8px;
  border-radius: 6px;
  cursor: pointer;
  font-size: 13px;
  transition: all 0.15s ease;

  .tree-icon {
    font-size: 14px;
    color: #86868b;
    flex-shrink: 0;
  }

  .tree-label {
    color: #1d1d1f;
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  &:hover {
    background: rgba(0, 0, 0, 0.04);
  }

  &.active {
    background: rgba(0, 122, 255, 0.08);

    .tree-icon,
    .tree-label {
      color: #007aff;
    }

    .tree-label {
      font-weight: 500;
    }
  }
}

@media (max-width: 1280px) {
  .sidebar {
    width: 240px;
    &.collapsed { width: 52px; }
  }
}

@media (max-width: 768px) {
  .sidebar {
    position: fixed;
    left: 0;
    top: 52px;
    bottom: 0;
    z-index: 90;
    transform: translateX(-100%);
    transition: transform 0.3s cubic-bezier(0.25, 0.1, 0.25, 1);

    &.mobile-open {
      transform: translateX(0);
    }

    &.collapsed {
      width: 280px;
    }
  }
}
</style>
