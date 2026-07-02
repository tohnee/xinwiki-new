import { ref, onMounted, onUnmounted } from 'vue'

export function useWorkspaceLayout() {
  const sidebarCollapsed = ref(false)
  const rightPanelVisible = ref(true)
  const rightPanelTab = ref<'generate' | 'sources' | 'thinking' | 'notes'>('generate')
  const isMobile = ref(false)

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

  onMounted(() => {
    handleResize()
    window.addEventListener('resize', handleResize)
  })

  onUnmounted(() => {
    window.removeEventListener('resize', handleResize)
  })

  return {
    sidebarCollapsed,
    rightPanelVisible,
    rightPanelTab,
    isMobile,
    toggleSidebar,
    toggleRightPanel,
  }
}
