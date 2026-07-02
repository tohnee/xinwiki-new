<script setup lang="ts">
import { TipsIcon } from 'tdesign-icons-vue-next'
import ThinkingChainViewer from '@/components/ThinkingChainViewer.vue'
import type { ThinkingStep } from './useGeneration'
import { hasThinkingContent } from './thinkingSteps'

const props = defineProps<{
  sampleThinkingSteps: ThinkingStep[]
}>()
</script>

<template>
  <div class="panel-content">
    <!-- P2 fix: previously the panel always rendered the hardcoded
         "thinking trace" mock. Now it shows an empty state when no
         generation is in flight, and the steps (when present) are
         honestly labelled as generation-progress indicators rather
         than as a real AI thinking trace. -->
    <div v-if="!hasThinkingContent(props.sampleThinkingSteps)" class="empty-state">
      <TipsIcon class="empty-icon" />
      <p class="empty-title">暂无思维链</p>
      <p class="empty-desc">
        触发一次内容生成后，生成进度会显示在这里。
      </p>
    </div>
    <ThinkingChainViewer v-else :steps="sampleThinkingSteps" />
  </div>
</template>

<style lang="less" scoped>
.panel-content {
  padding: 20px 16px;
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  padding: 40px 16px;
  color: #86868b;
}

.empty-icon {
  font-size: 36px;
  color: #c7c7cc;
  margin-bottom: 12px;
}

.empty-title {
  font-size: 14px;
  font-weight: 500;
  color: #6c6c70;
  margin: 0 0 6px 0;
}

.empty-desc {
  font-size: 12px;
  line-height: 1.5;
  color: #86868b;
  margin: 0;
  max-width: 240px;
}
</style>
