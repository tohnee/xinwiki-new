/**
 * thinkingSteps - pure helpers for the Workspace ThinkingPanel.
 *
 * P2 fix: useGeneration.ts used to hardcode a 4-step "thinking trace"
 * (`sampleThinkingSteps`) inside the composable. The steps looked like
 * real AI thinking ("正在分析您的生成指令...", "调用生成模型...") but
 * were actually just a synthetic progress indicator - the artifact
 * pipeline does not stream thinking tokens back to the client yet.
 *
 * This module owns the construction of those progress steps so they
 * are (a) unit-testable, (b) honestly labelled as progress rather
 * than as a thinking trace, and (c) easy to swap for real thinking
 * tokens when the backend exposes them.
 */
import type { ThinkingStep } from './useGeneration'

export type StepStatus = ThinkingStep['status']

export interface ProgressStepSpec {
  id: string
  type: ThinkingStep['type']
  title: string
  content: string
}

/**
 * The canonical progress-step templates for an artifact generation.
 * Order matters: step 1 starts `running`, the rest start `pending`.
 */
export const GENERATION_PROGRESS_STEPS: readonly ProgressStepSpec[] = [
  {
    id: 'progress-understand',
    type: 'thinking',
    title: '理解生成需求',
    content: '正在分析生成指令，提取关键要点。',
  },
  {
    id: 'progress-search',
    type: 'search',
    title: '检索相关内容',
    content: '从知识库中检索相关资料。',
  },
  {
    id: 'progress-organize',
    type: 'retrieve',
    title: '整理素材',
    content: '组织素材并构建大纲。',
  },
  {
    id: 'progress-generate',
    type: 'reasoning',
    title: '生成内容',
    content: '调用生成模型生成目标产物。',
  },
] as const

/**
 * Build the initial progress-step list for an artifact generation.
 * Step 0 starts `running`; the rest are `pending`.
 */
export const buildInitialProgressSteps = (): ThinkingStep[] => {
  const now = Date.now()
  return GENERATION_PROGRESS_STEPS.map((spec, i) => ({
    id: spec.id,
    type: spec.type,
    title: spec.title,
    content: spec.content,
    status: i === 0 ? 'running' : 'pending',
    timestamp: now,
  }))
}

/**
 * Mark the first N steps completed and the next one running. Used when
 * `createArtifact` resolves and we want to advance the progress UI.
 */
export const advanceProgressSteps = (
  steps: readonly ThinkingStep[],
  completedCount: number,
): ThinkingStep[] => {
  if (completedCount < 0) completedCount = 0
  return steps.map((s, i) => {
    if (i < completedCount) return { ...s, status: 'completed' as const }
    if (i === completedCount) return { ...s, status: 'running' as const }
    return s
  })
}

/**
 * Mark every step completed. Used when generation finishes or fails.
 */
export const completeAllSteps = (steps: readonly ThinkingStep[]): ThinkingStep[] =>
  steps.map(s => ({ ...s, status: 'completed' as const }))

/**
 * Whether the panel has any real content to render. The empty-state
 * placeholder is shown when there are no steps yet (e.g. the user has
 * not triggered a generation, or real thinking tokens have not started
 * streaming).
 */
export const hasThinkingContent = (steps: readonly ThinkingStep[] | null | undefined): boolean =>
  Array.isArray(steps) && steps.length > 0
