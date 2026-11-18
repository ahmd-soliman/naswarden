<script setup lang="ts">
import { defineComponent, onErrorCaptured, ref, watch } from 'vue'

// Keeps a rendering error inside one part of the page. Without it a single
// bad field (a null where a list was expected) blanked the whole dashboard;
// now that part says it failed and the rest keeps updating.
const props = defineProps<{
  // What failed, for the message: "Pools", "the dashboard"...
  label: string
  // Changes on every update; a failed part tries again when it does.
  resetKey?: unknown
  // The whole page failed: the message stands alone, with page margins.
  page?: boolean
}>()

const failed = ref<string | null>(null)

// Only errors while drawing take the part over. One in a click handler or a
// watcher leaves the page as it is and goes on to Vue's normal logging.
// info is "render function" in development, an error-reference link with
// the same code (0 setup, 1 render) in production builds.
const DRAWING = /^(setup function|render function)$|#runtime-[01]$/

onErrorCaptured((err, _instance, info) => {
  if (!DRAWING.test(info)) return true
  failed.value = err instanceof Error ? err.message : String(err)
  console.error(`NasWarden could not show ${props.label}:`, err)
  return false
})

watch(
  () => props.resetKey,
  () => {
    failed.value = null
  },
)

// The slot is rendered by this inner component, not by the boundary itself:
// Vue reports a render error to the parents of the component that threw, so
// the boundary only hears about errors from below it.
const Content = defineComponent({
  setup(_, { slots }) {
    return () => slots.default?.()
  },
})
</script>

<template>
  <p v-if="failed" class="render-error" :class="{ 'render-error--page': page }" role="alert">
    <strong>Couldn't show {{ label }}.</strong>
    <span>{{ failed }}</span>
    <span class="render-error__hint">NasWarden tries again on the next update. Please report this if it keeps happening.</span>
  </p>
  <Content v-else><slot /></Content>
</template>

<style scoped>
/* Same look as the "Can't get data from TrueNAS" banner */
.render-error {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  margin: 0 0 1rem;
  padding: 0.75rem 1rem;
  border: 1px solid color-mix(in srgb, var(--crit-t) 45%, transparent);
  border-left-width: 4px;
  border-radius: 8px;
  background: color-mix(in srgb, var(--crit-t) 10%, var(--card-bg));
  font-size: 0.9rem;
  overflow-wrap: anywhere;
}
.render-error--page {
  margin: 1.25rem clamp(1rem, 2vw, 2.5rem);
}
.render-error strong {
  color: var(--crit-t);
}
.render-error__hint {
  color: var(--text-dim);
}
</style>
