<script setup lang="ts">
import type { Service } from '../composables/usePoolSocket'
import { serviceLabel, serviceTone } from '../composables/services'

defineProps<{ services: Service[]; active?: string }>()
defineEmits<{ select: [name: string] }>()
</script>

<template>
  <ul class="chips" aria-label="Services">
    <li v-for="s in services" :key="s.name">
      <button
        type="button"
        class="chip"
        :class="[`chip--${serviceTone(s)}`, { 'chip--active': active === s.name }]"
        :aria-pressed="active === s.name"
        :title="s.problem ? `${serviceLabel(s.name)} is stopped but set to start on boot` : undefined"
        @click="$emit('select', s.name)"
      >
        <span class="chip__dot" aria-hidden="true" />
        {{ serviceLabel(s.name) }}
        <span class="chip__state">{{ s.problem ? 'stopped' : s.state === 'RUNNING' ? 'running' : s.state.toLowerCase() }}</span>
      </button>
    </li>
  </ul>
</template>

<style scoped>
.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
  list-style: none;
  margin: 0.6rem 0 0;
  padding: 0;
}
.chip {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.2rem 0.65rem;
  border: 1px solid var(--border);
  border-radius: 999px;
  background: var(--card-bg);
  color: var(--text);
  font: inherit;
  font-size: 0.8rem;
  cursor: pointer;
}
.chip:hover,
.chip--active {
  border-color: var(--text-dim);
}
.chip__dot {
  width: 0.5rem;
  height: 0.5rem;
  border-radius: 50%;
  background: var(--gray);
}
.chip--ok .chip__dot {
  background: var(--ok);
}
.chip--crit {
  border-color: var(--crit);
  background: color-mix(in srgb, var(--crit-t) 15%, var(--card-bg));
}
.chip--crit .chip__dot {
  background: var(--crit);
}
/* the state is written out too, so it never rides on colour alone */
.chip__state {
  color: var(--text-dim);
  font-size: 0.72rem;
}
.chip--crit .chip__state {
  color: var(--crit-t);
  font-weight: 600;
}
</style>
