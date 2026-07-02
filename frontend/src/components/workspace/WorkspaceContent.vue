<script setup lang="ts">
import { ChevronRightIcon } from 'tdesign-icons-vue-next'

withDefaults(defineProps<{
  breadcrumb?: string[]
}>(), {
  breadcrumb: () => ['知识库', '当前页面'],
})
</script>

<template>
  <main class="main-content">
    <div class="content-scroll">
      <div class="content-header">
        <div class="breadcrumb">
          <template v-for="(item, i) in breadcrumb" :key="i">
            <span class="breadcrumb-item" :class="{ active: i === breadcrumb.length - 1 }">{{ item }}</span>
            <ChevronRightIcon v-if="i < breadcrumb.length - 1" class="breadcrumb-separator" />
          </template>
        </div>
        <div class="content-actions">
          <slot name="header-actions" />
        </div>
      </div>

      <div class="content-area">
        <slot />
      </div>
    </div>
  </main>
</template>

<style lang="less" scoped>
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
  display: flex;
  flex-direction: column;
  min-height: 0;

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
  margin-bottom: 20px;
  flex-shrink: 0;
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
  flex: 1;
  padding: 24px;
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
}

@media (max-width: 768px) {
  .main-content {
    width: 100%;
  }

  .content-scroll {
    padding: 16px;
  }

  .content-area {
    padding: 20px;
    border-radius: 12px;
  }
}
</style>
