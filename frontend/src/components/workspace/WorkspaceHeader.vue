<script setup lang="ts">
import { ref, watch, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import {
  ViewListIcon,
  SearchIcon,
  BookIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  StarFilledIcon,
  FileTxtIcon,
  SettingIcon,
} from 'tdesign-icons-vue-next'
import XinWikiLogo from '@/components/XinWikiLogo.vue'
import { searchKnowledge } from '@/api/knowledge-base'
import { useKbStore } from './useKbStore'
import {
  buildDropdownItems,
  chevronClass,
  selectorIsInteractive,
  selectorAriaLabel,
} from './kbSelector'

const props = defineProps<{
  sidebarCollapsed: boolean
  rightPanelVisible: boolean
  searchQuery: string
  kbName?: string
}>()

const emit = defineEmits<{
  (e: 'toggle-sidebar'): void
  (e: 'toggle-right-panel'): void
  (e: 'update:searchQuery', value: string): void
  (e: 'select-search-result', result: { id: string; title: string; type: string }): void
}>()

const router = useRouter()

const results = ref<Array<{ id: string; title: string; excerpt?: string; type: string }>>([])
const showResults = ref(false)
const searching = ref(false)

const hasQuery = computed(() => props.searchQuery.trim().length > 0)

// P2 fix: KB selector used to be a dead div. Now it opens a real
// dropdown listing every KB from the shared kbStore, and clicking a
// row switches the active KB for the whole workspace surface.
const { kbList, activeKbId, setActiveKb } = useKbStore()
const kbDropdownOpen = ref(false)
const kbDropdownItems = computed(() => buildDropdownItems(kbList.value, activeKbId.value))
const kbInteractive = computed(() => selectorIsInteractive(kbList.value))
const kbAriaLabel = computed(() => selectorAriaLabel(kbDropdownOpen.value, props.kbName))
const kbChevronClass = computed(() => chevronClass(kbDropdownOpen.value))

const toggleKbDropdown = () => {
  if (!kbInteractive.value) return
  kbDropdownOpen.value = !kbDropdownOpen.value
}

const pickKb = (id: string) => {
  setActiveKb(id)
  kbDropdownOpen.value = false
}

const goToKbManagement = () => {
  kbDropdownOpen.value = false
  router.push('/platform/knowledge-bases')
}

// Close the dropdown when clicking outside it.
const handleDocClick = (e: MouseEvent) => {
  if (!kbDropdownOpen.value) return
  const target = e.target as HTMLElement
  if (!target.closest('.kb-selector')) {
    kbDropdownOpen.value = false
  }
}

onMounted(() => {
  document.addEventListener('click', handleDocClick, true)
})
onUnmounted(() => {
  document.removeEventListener('click', handleDocClick, true)
})

let debounceTimer: ReturnType<typeof setTimeout> | null = null

watch(() => props.searchQuery, (q) => {
  if (debounceTimer) clearTimeout(debounceTimer)
  const trimmed = q.trim()
  if (!trimmed) {
    results.value = []
    showResults.value = false
    return
  }
  if (trimmed.length < 2) return
  debounceTimer = setTimeout(async () => {
    searching.value = true
    try {
      const res = await searchKnowledge(trimmed, 0, 8) as any
      const list = res?.data?.results || res?.results || res?.data?.items || []
      results.value = Array.isArray(list)
        ? list.slice(0, 8).map((r: any) => ({
            id: r.id || r.knowledge_id || r.kb_id || String(Math.random()),
            title: r.title || r.name || r.doc_title || '(无标题)',
            excerpt: r.excerpt || r.content || r.snippet,
            type: r.type || 'document',
          }))
        : []
      showResults.value = results.value.length > 0
    } catch (e) {
      console.warn('[header search] failed', e)
      results.value = []
      showResults.value = false
    } finally {
      searching.value = false
    }
  }, 300)
})

const handleBlur = () => {
  setTimeout(() => { showResults.value = false }, 150)
}
const handleFocus = () => {
  if (results.value.length > 0) showResults.value = true
}

const pickResult = (r: { id: string; title: string; type: string }) => {
  emit('select-search-result', r)
  showResults.value = false
}
</script>

<template>
  <header class="workspace-header">
    <div class="header-left">
      <button class="icon-button" @click="emit('toggle-sidebar')" :title="sidebarCollapsed ? '展开侧边栏' : '收起侧边栏'">
        <ViewListIcon v-if="sidebarCollapsed" />
        <ChevronLeftIcon v-else />
      </button>
      <div class="logo-container">
        <XinWikiLogo :size="28" />
        <span v-if="!sidebarCollapsed" class="logo-text">XinWiki</span>
      </div>
      <div class="kb-selector" :class="{ interactive: kbInteractive }" :aria-label="kbAriaLabel" @click="toggleKbDropdown">
        <BookIcon class="kb-icon" />
        <span class="kb-name">{{ kbName || '选择知识库' }}</span>
        <ChevronRightIcon :class="kbChevronClass" />

        <!-- P2 fix: the chevron finally has a dropdown behind it. -->
        <div v-if="kbDropdownOpen" class="kb-dropdown">
          <div
            v-for="item in kbDropdownItems"
            :key="item.id"
            class="kb-dropdown-item"
            :class="{ active: item.isActive }"
            @click.stop="pickKb(item.id)"
          >
            <BookIcon class="kb-dropdown-icon" />
            <span class="kb-dropdown-name">{{ item.name }}</span>
          </div>
          <div class="kb-dropdown-divider" />
          <div class="kb-dropdown-item manage" @click.stop="goToKbManagement">
            <SettingIcon class="kb-dropdown-icon" />
            <span class="kb-dropdown-name">管理知识库</span>
          </div>
        </div>
      </div>
    </div>

    <div class="header-center">
      <div class="global-search" :class="{ 'has-results': showResults }">
        <SearchIcon class="search-icon" />
        <input
          :value="searchQuery"
          @input="emit('update:searchQuery', ($event.target as HTMLInputElement).value)"
          @focus="handleFocus"
          @blur="handleBlur"
          @keydown.esc="showResults = false"
          type="text"
          placeholder="搜索文档、对话、知识源..."
          class="search-input"
        />
        <div v-if="searching" class="search-loading">
          <div class="mini-spinner" />
        </div>
        <kbd v-else class="search-shortcut">⌘K</kbd>

        <div v-if="showResults" class="search-dropdown">
          <div v-if="results.length === 0 && !searching" class="search-empty">无结果</div>
          <div
            v-for="r in results"
            :key="r.id"
            class="search-result"
            @mousedown.prevent="pickResult(r)"
          >
            <FileTxtIcon class="result-icon" />
            <div class="result-body">
              <div class="result-title">{{ r.title }}</div>
              <div v-if="r.excerpt" class="result-excerpt">{{ r.excerpt.slice(0, 80) }}</div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="header-right">
      <button class="icon-button" title="AI生成面板" @click="emit('toggle-right-panel')">
        <StarFilledIcon :class="{ 'active': rightPanelVisible }" />
      </button>
      <div class="user-avatar">
        <span class="avatar-initials">XW</span>
      </div>
    </div>
  </header>
</template>

<style lang="less" scoped>
.workspace-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 52px;
  padding: 0 16px;
  background: rgba(255, 255, 255, 0.72);
  backdrop-filter: saturate(180%) blur(20px);
  -webkit-backdrop-filter: saturate(180%) blur(20px);
  border-bottom: 1px solid rgba(0, 0, 0, 0.06);
  z-index: 100;
  flex-shrink: 0;
  position: relative;
}

.header-left,
.header-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.logo-container {
  display: flex;
  align-items: center;
  gap: 8px;
}

.logo-text {
  font-size: 17px;
  font-weight: 600;
  color: #1d1d1f;
  letter-spacing: -0.02em;
}

.kb-selector {
  position: relative;
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border-radius: 8px;
  cursor: default;
  transition: all 0.2s cubic-bezier(0.25, 0.1, 0.25, 1);

  &.interactive {
    cursor: pointer;
    &:hover {
      background: rgba(0, 0, 0, 0.04);
    }
  }
}

.kb-icon {
  font-size: 16px;
  color: #007aff;
}

.kb-name {
  font-size: 14px;
  font-weight: 500;
  color: #1d1d1f;
  max-width: 160px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.chevron-icon {
  font-size: 14px;
  color: #86868b;
  transition: transform 0.2s ease;

  &.open {
    transform: rotate(90deg);
  }
}

.kb-dropdown {
  position: absolute;
  top: calc(100% + 4px);
  left: 0;
  min-width: 220px;
  max-height: 320px;
  overflow-y: auto;
  background: white;
  border-radius: 10px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12);
  border: 1px solid rgba(0, 0, 0, 0.06);
  padding: 6px;
  z-index: 300;
  animation: kbDropdownIn 0.15s ease;
}

@keyframes kbDropdownIn {
  from { opacity: 0; transform: translateY(-4px); }
  to { opacity: 1; transform: translateY(0); }
}

.kb-dropdown-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border-radius: 6px;
  cursor: pointer;
  transition: background 0.1s;

  &:hover {
    background: rgba(0, 122, 255, 0.06);
  }

  &.active {
    background: rgba(0, 122, 255, 0.1);
    .kb-dropdown-name { color: #007aff; font-weight: 500; }
  }

  &.manage {
    color: #6c6c70;
    .kb-dropdown-name { font-size: 13px; }
  }
}

.kb-dropdown-icon {
  font-size: 14px;
  color: #86868b;
  flex-shrink: 0;
}

.kb-dropdown-name {
  font-size: 14px;
  color: #1d1d1f;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.kb-dropdown-divider {
  height: 1px;
  background: rgba(0, 0, 0, 0.06);
  margin: 4px 0;
}

.global-search {
  position: relative;
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  max-width: 520px;
  height: 32px;
  padding: 0 12px;
  background: rgba(0, 0, 0, 0.04);
  border-radius: 8px;
  transition: all 0.2s cubic-bezier(0.25, 0.1, 0.25, 1);

  &:focus-within, &.has-results {
    background: rgba(255, 255, 255, 0.95);
    box-shadow: 0 0 0 4px rgba(0, 122, 255, 0.1);
    border-bottom-left-radius: 0;
    border-bottom-right-radius: 0;
  }
}

.search-icon {
  font-size: 16px;
  color: #86868b;
  flex-shrink: 0;
}

.search-input {
  flex: 1;
  border: none;
  background: transparent;
  font-size: 14px;
  color: #1d1d1f;
  outline: none;

  &::placeholder {
    color: #86868b;
  }
}

.search-shortcut {
  display: inline-flex;
  align-items: center;
  padding: 2px 6px;
  font-size: 11px;
  font-family: -apple-system, BlinkMacSystemFont, monospace;
  color: #86868b;
  background: rgba(0, 0, 0, 0.06);
  border-radius: 4px;
  flex-shrink: 0;
}

.search-loading {
  display: flex;
  align-items: center;
}
.mini-spinner {
  width: 14px;
  height: 14px;
  border: 2px solid rgba(0, 122, 255, 0.2);
  border-top-color: #007aff;
  border-radius: 50%;
  animation: spin 0.7s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }

.search-dropdown {
  position: absolute;
  top: 100%;
  left: 0;
  right: 0;
  background: white;
  border-bottom-left-radius: 8px;
  border-bottom-right-radius: 8px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12);
  padding: 6px 0;
  max-height: 360px;
  overflow-y: auto;
  z-index: 200;
  border: 1px solid rgba(0, 122, 255, 0.2);
  border-top: none;
}

.search-empty {
  padding: 14px 16px;
  font-size: 13px;
  color: #86868b;
  text-align: center;
}

.search-result {
  display: flex;
  gap: 10px;
  padding: 10px 14px;
  cursor: pointer;
  transition: background 0.1s;

  &:hover {
    background: rgba(0, 122, 255, 0.06);
  }
}

.result-icon {
  font-size: 16px;
  color: #007aff;
  flex-shrink: 0;
  margin-top: 2px;
}

.result-body {
  flex: 1;
  min-width: 0;
}

.result-title {
  font-size: 14px;
  font-weight: 500;
  color: #1d1d1f;
  margin-bottom: 2px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.result-excerpt {
  font-size: 12px;
  color: #86868b;
  line-height: 1.4;
  overflow: hidden;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
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

  .active {
    color: #007aff;
  }
}

.user-avatar {
  width: 30px;
  height: 30px;
  border-radius: 50%;
  background: linear-gradient(135deg, #007aff 0%, #5856d6 100%);
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: transform 0.15s ease;

  &:hover {
    transform: scale(1.05);
  }
}

.avatar-initials {
  font-size: 12px;
  font-weight: 600;
  color: white;
}

@media (max-width: 768px) {
  .header-center {
    display: none;
  }
}
</style>
