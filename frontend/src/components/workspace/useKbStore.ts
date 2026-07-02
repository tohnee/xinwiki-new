/**
 * useKbStore - reactive KB state shared across the Workspace surface.
 *
 * P2 fix: this is the single source of truth for the KB list + active
 * KB id. Workspace.vue / XinWikiWorkspace.vue / WorkspaceSidebar.vue
 * used to each call `listKnowledgeBases({ creator: 'all' })` on mount;
 * they now call `useKbStore()` and read the cached state instead.
 *
 * The store is module-scoped: the first caller triggers the load,
 * subsequent callers within the same SPA session reuse the result.
 * `reload()` is exposed for explicit refresh (e.g. after creating a
 * new KB).
 */
import { ref, computed, readonly } from 'vue'
import { listKnowledgeBases } from '@/api/knowledge-base'
import { useAuthStore } from '@/stores/auth'
import {
  pickInitialKbId,
  resolveKbName,
  normalizeKbList,
  type KbListItem,
} from './kbStore'

// Module-scoped state: shared across every caller of useKbStore().
const kbList = ref<KbListItem[]>([])
const activeKbId = ref<string>('')
const loading = ref(false)
const loaded = ref(false)
let loadPromise: Promise<void> | null = null

const ensureLoaded = async (): Promise<void> => {
  if (loaded.value) return
  if (loadPromise) return loadPromise
  loadPromise = doLoad()
  return loadPromise
}

const doLoad = async (): Promise<void> => {
  loading.value = true
  try {
    const authStore = useAuthStore()
    const res = await listKnowledgeBases({ creator: 'all' }) as any
    kbList.value = normalizeKbList(res)
    activeKbId.value = pickInitialKbId(
      authStore.currentKnowledgeBase?.id,
      kbList.value,
    )
    loaded.value = true
  } catch (e) {
    console.warn('[kbStore] load KBs failed', e)
    kbList.value = []
    activeKbId.value = ''
  } finally {
    loading.value = false
    loadPromise = null
  }
}

export function useKbStore() {
  const activeKbName = computed(() => resolveKbName(activeKbId.value, kbList.value))

  /**
   * Switch the active KB. No-op if the id is not in the list - this
   * guards against stale deep links to a KB the user lost access to.
   */
  const setActiveKb = (id: string) => {
    if (!id) return
    if (!kbList.value.some(k => k.id === id)) return
    activeKbId.value = id
  }

  /**
   * Force a fresh load (e.g. after creating / deleting a KB). Clears
   * the cache and re-fetches.
   */
  const reload = async (): Promise<void> => {
    loaded.value = false
    loadPromise = null
    return ensureLoaded()
  }

  return {
    kbList: readonly(kbList),
    activeKbId: readonly(activeKbId),
    activeKbName,
    loading: readonly(loading),
    ensureLoaded,
    reload,
    setActiveKb,
  }
}
