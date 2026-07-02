<script setup lang="ts">
/**
 * RoutePlaceholder - backdrop for routes whose real UI lives in a
 * globally-mounted modal (currently only IntegrationsModal).
 *
 * P1 fix: previously this had `display: none`, so if the modal ever
 * failed to mount (e.g. JS error in IntegrationsModal) the user saw
 * a blank white page with no way forward. Now it renders a minimal
 * fallback with a link back to the knowledge base list, so the user
 * is never stuck on an empty route.
 */
</script>

<template>
  <div class="route-placeholder">
    <p class="placeholder-text">正在打开…</p>
    <router-link to="/platform/knowledge-bases" class="placeholder-link">
      返回知识库
    </router-link>
  </div>
</template>

<style scoped>
.route-placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  height: 100%;
  color: #86868b;
  font-size: 14px;
}

.placeholder-link {
  color: #007aff;
  text-decoration: none;
  font-size: 13px;
}

.placeholder-link:hover {
  text-decoration: underline;
}
</style>
