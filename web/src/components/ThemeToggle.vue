<script setup lang="ts">
import type { Theme } from '../composables/useTheme'

const theme = defineModel<Theme>({ required: true })

const options: { value: Theme; label: string; path: string }[] = [
  { value: 'system', label: 'System', path: 'M4 5h16v11H4zM9 20h6M12 16v4' },
  {
    value: 'light',
    label: 'Light',
    path: 'M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8zM12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4',
  },
  { value: 'dark', label: 'Dark', path: 'M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z' },
]
</script>

<template>
  <div class="theme-toggle" role="group" aria-label="Colour theme">
    <button
      v-for="o in options"
      :key="o.value"
      type="button"
      :aria-pressed="theme === o.value"
      :aria-label="o.label"
      :title="o.label"
      @click="theme = o.value"
    >
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
        <path :d="o.path" />
      </svg>
    </button>
  </div>
</template>

<style scoped>
.theme-toggle {
  display: inline-flex;
  border: 1px solid var(--border);
  border-radius: 8px;
  overflow: hidden;
}
.theme-toggle button {
  display: inline-flex;
  align-items: center;
  background: transparent;
  border: 0;
  color: var(--text-dim);
  padding: 0.3rem 0.5rem;
  cursor: pointer;
}
.theme-toggle button + button {
  border-left: 1px solid var(--border);
}
.theme-toggle button[aria-pressed='true'] {
  background: rgb(var(--tint-rgb) / 0.08);
  color: var(--text);
}
.theme-toggle button:hover {
  color: var(--text);
}
.theme-toggle button:focus-visible {
  outline: 2px solid var(--ok-t);
  outline-offset: -2px;
}
.theme-toggle svg {
  width: 14px;
  height: 14px;
}
</style>
