<script setup lang="ts">
withDefaults(defineProps<{
  size?: number
  color?: string
}>(), {
  size: 32,
  color: '#007aff'
})
</script>

<template>
  <svg
    :width="size"
    :height="size"
    viewBox="0 0 32 32"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
  >
    <defs>
      <linearGradient id="xinwikiGradient" x1="0%" y1="0%" x2="100%" y2="100%">
        <stop offset="0%" stop-color="#007AFF" />
        <stop offset="50%" stop-color="#5856D6" />
        <stop offset="100%" stop-color="#AF52DE" />
      </linearGradient>
      <filter id="xinwikiShadow" x="-20%" y="-20%" width="140%" height="140%">
        <feDropShadow dx="0" dy="1" stdDeviation="1" flood-color="#007AFF" flood-opacity="0.2" />
      </filter>
    </defs>
    <rect
      x="2"
      y="2"
      width="28"
      height="28"
      rx="8"
      fill="url(#xinwikiGradient)"
      filter="url(#xinwikiShadow)"
    />
    <path
      d="M9 22V10l7 8 7-8v12"
      stroke="white"
      stroke-width="2.5"
      stroke-linecap="round"
      stroke-linejoin="round"
      fill="none"
    />
    <circle cx="16" cy="8" r="1.5" fill="white" />
    <path
      d="M10 20h12"
      stroke="white"
      stroke-width="2"
      stroke-linecap="round"
      opacity="0.6"
    />
  </svg>
</template>
