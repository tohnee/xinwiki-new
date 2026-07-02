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
  RefreshIcon,
  ChevronRightIcon,
  FolderIcon,
  FolderOpenIcon,
} from 'tdesign-icons-vue-next'
import { getWikiIndex, listWikiPages, type WikiPage, type WikiIndexResponse } from '@/api/wiki'
import { resolveNavAction, navHasContent, type SidebarNavId } from './sidebarNav'
import { useKbStore } from './useKbStore'

const props = defineProps<{
  collapsed: boolean
  isMobile: boolean
  selectedPageId?: string
}>()

const emit = defineEmits<{
  (e: 'toggle'): void
  (e: 'select-page', page: WikiPage | null): void
  /**
   * P1 fix: the "新建" button used to be a dead shell. Now it emits
   * `create-page` so the parent (Workspace.vue) can route to the
   * wiki page editor for the active KB.
   */
  (e: 'create-page'): void
}>()

// P2 fix: KB list + active KB id are owned by the shared kbStore.
// The sidebar used to call listKnowledgeBases() on mount; now it just
// reads from the shared cache and reacts to activeKbId changes.
const { kbList, activeKbId, ensureLoaded, setActiveKb } = useKbStore()

const activeNav = ref<'chat' | 'knowledge' | 'favorites' | 'history'>('chat')
const pages = ref<WikiPage[]>([])
const loadingPages = ref(false)
const expandedFolders = ref<Set<string>>(new Set(['root', '']))
const searchQuery = ref('')
const searchQueryDebounced = ref('')
let searchDebounceTimer: ReturnType<typeof setTimeout> | null = null

const sidebarNavItems = computed(() => [
  { id: 'chat', name: '问答', icon: ChatIcon },
  { id: 'knowledge', name: '知识库', icon: BookIcon, active: activeNav.value === 'knowledge' },
  { id: 'favorites', name: '收藏夹', icon: StarIcon },
  { id: 'history', name: '历史记录', icon: TimeIcon },
])

watch(searchQuery, (val) => {
  if (searchDebounceTimer) clearTimeout(searchDebounceTimer)
  searchDebounceTimer = setTimeout(() => {
    searchQueryDebounced.value = val
  }, 200)
})

const filteredPages = computed(() => {
  if (!searchQueryDebounced.value.trim()) return pages.value
  const q = searchQueryDebounced.value.toLowerCase()
  return pages.value.filter(p =>
    (p.title || '').toLowerCase().includes(q) ||
    (p.summary || '').toLowerCase().includes(q),
  )
})

interface TreeNode {
  page?: WikiPage
  slug: string
  title: string
  children: TreeNode[]
  isFolder: boolean
}

const pageTree = computed(() => {
  const map = new Map<string, TreeNode>()
  map.set('', { slug: '', title: '', children: [], isFolder: true })

  for (const p of filteredPages.value) {
    const parent = p.parent_slug || ''
    if (!map.has(parent)) {
      map.set(parent, { slug: parent, title: parent.split('/').pop() || parent, children: [], isFolder: true })
    }
    const node: TreeNode = {
      page: p,
      slug: p.slug,
      title: p.title,
      children: [],
      isFolder: p.page_type === 'folder',
    }
    map.get(parent)!.children.push(node)
    if (!map.has(p.slug)) {
      map.set(p.slug, node)
    } else {
      const existing = map.get(p.slug)!
      existing.page = p
      existing.title = p.title
      existing.isFolder = p.page_type === 'folder'
    }
  }

  for (const node of map.values()) {
    node.children.sort((a, b) => {
      const aOrder = a.page?.sort_order ?? 0
      const bOrder = b.page?.sort_order ?? 0
      if (aOrder !== bOrder) return aOrder - bOrder
      if (a.isFolder !== b.isFolder) return a.isFolder ? -1 : 1
      return a.title.localeCompare(b.title)
    })
  }

  return map
})

const ensureAncestorsExpanded = (slug: string) => {
  if (!slug) return
  const parts = slug.split('/')
  for (let i = 1; i <= parts.length; i++) {
    const ancestor = parts.slice(0, i).join('/')
    expandedFolders.value.add(ancestor)
  }
}

watch(() => props.selectedPageId, (newId) => {
  if (newId) {
    ensureAncestorsExpanded(newId)
    activeNav.value = 'knowledge'
  }
}, { immediate: true })

const isPageSelected = (page: WikiPage) => {
  if (!props.selectedPageId) return false
  return props.selectedPageId === page.id || props.selectedPageId === page.slug
}

const toggleFolder = (slug: string) => {
  if (expandedFolders.value.has(slug)) {
    expandedFolders.value.delete(slug)
  } else {
    expandedFolders.value.add(slug)
  }
}

const handleNavClick = (id: string) => {
  // P1 fix: route nav clicks through the shared, unit-tested
  // resolver. Previously favorites/history silently flipped the
  // highlight with no content behind them; the template now uses
  // navHasContent() to render an empty-state placeholder instead.
  const action = resolveNavAction(id)
  activeNav.value = action.activeNav
  if (action.selectPage === 'clear') {
    emit('select-page', null)
  }
}

const handleCreate = () => {
  // P1 fix: the "新建" button had no @click. Emit `create-page` so
  // the parent can route to the page editor for the active KB.
  emit('create-page')
}

const onKbSelectChange = (e: Event) => {
  // P2 fix: activeKbId is owned by the shared kbStore (readonly here).
  // Route the select through setActiveKb so all three components see
  // the change.
  const id = (e.target as HTMLSelectElement).value
  setActiveKb(id)
}

const showEmptyState = computed(() => !navHasContent(activeNav.value))

const handlePageClick = (page: WikiPage) => {
  if (page.page_type === 'folder') {
    toggleFolder(page.slug)
    return
  }
  activeNav.value = 'knowledge'
  emit('select-page', page)
}

const loadPages = async () => {
  if (!activeKbId.value) {
    pages.value = []
    return
  }
  loadingPages.value = true
  try {
    const res = await listWikiPages(activeKbId.value, { page: 1, page_size: 500 }) as any
    pages.value = res?.pages || res?.data?.pages || []
  } catch (e) {
    try {
      const idxRes = await getWikiIndex(activeKbId.value, { limit: 500 }) as any
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
              page_type: g.type === 'folders' ? 'folder' : 'page',
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
  await ensureLoaded()
  if (activeKbId.value) loadPages()
})

const rootChildren = computed(() => pageTree.value.get('')?.children || [])
</script>

<template>
  <aside class="sidebar" :class="{ collapsed: props.collapsed, 'mobile-open': props.isMobile && !props.collapsed }">
    <div class="sidebar-scroll">
      <div v-if="!props.collapsed" class="sidebar-new-btn">
        <button class="new-button" @click="handleCreate" title="新建页面">
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
            <button class="section-action" @click="loadPages" title="刷新" :disabled="loadingPages">
              <RefreshIcon :class="{ spin: loadingPages }" size="small" />
            </button>
          </div>

          <!-- P1 fix: favorites / history have no backend yet. Show an
               empty-state placeholder instead of silently rendering the
               page tree (which made it look like the feature was wired up). -->
          <div v-if="showEmptyState" class="nav-empty-state">
            <StarIcon class="empty-icon" />
            <p class="empty-text">暂未开放，敬请期待</p>
          </div>

          <template v-else>
          <div v-if="kbList.length > 1" class="kb-selector">
            <select :value="activeKbId" class="kb-select" @change="onKbSelectChange">
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

            <template v-for="node in rootChildren" :key="node.slug">
              <div
                v-if="node.isFolder && node.children.length === 0 && !node.page"
                class="tree-item tree-folder"
                :class="{ active: selectedPageId === node.slug }"
                @click="toggleFolder(node.slug)"
              >
                <ChevronRightIcon
                  class="tree-expand-icon"
                  :class="{ expanded: expandedFolders.has(node.slug) }"
                  size="small"
                />
                <FolderIcon v-if="!expandedFolders.has(node.slug)" class="tree-icon" />
                <FolderOpenIcon v-else class="tree-icon" />
                <span class="tree-label">{{ node.title }}</span>
              </div>

              <template v-else>
                <div
                  v-if="node.isFolder"
                  class="tree-item tree-folder"
                  :class="{ active: selectedPageId === node.slug }"
                  @click="toggleFolder(node.slug)"
                >
                  <ChevronRightIcon
                    class="tree-expand-icon"
                    :class="{ expanded: expandedFolders.has(node.slug) }"
                    size="small"
                  />
                  <FolderIcon v-if="!expandedFolders.has(node.slug)" class="tree-icon" />
                  <FolderOpenIcon v-else class="tree-icon" />
                  <span class="tree-label">{{ node.title }}</span>
                </div>
                <div v-else class="tree-item tree-indent-0" :class="{ active: node.page && isPageSelected(node.page) }" @click="node.page && handlePageClick(node.page)">
                  <span class="tree-expand-placeholder" />
                  <FileTxtIcon v-if="!node.isFolder" class="tree-icon" />
                  <BookIcon v-else class="tree-icon" />
                  <span class="tree-label">{{ node.title }}</span>
                </div>

                <div v-if="node.isFolder && expandedFolders.has(node.slug)" class="tree-children">
                  <template v-for="child in node.children" :key="child.slug">
                    <div
                      v-if="child.isFolder"
                      class="tree-item tree-folder tree-indent-1"
                      :class="{ active: selectedPageId === child.slug }"
                      @click="toggleFolder(child.slug)"
                    >
                      <ChevronRightIcon
                        class="tree-expand-icon"
                        :class="{ expanded: expandedFolders.has(child.slug) }"
                        size="small"
                      />
                      <FolderIcon v-if="!expandedFolders.has(child.slug)" class="tree-icon" />
                      <FolderOpenIcon v-else class="tree-icon" />
                      <span class="tree-label">{{ child.title }}</span>
                    </div>
                    <div v-else class="tree-item tree-indent-1" :class="{ active: child.page && isPageSelected(child.page) }" @click="child.page && handlePageClick(child.page)">
                      <span class="tree-expand-placeholder" />
                      <FileTxtIcon class="tree-icon" />
                      <span class="tree-label">{{ child.title }}</span>
                    </div>

                    <div v-if="child.isFolder && expandedFolders.has(child.slug)" class="tree-children">
                      <template v-for="grandchild in child.children" :key="grandchild.slug">
                        <div
                          v-if="grandchild.isFolder"
                          class="tree-item tree-folder tree-indent-2"
                          :class="{ active: selectedPageId === grandchild.slug }"
                          @click="toggleFolder(grandchild.slug)"
                        >
                          <ChevronRightIcon
                            class="tree-expand-icon"
                            :class="{ expanded: expandedFolders.has(grandchild.slug) }"
                            size="small"
                          />
                          <FolderIcon v-if="!expandedFolders.has(grandchild.slug)" class="tree-icon" />
                          <FolderOpenIcon v-else class="tree-icon" />
                          <span class="tree-label">{{ grandchild.title }}</span>
                        </div>
                        <div v-else class="tree-item tree-indent-2" :class="{ active: grandchild.page && isPageSelected(grandchild.page) }" @click="grandchild.page && handlePageClick(grandchild.page)">
                          <span class="tree-expand-placeholder" />
                          <FileTxtIcon class="tree-icon" />
                          <span class="tree-label">{{ grandchild.title }}</span>
                        </div>

                        <div v-if="grandchild.isFolder && expandedFolders.has(grandchild.slug)" class="tree-children">
                          <div
                            v-for="ggc in grandchild.children"
                            :key="ggc.slug"
                            class="tree-item tree-indent-3"
                            :class="{ active: ggc.page && isPageSelected(ggc.page) }"
                            @click="ggc.page && handlePageClick(ggc.page)"
                          >
                            <span class="tree-expand-placeholder" />
                            <FileTxtIcon v-if="!ggc.isFolder" class="tree-icon" />
                            <FolderIcon v-else class="tree-icon" />
                            <span class="tree-label">{{ ggc.title }}</span>
                          </div>
                        </div>
                      </template>
                    </div>
                  </template>
                </div>
              </template>
            </template>

            <div v-if="filteredPages.length === 0 && !loadingPages" class="tree-empty">
              暂无页面
            </div>
          </div>
          </template>
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

  &:hover:not(:disabled) {
    background: rgba(0, 0, 0, 0.06);
    color: #1d1d1f;
  }

  &:disabled {
    opacity: 0.5;
    cursor: not-allowed;
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
  gap: 1px;
}

.tree-children {
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.tree-empty {
  padding: 12px;
  text-align: center;
  font-size: 12px;
  color: #86868b;
}

.nav-empty-state {
  padding: 32px 16px;
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;

  .empty-icon {
    font-size: 28px;
    color: #c7c7cc;
    margin-bottom: 10px;
  }

  .empty-text {
    font-size: 13px;
    color: #86868b;
    margin: 0;
    line-height: 1.5;
  }
}

.tree-item {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 5px 6px;
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

  .tree-expand-icon {
    font-size: 12px;
    color: #c7c7cc;
    flex-shrink: 0;
    transition: transform 0.15s ease;

    &.expanded {
      transform: rotate(90deg);
    }
  }

  .tree-expand-placeholder {
    width: 12px;
    flex-shrink: 0;
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

.tree-folder {
  .tree-icon {
    color: #86868b;
  }
}

.tree-indent-0 {
  padding-left: calc(6px + 18px);
}

.tree-indent-1 {
  padding-left: calc(6px + 18px * 2);
}

.tree-indent-2 {
  padding-left: calc(6px + 18px * 3);
}

.tree-indent-3 {
  padding-left: calc(6px + 18px * 4);
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
