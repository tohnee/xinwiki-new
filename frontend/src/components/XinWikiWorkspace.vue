<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import {
  ViewListIcon,
  SearchIcon,
  BookIcon,
  StarIcon,
  TimeIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  AddIcon,
  StarFilledIcon,
  CloseIcon,
  SendIcon,
  FileTxtIcon,
  FileExcelIcon,
  ChartIcon,
  HelpCircleFilledIcon,
  TipsIcon,
} from 'tdesign-icons-vue-next'
import XinWikiLogo from '@/components/XinWikiLogo.vue'
import ThinkingChainViewer from '@/components/ThinkingChainViewer.vue'

// Layout state
const sidebarCollapsed = ref(false)
const rightPanelVisible = ref(true)
const rightPanelTab = ref<'generate' | 'sources' | 'thinking'>('generate')
const isMobile = ref(false)

// Search
const searchQuery = ref('')

// Generation state
const generateInput = ref('')
const isGenerating = ref(false)
const generationType = ref<'summary' | 'briefing' | 'faq' | 'timeline' | 'mindmap' | 'presentation' | 'chart'>('summary')

// Sample thinking steps
const sampleThinkingSteps = ref([
  {
    id: '1',
    type: 'thinking' as const,
    title: '理解用户问题',
    content: '正在分析用户的查询意图，识别关键概念和需求...',
    status: 'completed' as const,
    duration: 120,
    timestamp: Date.now() - 5000,
  },
  {
    id: '2',
    type: 'search' as const,
    title: '检索知识库',
    content: '使用混合检索（BM25 + 向量 + 知识图谱）搜索相关文档...',
    status: 'completed' as const,
    duration: 350,
    timestamp: Date.now() - 4500,
    details: { query: 'hybrid retrieval RRF', results: 15, topK: 5 },
  },
  {
    id: '3',
    type: 'retrieve' as const,
    title: '提取相关片段',
    content: '从检索结果中提取最相关的内容片段，进行重排序...',
    status: 'completed' as const,
    duration: 200,
    timestamp: Date.now() - 4000,
  },
  {
    id: '4',
    type: 'reasoning' as const,
    title: '推理生成回答',
    content: '基于检索到的上下文信息，构建准确、有引用支持的回答...',
    status: 'running' as const,
    timestamp: Date.now() - 500,
  },
])

// Generation types
const generationTypes = [
  { id: 'summary', name: '内容总结', icon: FileTxtIcon, description: '生成长文内容的简明摘要' },
  { id: 'briefing', name: '研究简报', icon: FileExcelIcon, description: '生成结构化的研究报告' },
  { id: 'faq', name: '常见问题', icon: HelpCircleFilledIcon, description: '基于文档生成问答对' },
  { id: 'timeline', name: '时间线', icon: TimeIcon, description: '提取关键事件生成时间线' },
  { id: 'mindmap', name: '思维导图', icon: TipsIcon, description: '生成结构化思维导图' },
  { id: 'presentation', name: '演示文稿', icon: FileExcelIcon, description: '生成PPT大纲和内容' },
  { id: 'chart', name: '数据图表', icon: ChartIcon, description: '提取数据生成可视化图表' },
]

// Sample generated content
const generatedContent = ref<string>('')
const generatedCitations = ref<Array<{ id: string; title: string; excerpt: string }>>([])

// Sidebar navigation items
const sidebarNavItems = computed(() => [
  { id: 'knowledge', name: '知识库', icon: BookIcon, active: true },
  { id: 'favorites', name: '收藏夹', icon: StarIcon, badge: 5 },
  { id: 'history', name: '历史记录', icon: TimeIcon, badge: 12 },
])

const handleResize = () => {
  isMobile.value = window.innerWidth < 768
  if (isMobile.value) {
    sidebarCollapsed.value = true
    rightPanelVisible.value = false
  }
}

const toggleSidebar = () => {
  sidebarCollapsed.value = !sidebarCollapsed.value
}

const toggleRightPanel = () => {
  rightPanelVisible.value = !rightPanelVisible.value
}

const handleGenerate = async () => {
  if (!generateInput.value.trim()) return
  isGenerating.value = true
  
  setTimeout(() => {
    const selectedType = generationTypes.find(t => t.id === generationType.value)
    generatedContent.value = `### ${selectedType?.name}\n\n基于您提供的知识源，已生成以下内容：\n\n${generateInput.value}\n\n**核心要点：**\n1. 混合检索架构结合BM25关键词匹配与向量语义搜索\n2. RRF融合算法有效提升检索准确率37%\n3. 增量编译机制减少重复计算，响应速度提升60%\n4. 思维链可视化完整展示AI推理过程`
    
    generatedCitations.value = [
      { id: '1', title: 'XinWiki混合检索架构设计', excerpt: 'XinWiki采用BM25 + 向量 + 知识图谱的三层混合检索架构...' },
      { id: '2', title: 'RRF融合算法原理与实现', excerpt: 'Reciprocal Rank Fusion算法通过倒数排名融合多个检索结果...' },
    ]
    isGenerating.value = false
    generateInput.value = ''
  }, 2000)
}

onMounted(() => {
  handleResize()
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
})
</script>

<template>
  <div class="xinwiki-workspace">
    <header class="workspace-header">
      <div class="header-left">
        <button class="icon-button" @click="toggleSidebar" :title="sidebarCollapsed ? '展开侧边栏' : '收起侧边栏'">
          <ViewListIcon v-if="sidebarCollapsed" />
          <ChevronLeftIcon v-else />
        </button>
        <div class="logo-container">
          <XinWikiLogo :size="28" />
          <span v-if="!sidebarCollapsed" class="logo-text">XinWiki</span>
        </div>
        <div class="kb-selector">
          <BookIcon class="kb-icon" />
          <span class="kb-name">选择知识库</span>
          <ChevronRightIcon class="chevron-icon" />
        </div>
      </div>

      <div class="header-center">
        <div class="global-search">
          <SearchIcon class="search-icon" />
          <input
            v-model="searchQuery"
            type="text"
            placeholder="搜索文档、对话、知识源..."
            class="search-input"
          />
          <kbd class="search-shortcut">⌘K</kbd>
        </div>
      </div>

      <div class="header-right">
        <button class="icon-button" title="AI生成面板" @click="toggleRightPanel">
          <StarFilledIcon :class="{ 'active': rightPanelVisible }" />
        </button>
        <div class="user-avatar">
          <span class="avatar-initials">XW</span>
        </div>
      </div>
    </header>

    <div class="workspace-body">
      <aside class="sidebar" :class="{ collapsed: sidebarCollapsed }">
        <div class="sidebar-scroll">
          <div v-if="!sidebarCollapsed" class="sidebar-new-btn">
            <button class="new-button">
              <AddIcon />
              <span>新建</span>
            </button>
          </div>

          <nav class="sidebar-nav">
            <div class="nav-section">
              <div v-if="!sidebarCollapsed" class="nav-section-header">
                <span class="section-title">导航</span>
              </div>
              <div
                v-for="navItem in sidebarNavItems"
                :key="navItem.id"
                class="nav-item"
                :class="{ active: navItem.active }"
              >
                <component :is="navItem.icon" class="nav-icon" />
                <span v-if="!sidebarCollapsed" class="nav-label">{{ navItem.name }}</span>
                <span v-if="!sidebarCollapsed && navItem.badge" class="nav-badge">{{ navItem.badge }}</span>
              </div>
            </div>

            <div v-if="!sidebarCollapsed" class="nav-section">
              <div class="nav-section-header">
                <span class="section-title">目录</span>
                <button class="section-action">
                  <AddIcon size="small" />
                </button>
              </div>
              <div class="tree-container">
                <div class="tree-item active">
                  <BookIcon class="tree-icon" />
                  <span class="tree-label">全部页面</span>
                </div>
                <div class="tree-item">
                  <FileTxtIcon class="tree-icon" />
                  <span class="tree-label">快速入门</span>
                </div>
                <div class="tree-item">
                  <FileTxtIcon class="tree-icon" />
                  <span class="tree-label">API 参考文档</span>
                </div>
              </div>
            </div>
          </nav>
        </div>
      </aside>

      <main class="main-content">
        <div class="content-scroll">
          <div class="content-header">
            <div class="breadcrumb">
              <span class="breadcrumb-item">知识库</span>
              <ChevronRightIcon class="breadcrumb-separator" />
              <span class="breadcrumb-item active">当前页面</span>
            </div>
            <div class="content-actions">
              <slot name="header-actions" />
            </div>
          </div>

          <div class="content-area">
            <slot />
            <div v-if="!$slots.default" class="welcome-screen">
              <div class="welcome-icon">
                <StarFilledIcon size="64" />
              </div>
              <h2 class="welcome-title">欢迎使用 XinWiki</h2>
              <p class="welcome-desc">
                AI 驱动的企业级知识库平台，支持高精度问答、混合检索和智能生成
              </p>
              <div class="welcome-features">
                <div class="feature-card">
                  <SearchIcon class="feature-icon" />
                  <h4>混合检索</h4>
                  <p>BM25 + 向量 + 知识图谱，精准找到你需要的信息</p>
                </div>
                <div class="feature-card">
                  <TipsIcon class="feature-icon" />
                  <h4>智能生成</h4>
                  <p>一键生成总结、简报、FAQ、思维导图等多种内容</p>
                </div>
                <div class="feature-card">
                  <TipsIcon class="feature-icon" />
                  <h4>思维链可视化</h4>
                  <p>完整展示 AI 思考过程，每一步都清晰可见</p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </main>

      <aside v-if="rightPanelVisible && !isMobile" class="right-panel">
        <div class="panel-header">
          <div class="panel-tabs">
            <button
              class="panel-tab"
              :class="{ active: rightPanelTab === 'generate' }"
              @click="rightPanelTab = 'generate'"
            >
              <StarFilledIcon />
              <span class="tab-label">生成</span>
            </button>
            <button
              class="panel-tab"
              :class="{ active: rightPanelTab === 'sources' }"
              @click="rightPanelTab = 'sources'"
            >
              <FileTxtIcon />
              <span class="tab-label">来源</span>
            </button>
            <button
              class="panel-tab"
              :class="{ active: rightPanelTab === 'thinking' }"
              @click="rightPanelTab = 'thinking'"
            >
              <TipsIcon />
              <span class="tab-label">思维链</span>
            </button>
          </div>
          <button class="icon-button panel-close" @click="toggleRightPanel" title="关闭面板">
            <CloseIcon size="small" />
          </button>
        </div>

        <div class="panel-scroll">
          <div v-if="rightPanelTab === 'generate'" class="panel-content">
            <div class="generate-types">
              <button
                v-for="type in generationTypes"
                :key="type.id"
                class="type-card"
                :class="{ selected: generationType === type.id }"
                @click="generationType = type.id as any"
              >
                <component :is="type.icon" class="type-icon" />
                <span class="type-name">{{ type.name }}</span>
              </button>
            </div>

            <div class="generate-input-container">
              <textarea
                v-model="generateInput"
                placeholder="输入指令，例如：总结这篇文档的核心要点..."
                class="generate-input"
                rows="3"
                @keydown.enter.meta="handleGenerate"
              />
              <button
                class="generate-button"
                :disabled="!generateInput.trim() || isGenerating"
                @click="handleGenerate"
              >
                <SendIcon v-if="!isGenerating" />
                <div v-else class="loading-spinner small" />
              </button>
            </div>

            <div v-if="generatedContent" class="generated-content">
              <div class="content-header-bar">
                <span class="content-title">{{ generationTypes.find(t => t.id === generationType)?.name }}</span>
                <div class="content-actions">
                  <button class="action-button">复制</button>
                  <button class="action-button">导出</button>
                </div>
              </div>
              <div class="content-body" v-html="generatedContent.replace(/\n/g, '<br>').replace(/### (.*?)(<br>|$)/g, '<h4>$1</h4>').replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')" />
              
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

            <div v-else class="empty-state">
              <StarFilledIcon class="empty-icon" />
              <h4 class="empty-title">AI 智能生成</h4>
              <p class="empty-description">
                NotebookLM 风格的智能生成面板，支持一键生成总结、简报、FAQ、思维导图等多种内容
              </p>
            </div>
          </div>

          <div v-if="rightPanelTab === 'sources'" class="panel-content">
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

          <div v-if="rightPanelTab === 'thinking'" class="panel-content">
            <ThinkingChainViewer :steps="sampleThinkingSteps" />
          </div>
        </div>
      </aside>

      <button
        v-if="!rightPanelVisible && !isMobile"
        class="panel-collapse-handle"
        @click="toggleRightPanel"
        title="打开生成面板"
      >
        <StarFilledIcon />
      </button>
    </div>
  </div>
</template>

<style lang="less" scoped>
.xinwiki-workspace {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100vh;
  background: linear-gradient(180deg, #f5f7fa 0%, #e8edf5 100%);
  font-family: -apple-system, BlinkMacSystemFont, 'SF Pro Display', 'SF Pro Text', 'Helvetica Neue', Arial, sans-serif;
  overflow: hidden;
}

.frosted-glass {
  background: rgba(255, 255, 255, 0.72);
  backdrop-filter: saturate(180%) blur(20px);
  -webkit-backdrop-filter: saturate(180%) blur(20px);
  border: 1px solid rgba(255, 255, 255, 0.5);
}

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
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s cubic-bezier(0.25, 0.1, 0.25, 1);
  
  &:hover {
    background: rgba(0, 0, 0, 0.04);
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
}

.chevron-icon {
  font-size: 14px;
  color: #86868b;
}

.global-search {
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
  
  &:focus-within {
    background: rgba(255, 255, 255, 0.9);
    box-shadow: 0 0 0 4px rgba(0, 122, 255, 0.1);
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

.workspace-body {
  flex: 1;
  display: flex;
  min-height: 0;
  overflow: hidden;
}

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

.collapsed {
  .nav-item {
    justify-content: center;
    padding: 8px;
  }
  
  .new-button {
    padding: 10px;
    
    span {
      display: none;
    }
  }
}

.tree-container {
  display: flex;
  flex-direction: column;
  gap: 2px;
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

.main-content {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  background: transparent;
}

.content-scroll {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  padding: 24px 32px;
  
  &::-webkit-scrollbar {
    width: 8px;
  }
  
  &::-webkit-scrollbar-track {
    background: transparent;
  }
  
  &::-webkit-scrollbar-thumb {
    background: rgba(0, 0, 0, 0.15);
    border-radius: 4px;
    
    &:hover {
      background: rgba(0, 0, 0, 0.25);
    }
  }
}

.content-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 24px;
}

.breadcrumb {
  display: flex;
  align-items: center;
  gap: 6px;
}

.breadcrumb-item {
  font-size: 14px;
  color: #86868b;
  
  &.active {
    color: #1d1d1f;
    font-weight: 500;
  }
}

.breadcrumb-separator {
  font-size: 14px;
  color: #c7c7cc;
}

.content-area {
  background: white;
  border-radius: 16px;
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.06);
  border: 1px solid rgba(0, 0, 0, 0.04);
  min-height: 600px;
  padding: 32px;
}

.welcome-screen {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  padding: 40px 20px;
}

.welcome-icon {
  width: 96px;
  height: 96px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, rgba(0, 122, 255, 0.1) 0%, rgba(88, 86, 214, 0.1) 100%);
  border-radius: 24px;
  color: #007aff;
  margin-bottom: 24px;
}

.welcome-title {
  font-size: 28px;
  font-weight: 700;
  color: #1d1d1f;
  margin: 0 0 12px 0;
  letter-spacing: -0.02em;
}

.welcome-desc {
  font-size: 16px;
  color: #86868b;
  margin: 0 0 40px 0;
  max-width: 500px;
  line-height: 1.6;
}

.welcome-features {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 20px;
  width: 100%;
  max-width: 800px;
}

.feature-card {
  padding: 24px;
  background: rgba(0, 0, 0, 0.02);
  border-radius: 16px;
  border: 1px solid rgba(0, 0, 0, 0.04);
  text-align: left;
  transition: all 0.2s ease;
  
  &:hover {
    background: rgba(0, 122, 255, 0.04);
    border-color: rgba(0, 122, 255, 0.2);
    transform: translateY(-2px);
  }
}

.feature-icon {
  font-size: 28px;
  color: #007aff;
  margin-bottom: 12px;
}

.feature-card h4 {
  font-size: 17px;
  font-weight: 600;
  color: #1d1d1f;
  margin: 0 0 8px 0;
}

.feature-card p {
  font-size: 14px;
  color: #86868b;
  margin: 0;
  line-height: 1.5;
}

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
  
  &:hover {
    background: rgba(0, 122, 255, 0.04);
    border-color: rgba(0, 122, 255, 0.2);
    transform: translateY(-1px);
  }
  
  &.selected {
    background: rgba(0, 122, 255, 0.08);
    border-color: #007aff;
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
  margin-bottom: 20px;
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
  
  &.small {
    width: 14px;
    height: 14px;
  }
}

@keyframes spin {
  to { transform: rotate(360deg); }
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
  padding: 4px 10px;
  font-size: 12px;
  font-weight: 500;
  color: #007aff;
  background: rgba(0, 122, 255, 0.08);
  border: none;
  border-radius: 6px;
  cursor: pointer;
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
  
  :deep(h4) {
    margin: 0 0 12px 0;
    font-size: 16px;
    font-weight: 600;
    color: #1d1d1f;
  }
  
  :deep(strong) {
    font-weight: 600;
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
  .sidebar {
    width: 240px;
  }
  
  .right-panel {
    width: 340px;
  }
}

@media (max-width: 768px) {
  .content-scroll {
    padding: 16px;
  }
  
  .content-area {
    padding: 20px;
    border-radius: 12px;
  }
  
  .header-center {
    display: none;
  }
  
  .sidebar {
    position: fixed;
    left: 0;
    top: 52px;
    bottom: 0;
    z-index: 90;
    transform: translateX(-100%);
    transition: transform 0.3s cubic-bezier(0.25, 0.1, 0.25, 1);
    
    &:not(.collapsed) {
      transform: translateX(0);
    }
  }
  
  .main-content {
    width: 100%;
  }
  
  .welcome-features {
    grid-template-columns: 1fr;
  }
}
</style>
