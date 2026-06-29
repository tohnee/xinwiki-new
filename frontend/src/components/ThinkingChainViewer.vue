<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import {
  ChevronDownIcon,
  ChevronRightIcon,
  CheckCircleFilledIcon,
  TimeFilledIcon,
  ErrorCircleFilledIcon,
  LoadingIcon,
  TipsIcon,
  SearchIcon,
  FileTxtIcon,
  ToolsIcon,
  ChatIcon,
} from 'tdesign-icons-vue-next'

interface ThinkingStep {
  id: string
  type: 'thinking' | 'search' | 'retrieve' | 'tool' | 'reasoning' | 'answer'
  title: string
  content: string
  status: 'pending' | 'running' | 'completed' | 'error'
  duration?: number
  timestamp: number
  details?: any
  children?: ThinkingStep[]
}

const props = withDefaults(defineProps<{
  steps?: ThinkingStep[]
  mini?: boolean
  autoExpand?: boolean
}>(), {
  steps: () => [],
  mini: false,
  autoExpand: true,
})

const expandedSteps = ref<Set<string>>(new Set())

const stepIcons: Record<ThinkingStep['type'], any> = {
  thinking: TipsIcon,
  search: SearchIcon,
  retrieve: FileTxtIcon,
  tool: ToolsIcon,
  reasoning: ChatIcon,
  answer: CheckCircleFilledIcon,
}

const stepColors: Record<ThinkingStep['type'], string> = {
  thinking: '#ff9500',
  search: '#007aff',
  retrieve: '#5856d6',
  tool: '#ff2d55',
  reasoning: '#34c759',
  answer: '#30d158',
}

const statusIcons: Record<string, any> = {
  pending: TimeFilledIcon,
  running: LoadingIcon,
  completed: CheckCircleFilledIcon,
  error: ErrorCircleFilledIcon,
}

const statusColors: Record<string, string> = {
  pending: '#8e8e93',
  running: '#007aff',
  completed: '#34c759',
  error: '#ff3b30',
}

const statusText: Record<string, string> = {
  pending: '等待中',
  running: '运行中',
  completed: '已完成',
  error: '错误',
}

const formatDuration = (ms?: number) => {
  if (!ms) return ''
  if (ms < 1000) return `${ms}ms`
  return `${(ms / 1000).toFixed(1)}s`
}

const formatTime = (timestamp: number) => {
  const date = new Date(timestamp)
  return date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

const toggleStep = (stepId: string) => {
  if (expandedSteps.value.has(stepId)) {
    expandedSteps.value.delete(stepId)
  } else {
    expandedSteps.value.add(stepId)
  }
}

watch(() => props.steps, (newSteps) => {
  if (props.autoExpand && newSteps.length > 0) {
    newSteps.forEach(step => expandedSteps.value.add(step.id))
  }
}, { immediate: true })

const totalDuration = computed(() => {
  if (!props.steps.length) return 0
  const first = props.steps[0]?.timestamp || 0
  const last = props.steps[props.steps.length - 1]?.timestamp || 0
  return last - first
})

const completedCount = computed(() => props.steps.filter(s => s.status === 'completed').length)
const runningCount = computed(() => props.steps.filter(s => s.status === 'running').length)
</script>

<template>
  <div class="thinking-chain-viewer" :class="{ 'mini': mini }">
    <!-- Summary Bar (mini mode) -->
    <div v-if="mini" class="thinking-summary">
      <div class="summary-progress">
        <div class="progress-dots">
          <span
            v-for="(step, idx) in steps.slice(0, 5)"
            :key="step.id"
            class="progress-dot"
            :class="step.status"
            :style="{ background: step.status === 'completed' ? stepColors[step.type] : undefined }"
          />
        </div>
      </div>
      <span class="summary-text" v-if="runningCount > 0">
        思考中...
      </span>
      <span class="summary-text" v-else-if="completedCount === steps.length && steps.length > 0">
        已完成 · {{ formatDuration(totalDuration) }}
      </span>
    </div>

    <!-- Full Viewer -->
    <div v-else class="thinking-timeline">
      <!-- Header -->
      <div v-if="steps.length > 0" class="timeline-header">
        <div class="header-stats">
          <span class="stat-item">
            <CheckCircleFilledIcon class="stat-icon success" />
            {{ completedCount }}/{{ steps.length }}
          </span>
          <span v-if="runningCount > 0" class="stat-item">
            <LoadingIcon class="stat-icon spinning" />
            {{ runningCount }} 运行中
          </span>
          <span class="stat-item">
            <TimeFilledIcon class="stat-icon" />
            {{ formatDuration(totalDuration) }}
          </span>
        </div>
      </div>

      <!-- Steps -->
      <div class="timeline-steps">
        <div v-for="(step, idx) in steps" :key="step.id" class="step-container">
          <!-- Timeline connector -->
          <div class="timeline-connector" v-if="idx < steps.length - 1">
            <div
              class="connector-line"
              :class="{ completed: step.status === 'completed' }"
              :style="{ background: step.status === 'completed' ? stepColors[step.type] : undefined }"
            />
          </div>

          <!-- Step Node -->
          <div class="step-node" :class="[step.type, step.status]">
            <button class="step-header" @click="toggleStep(step.id)">
              <div
                class="step-icon"
                :style="{ background: step.status === 'completed' ? stepColors[step.type] + '15' : undefined }"
              >
                <component
                  :is="step.status === 'running' ? LoadingIcon : stepIcons[step.type]"
                  :class="{ 'spinning': step.status === 'running' }"
                  :style="{ color: step.status === 'completed' ? stepColors[step.type] : undefined }"
                />
              </div>

              <div class="step-info">
                <div class="step-title-row">
                  <span class="step-title">{{ step.title }}</span>
                  <span v-if="step.duration" class="step-duration">{{ formatDuration(step.duration) }}</span>
                </div>
                <div class="step-meta">
                  <span class="step-time">{{ formatTime(step.timestamp) }}</span>
                  <span class="step-status" :style="{ color: statusColors[step.status] }">
                    <component :is="statusIcons[step.status]" :class="{ 'spinning': step.status === 'running' }" />
                    {{ statusText[step.status] }}
                  </span>
                </div>
              </div>

              <component
                :is="expandedSteps.has(step.id) ? ChevronDownIcon : ChevronRightIcon"
                class="expand-icon"
              />
            </button>

            <!-- Step Content -->
            <transition name="expand">
              <div v-if="expandedSteps.has(step.id)" class="step-content">
                <div class="content-text">{{ step.content }}</div>
                
                <!-- Details -->
                <div v-if="step.details" class="content-details">
                  <pre class="details-json">{{ JSON.stringify(step.details, null, 2) }}</pre>
                </div>

                <!-- Children steps -->
                <div v-if="step.children?.length" class="child-steps">
                  <ThinkingChainViewer :steps="step.children" mini />
                </div>
              </div>
            </transition>
          </div>
        </div>
      </div>

      <!-- Empty State -->
      <div v-if="steps.length === 0" class="empty-state">
        <TipIcon class="empty-icon" />
        <p class="empty-text">暂无思维链步骤</p>
      </div>
    </div>
  </div>
</template>

<style lang="less" scoped>
.thinking-chain-viewer {
  font-family: -apple-system, BlinkMacSystemFont, 'SF Pro Text', sans-serif;
  
  &.mini {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 12px;
    color: #8e8e93;
  }
}

/* Mini Mode */
.thinking-summary {
  display: flex;
  align-items: center;
  gap: 8px;
}

.summary-progress {
  display: flex;
  align-items: center;
}

.progress-dots {
  display: flex;
  gap: 3px;
}

.progress-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: rgba(0, 0, 0, 0.1);
  transition: all 0.3s ease;
  
  &.running {
    background: #007aff;
    animation: pulse 1.5s infinite;
  }
  
  &.completed {
    transform: scale(1);
  }
  
  &.error {
    background: #ff3b30;
  }
}

@keyframes pulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.5; transform: scale(0.8); }
}

.summary-text {
  white-space: nowrap;
}

/* Full Viewer */
.thinking-timeline {
  width: 100%;
}

.timeline-header {
  margin-bottom: 16px;
  padding-bottom: 12px;
  border-bottom: 1px solid rgba(0, 0, 0, 0.06);
}

.header-stats {
  display: flex;
  gap: 16px;
}

.stat-item {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  font-weight: 500;
  color: #1d1d1f;
}

.stat-icon {
  font-size: 14px;
  
  &.success { color: #34c759; }
  &.spinning { animation: spin 1s linear infinite; color: #007aff; }
}

/* Timeline Steps */
.timeline-steps {
  position: relative;
  padding-left: 28px;
}

.step-container {
  position: relative;
  padding-bottom: 16px;
  
  &:last-child {
    padding-bottom: 0;
  }
}

.timeline-connector {
  position: absolute;
  left: 11px;
  top: 32px;
  bottom: 0;
  width: 2px;
}

.connector-line {
  width: 100%;
  height: 100%;
  background: rgba(0, 0, 0, 0.08);
  border-radius: 1px;
  transition: background 0.3s ease;
  
  &.completed {
    background: linear-gradient(180deg, currentColor 0%, currentColor 100%);
  }
}

.step-node {
  position: relative;
}

.step-header {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  width: 100%;
  padding: 8px 12px;
  border: none;
  background: transparent;
  border-radius: 10px;
  cursor: pointer;
  text-align: left;
  transition: all 0.15s ease;
  
  &:hover {
    background: rgba(0, 0, 0, 0.03);
  }
}

.step-icon {
  flex-shrink: 0;
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: rgba(0, 0, 0, 0.05);
  font-size: 13px;
  position: relative;
  z-index: 1;
  transition: all 0.2s ease;
  
  .spinning {
    animation: spin 1s linear infinite;
  }
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.step-info {
  flex: 1;
  min-width: 0;
}

.step-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 3px;
}

.step-title {
  font-size: 14px;
  font-weight: 500;
  color: #1d1d1f;
}

.step-duration {
  font-size: 12px;
  font-weight: 600;
  color: #8e8e93;
  background: rgba(0, 0, 0, 0.05);
  padding: 2px 6px;
  border-radius: 4px;
}

.step-meta {
  display: flex;
  align-items: center;
  gap: 12px;
}

.step-time {
  font-size: 12px;
  color: #8e8e93;
}

.step-status {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  font-weight: 500;
  
  .spinning {
    animation: spin 1s linear infinite;
  }
}

.expand-icon {
  flex-shrink: 0;
  font-size: 16px;
  color: #c7c7cc;
  margin-top: 4px;
  transition: transform 0.2s ease;
}

/* Step Content */
.step-content {
  margin-top: 8px;
  margin-left: 36px;
  padding: 12px 16px;
  background: rgba(0, 0, 0, 0.02);
  border-radius: 10px;
  border: 1px solid rgba(0, 0, 0, 0.04);
}

.content-text {
  font-size: 13px;
  line-height: 1.6;
  color: #1d1d1f;
  white-space: pre-wrap;
  word-break: break-word;
}

.content-details {
  margin-top: 12px;
}

.details-json {
  margin: 0;
  padding: 12px;
  background: rgba(0, 0, 0, 0.03);
  border-radius: 8px;
  font-size: 11px;
  font-family: 'SF Mono', Menlo, Monaco, Consolas, monospace;
  color: #636366;
  overflow-x: auto;
  white-space: pre-wrap;
  word-break: break-all;
}

.child-steps {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid rgba(0, 0, 0, 0.06);
}

/* Expand Transition */
.expand-enter-active,
.expand-leave-active {
  transition: all 0.2s cubic-bezier(0.25, 0.1, 0.25, 1);
  overflow: hidden;
}

.expand-enter-from,
.expand-leave-to {
  opacity: 0;
  transform: translateY(-8px);
  max-height: 0;
  padding-top: 0;
  padding-bottom: 0;
  margin-top: 0;
}

/* Empty State */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 32px 16px;
  text-align: center;
}

.empty-icon {
  font-size: 32px;
  color: #c7c7cc;
  margin-bottom: 12px;
}

.empty-text {
  margin: 0;
  font-size: 14px;
  color: #8e8e93;
}

/* Step Status Variations */
.step-node.running {
  .step-icon {
    background: rgba(0, 122, 255, 0.1);
  }
}

.step-node.error {
  .step-header:hover {
    background: rgba(255, 59, 48, 0.05);
  }
}
</style>
