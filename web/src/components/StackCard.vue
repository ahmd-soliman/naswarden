<script setup lang="ts">
import { computed } from 'vue'
import AppIcon from './AppIcon.vue'
import { iconCandidates, memberTone, stackBadgeLabel } from '../composables/useStacks'
import type { Stack } from '../composables/useStacks'

const props = defineProps<{ stack: Stack; active?: boolean }>()
defineEmits<{ select: [] }>()

const candidates = computed(() => iconCandidates(props.stack))
const memberNames = computed(() => props.stack.members.map((m) => m.name).join(' · '))

function formatBytes(bytes: number): string {
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let value = bytes
  let unitIndex = 0
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024
    unitIndex++
  }
  return `${value.toFixed(1)} ${units[unitIndex]}`
}
</script>

<template>
  <div
    class="stack-card card--clickable"
    :class="{ 'card--open': active, 'stack-card--stopped': stack.health === 'gray' }"
    tabindex="0"
    role="button"
    @click="$emit('select')"
    @keydown.enter.prevent="$emit('select')"
    @keydown.space.prevent="$emit('select')"
  >
    <div class="stack-card__header">
      <span class="stack-card__title">
        <AppIcon :candidates="candidates" :size="22" />
        <span class="stack-card__name" :title="stack.name">{{ stack.name }}</span>
      </span>
      <span class="badge" :class="`badge--${stack.health}`">{{ stackBadgeLabel(stack) }}</span>
    </div>

    <div class="stack-card__strip" aria-hidden="true">
      <i v-for="m in stack.members" :key="m.name" :class="`seg--${memberTone(m)}`" />
    </div>
    <div class="stack-card__members" :title="memberNames">{{ memberNames }}</div>

    <template v-if="stack.running > 0">
      <div class="stack-card__stat">
        <span class="stack-card__stat-label">
          <svg class="metric-glyph" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="4" y="4" width="16" height="16" rx="2"/><rect x="9" y="9" width="6" height="6"/><path d="M9 1v3M15 1v3M9 20v3M15 20v3M1 9h3M1 15h3M20 9h3M20 15h3"/></svg>
          CPU
        </span>
        <span>{{ stack.cpu.toFixed(1) }}%</span>
      </div>
      <div class="stack-card__stat">
        <span class="stack-card__stat-label">
          <svg class="metric-glyph" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="7" width="20" height="10" rx="1"/><path d="M6 7v10M10 7v4M14 7v4M18 7v10"/></svg>
          Memory
        </span>
        <span>{{ formatBytes(stack.mem) }}</span>
      </div>
    </template>
    <div v-else class="stack-card__status">Nothing running</div>
  </div>
</template>

<style scoped>
.stack-card {
  background: var(--card-bg);
  border: 1px solid var(--border);
  --accent: var(--stack);
  border-top: 3px solid var(--accent);
  border-radius: 10px;
  padding: 1rem 1.25rem;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  min-width: 0;
}

/* Muted, not faded: opacity would drag the badge and status text below 4.5:1 */
.stack-card--stopped .stack-card__name {
  color: var(--text-dim);
}

.stack-card__header {
  display: flex;
  flex-wrap: wrap; /* a long badge drops below the name instead of squeezing it mid-word */
  align-items: center;
  justify-content: space-between;
  gap: 0.4rem 0.75rem;
}

.stack-card__title {
  display: flex;
  align-items: center;
  gap: 0.55rem;
  min-width: 0;
  flex: 1 1 auto;
}

.stack-card__name {
  font-size: 0.95rem;
  font-weight: 600;
  font-family: ui-monospace, monospace;
  overflow-wrap: anywhere;
  min-width: 0;
}

.badge {
  font-size: 0.8rem;
  font-weight: 600;
  padding: 0.15rem 0.55rem;
  border-radius: 999px;
  flex-shrink: 0;
  text-transform: uppercase;
  white-space: nowrap;
}

.stack-card__strip {
  display: flex;
  gap: 3px;
}

.stack-card__strip i {
  flex: 1;
  height: 6px;
  border-radius: 999px;
  background: var(--border);
}

.seg--ok {
  background: var(--ok) !important;
}
.seg--crit {
  background: var(--crit) !important;
}
.seg--gray {
  background: var(--gray) !important;
}

.stack-card__members {
  font-size: 0.8rem;
  color: var(--text-dim);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.stack-card__stat {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 0.8rem;
  color: var(--text-dim);
}

.stack-card__stat-label {
  display: flex;
  align-items: center;
}

.metric-glyph {
  width: 14px;
  height: 14px;
  color: var(--text-dim);
  flex-shrink: 0;
  margin-right: 0.3rem;
}

.stack-card__status {
  font-size: 0.8rem;
  color: var(--text-dim);
}
</style>
