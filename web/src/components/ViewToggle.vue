<script setup lang="ts">
import type { ViewMode } from '../composables/useViewMode'

defineProps<{ label: string }>()
const mode = defineModel<ViewMode>({ required: true })
</script>

<template>
  <div class="view-toggle" role="group" :aria-label="label">
    <button type="button" :aria-pressed="mode === 'cards'" @click="mode = 'cards'">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
        <rect x="3" y="3" width="7" height="7" rx="1" /><rect x="14" y="3" width="7" height="7" rx="1" />
        <rect x="3" y="14" width="7" height="7" rx="1" /><rect x="14" y="14" width="7" height="7" rx="1" />
      </svg>
      Cards
    </button>
    <button type="button" :aria-pressed="mode === 'table'" @click="mode = 'table'">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
        <path d="M3 6h18M3 12h18M3 18h18" />
      </svg>
      Table
    </button>
  </div>
</template>

<style scoped>
.view-toggle {
  display: inline-flex;
  border: 1px solid var(--border);
  border-radius: 8px;
  overflow: hidden;
}
.view-toggle button {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  background: transparent;
  border: 0;
  color: var(--text-dim);
  font-size: 0.8rem;
  padding: 0.3rem 0.65rem;
  cursor: pointer;
}
.view-toggle button + button {
  border-left: 1px solid var(--border);
}
.view-toggle button[aria-pressed='true'] {
  background: rgba(255, 255, 255, 0.08);
  color: var(--text);
}
.view-toggle button:hover {
  color: var(--text);
}
.view-toggle button:focus-visible {
  outline: 2px solid var(--ok-t);
  outline-offset: -2px;
}
.view-toggle svg {
  width: 14px;
  height: 14px;
}
</style>
