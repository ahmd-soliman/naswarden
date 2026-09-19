<script setup lang="ts">
import { computed, reactive } from 'vue'

const props = withDefaults(defineProps<{ candidates: string[]; size?: number }>(), { size: 24 })

// Public CDN mirror of the Dashboard Icons set. The browser fetches these
// directly (naswarden itself makes no outbound request); if the CDN is
// unreachable or a slug has no icon, the next candidate is tried and then
// the generic stack glyph is shown -- the page never depends on it.
const ICON_BASE = 'https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/svg/'

// Slugs that already failed this session, so a 404 is not retried on every
// render or for every card that shares it.
const failed = reactive(new Set<string>())

const current = computed(() => props.candidates.find((c) => !failed.has(c)))
const url = computed(() => {
  const slug = current.value
  if (!slug) return ''
  return slug === 'naswarden' ? '/favicon.svg' : `${ICON_BASE}${encodeURIComponent(slug)}.svg`
})
</script>

<template>
  <img
    v-if="current"
    :key="current"
    class="app-icon"
    :src="url"
    :width="size"
    :height="size"
    alt=""
    loading="lazy"
    referrerpolicy="no-referrer"
    @error="failed.add(current!)"
  />
  <svg
    v-else
    class="app-icon app-icon--fallback"
    :width="size"
    :height="size"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="2"
    aria-hidden="true"
  >
    <polygon points="12 2 2 7 12 12 22 7 12 2" />
    <polyline points="2 17 12 22 22 17" />
    <polyline points="2 12 12 17 22 12" />
  </svg>
</template>

<style scoped>
/* Logos are drawn for light backgrounds -- black ones (portainer, ollama,
   tailscale) disappear on a dark card -- so every icon sits on a light tile. */
.app-icon {
  flex-shrink: 0;
  object-fit: contain;
  box-sizing: border-box;
  padding: 2px;
  border-radius: 6px;
  background: #eef0f3;
}

.app-icon--fallback {
  color: var(--stack);
  background: none;
  padding: 0;
}
</style>
