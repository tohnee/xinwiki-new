<script setup lang="ts">
import ThinkingChainViewer from '@/components/ThinkingChainViewer.vue'
import type { ThinkingStep } from './useGeneration'

defineProps<{
  sampleThinkingSteps: ThinkingStep[]
}>()
</script>

<template>
  <div class="panel-content">
    <ThinkingChainViewer :steps="sampleThinkingSteps" />
  </div>
</template>

<style lang="less" scoped>
.panel-content {
  padding: 20px 16px;
}
</style>
