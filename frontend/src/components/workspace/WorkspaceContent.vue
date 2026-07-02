<script setup lang="ts">
import {
  ChevronRightIcon,
  StarFilledIcon,
  SearchIcon,
  TipsIcon,
} from 'tdesign-icons-vue-next'

withDefaults(defineProps<{
  showWelcome?: boolean
  breadcrumb?: string[]
}>(), {
  showWelcome: false,
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
        <div v-if="showWelcome" class="welcome-screen">
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

  .welcome-features {
    grid-template-columns: 1fr;
  }
}
</style>
