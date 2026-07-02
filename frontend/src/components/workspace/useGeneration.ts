import { ref, computed, onUnmounted } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import {
  FileTxtIcon,
  FileExcelIcon,
  ChartIcon,
  HelpCircleFilledIcon,
  TimeIcon,
  TipsIcon,
} from 'tdesign-icons-vue-next'
import { createArtifact, generateArtifact, getArtifact, downloadArtifactURL, type Artifact, type ArtifactStatus } from '@/api/artifact'
import {
  buildInitialProgressSteps,
  advanceProgressSteps,
  completeAllSteps,
} from './thinkingSteps'

export type GenerationTypeId = 'summary' | 'briefing' | 'faq' | 'timeline' | 'mindmap' | 'presentation' | 'chart'

export interface GenerationType {
  id: GenerationTypeId
  name: string
  icon: any
  description: string
  artifactType: 'markdown' | 'report' | 'ppt' | 'pdf' | 'chart'
}

export interface Citation {
  id: string
  title: string
  excerpt: string
}

export interface ThinkingStep {
  id: string
  type: 'thinking' | 'search' | 'retrieve' | 'reasoning'
  title: string
  content: string
  status: 'completed' | 'running' | 'pending'
  duration?: number
  timestamp: number
  details?: Record<string, any>
}

export type GenerationStatus = 'idle' | 'generating' | 'ready' | 'failed'

export function useGeneration() {
  const generateInput = ref('')
  const isGenerating = ref(false)
  const generationType = ref<GenerationTypeId>('summary')
  const generatedContent = ref<string>('')
  const generatedCitations = ref<Citation[]>([])
  const generationStatus = ref<GenerationStatus>('idle')
  const currentArtifact = ref<Artifact | null>(null)
  const generationError = ref<string>('')
  let pollTimer: ReturnType<typeof setTimeout> | null = null

  const generationTypes: GenerationType[] = [
    { id: 'summary', name: '内容总结', icon: FileTxtIcon, description: '生成长文内容的简明摘要', artifactType: 'markdown' },
    { id: 'briefing', name: '研究简报', icon: FileExcelIcon, description: '生成结构化的研究报告', artifactType: 'report' },
    { id: 'faq', name: '常见问题', icon: HelpCircleFilledIcon, description: '基于文档生成问答对', artifactType: 'markdown' },
    { id: 'timeline', name: '时间线', icon: TimeIcon, description: '提取关键事件生成时间线', artifactType: 'markdown' },
    { id: 'mindmap', name: '思维导图', icon: TipsIcon, description: '生成结构化思维导图', artifactType: 'markdown' },
    { id: 'presentation', name: '演示文稿', icon: FileExcelIcon, description: '生成PPT大纲和内容', artifactType: 'ppt' },
    { id: 'chart', name: '数据图表', icon: ChartIcon, description: '提取数据生成可视化图表', artifactType: 'chart' },
  ]

  // P2 fix: progress steps used to be inlined here as a hardcoded
  // mock. They are now built by the shared, unit-tested helpers in
  // thinkingSteps.ts so the panel is honestly labelled as a
  // generation progress indicator (the artifact pipeline does not
  // stream real thinking tokens yet).
  const sampleThinkingSteps = ref<ThinkingStep[]>([])

  const isDownloadable = computed(() => {
    if (!currentArtifact.value) return false
    const t = currentArtifact.value.type
    return t === 'ppt' || t === 'report' || t === 'chart' || t === 'markdown'
  })

  const artifactDownloadUrl = computed(() => {
    if (!currentArtifact.value?.id) return ''
    return downloadArtifactURL(currentArtifact.value.id)
  })

  const clearPoll = () => {
    if (pollTimer) {
      clearTimeout(pollTimer)
      pollTimer = null
    }
  }

  onUnmounted(() => {
    clearPoll()
  })

  const pollArtifact = async (id: string, attempts = 0) => {
    const MAX_ATTEMPTS = 90
    if (attempts >= MAX_ATTEMPTS) {
      clearPoll()
      generationStatus.value = 'failed'
      generationError.value = '生成超时，请稍后重试'
      isGenerating.value = false
      MessagePlugin.error('生成超时')
      return
    }
    try {
      const res = await getArtifact(id)
      if (res?.success && res.data) {
        currentArtifact.value = res.data
        const status: ArtifactStatus = res.data.status
        if (status === 'ready') {
          clearPoll()
          generationStatus.value = 'ready'
          isGenerating.value = false
          const meta = res.data.metadata || {}
          generatedContent.value = meta.content || meta.markdown || meta.text || ''
          if (Array.isArray(meta.citations)) {
            generatedCitations.value = meta.citations
          } else if (!generatedContent.value) {
            generatedContent.value = `### ${generationTypes.find(t => t.id === generationType.value)?.name || '内容'}\n\n生成已完成，可点击下方按钮下载文件。`
          }
          sampleThinkingSteps.value = completeAllSteps(sampleThinkingSteps.value)
          MessagePlugin.success('生成完成')
          return
        }
        if (status === 'failed') {
          clearPoll()
          generationStatus.value = 'failed'
          generationError.value = metaErrorMessage(res.data)
          isGenerating.value = false
          MessagePlugin.error('生成失败')
          return
        }
      }
    } catch (e: any) {
      console.warn('[artifact] poll error:', e)
    }
    pollTimer = setTimeout(() => pollArtifact(id, attempts + 1), 2000)
  }

  const metaErrorMessage = (art: Artifact): string => {
    const meta = art.metadata || {}
    return meta.error || meta.message || '生成失败，请稍后重试'
  }

  const handleGenerate = async () => {
    const prompt = generateInput.value.trim()
    if (!prompt) return
    const typeDef = generationTypes.find(t => t.id === generationType.value)
    if (!typeDef) return

    clearPoll()
    isGenerating.value = true
    generationStatus.value = 'generating'
    generationError.value = ''
    generatedContent.value = ''
    generatedCitations.value = []
    currentArtifact.value = null
    generateInput.value = ''

    sampleThinkingSteps.value = buildInitialProgressSteps()

    try {
      const res = await createArtifact({
        type: typeDef.artifactType,
        title: typeDef.name + ' - ' + prompt.slice(0, 30),
        sharing_policy: 'private',
      })
      if (res?.success && res.data?.id) {
        currentArtifact.value = res.data
        sampleThinkingSteps.value = advanceProgressSteps(sampleThinkingSteps.value, 2)
        await generateArtifact(res.data.id, prompt)
        pollArtifact(res.data.id)
      } else {
        throw new Error('create failed')
      }
    } catch (e: any) {
      isGenerating.value = false
      generationStatus.value = 'failed'
      generationError.value = e?.response?.data?.error?.message || e?.message || '创建生成任务失败'
      MessagePlugin.error(generationError.value)
      sampleThinkingSteps.value = completeAllSteps(sampleThinkingSteps.value)
    }
  }

  const resetGeneration = () => {
    clearPoll()
    generatedContent.value = ''
    generatedCitations.value = []
    currentArtifact.value = null
    generationStatus.value = 'idle'
    generationError.value = ''
    sampleThinkingSteps.value = []
  }

  return {
    generateInput,
    isGenerating,
    generationType,
    generatedContent,
    generatedCitations,
    generationTypes,
    sampleThinkingSteps,
    generationStatus,
    currentArtifact,
    generationError,
    isDownloadable,
    artifactDownloadUrl,
    handleGenerate,
    resetGeneration,
  }
}
