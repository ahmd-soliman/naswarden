<script setup lang="ts">
import { computed, reactive } from 'vue'
import type { ProtectionTask } from '../composables/usePoolSocket'
import {
  groupByKind,
  lastRunText,
  protectionAttention,
  scheduleText,
  startsCollapsed,
  taskBadge,
  taskKey,
} from '../composables/protection'

const props = defineProps<{ tasks: ProtectionTask[]; active?: string; nowSec: number }>()
defineEmits<{ select: [task: ProtectionTask] }>()

const groups = computed(() => groupByKind(props.tasks))

// Per-kind fold: what the person chose wins over the default.
const chosen = reactive<Record<string, boolean>>({})
const collapsed = (kind: string, tasks: ProtectionTask[]) => chosen[kind] ?? startsCollapsed(tasks)
const toggle = (kind: string, tasks: ProtectionTask[]) => (chosen[kind] = !collapsed(kind, tasks))

// What a folded kind says about itself, so folding never hides a problem.
function foldedNote(tasks: ProtectionTask[]) {
  const a = protectionAttention(tasks)
  if (a.count > 0) return { text: [a.failed ? `${a.failed} failed` : '', a.overdue ? `${a.overdue} overdue` : ''].filter(Boolean).join(' · '), tone: a.level }
  return { text: 'all OK', tone: 'ok' }
}
</script>

<template>
  <div class="prot">
    <div v-for="g in groups" :key="g.kind" class="prot__group">
      <button
        type="button"
        class="prot__head"
        :aria-expanded="!collapsed(g.kind, g.tasks)"
        :aria-controls="`prot-${g.kind}`"
        @click="toggle(g.kind, g.tasks)"
      >
        <span class="prot__chev" aria-hidden="true">{{ collapsed(g.kind, g.tasks) ? '▸' : '▾' }}</span>
        <span class="prot__kind">{{ g.label }}</span>
        <span class="prot__count">{{ g.tasks.length }}</span>
        <span v-if="collapsed(g.kind, g.tasks)" class="prot__note" :class="`prot__note--${foldedNote(g.tasks).tone}`">
          {{ foldedNote(g.tasks).text }}
        </span>
      </button>
      <ul v-show="!collapsed(g.kind, g.tasks)" :id="`prot-${g.kind}`" class="prot__list" :aria-label="g.label">
        <li v-for="t in g.tasks" :key="taskKey(t)">
          <button
            type="button"
            class="row"
            :class="[`row--${taskBadge(t).tone}`, { 'row--active': active === taskKey(t), 'row--off': t.state === 'DISABLED' }]"
            :aria-pressed="active === taskKey(t)"
            @click="$emit('select', t)"
          >
            <span class="row__main">
              <span class="row__name">{{ t.name }}</span>
              <span class="row__meta">
                <template v-if="t.kind !== 'snapshot' && t.source">{{ t.source }} · </template>{{ scheduleText(t.schedule, t.window) || 'no schedule' }}
              </span>
              <span class="row__meta">{{ lastRunText(t, nowSec) }}</span>
              <span v-if="t.state === 'FAILED' && t.error" class="row__error" :title="t.error">{{ t.error }}</span>
              <span v-else-if="t.state === 'RUNNING' && t.progress_description" class="row__meta">{{ t.progress_description }}</span>
            </span>
            <span v-if="t.state === 'RUNNING' && t.progress_percent !== undefined" class="row__bar" aria-hidden="true">
              <span :style="{ width: `${Math.min(100, Math.max(0, t.progress_percent))}%` }" />
            </span>
            <span class="badge" :class="`badge--${taskBadge(t).tone}`">{{ taskBadge(t).label }}</span>
          </button>
        </li>
      </ul>
    </div>
  </div>
</template>

<style scoped>
.prot {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
.prot__head {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  width: 100%;
  padding: 0.2rem 0.1rem;
  background: none;
  border: 0;
  color: var(--text);
  font: inherit;
  font-weight: 600;
  font-size: 0.9rem;
  cursor: pointer;
  text-align: left;
}
.prot__chev {
  color: var(--text-dim);
  width: 0.8rem;
}
.prot__count {
  color: var(--text-dim);
  font-weight: 400;
  font-size: 0.8rem;
}
.prot__note {
  font-size: 0.78rem;
  font-weight: 500;
  color: var(--text-dim);
}
.prot__note--crit {
  color: var(--crit-t);
}
.prot__note--warn {
  color: var(--warn-t);
}
.prot__note--ok {
  color: var(--ok-t);
}
.prot__list {
  list-style: none;
  margin: 0.35rem 0 0;
  padding: 0;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 24rem), 1fr));
  gap: 0.5rem;
}
.row {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  width: 100%;
  height: 100%;
  padding: 0.6rem 0.85rem;
  background: var(--card-bg);
  color: var(--text);
  border: 1px solid var(--border);
  border-left: 3px solid var(--gray);
  border-radius: 8px;
  font: inherit;
  text-align: left;
  cursor: pointer;
}
.row:hover,
.row--active {
  border-color: var(--text-dim);
}
.row--green {
  border-left-color: var(--ok);
}
.row--yellow {
  border-left-color: var(--warn);
}
.row--red {
  border-left-color: var(--crit);
}
.row--blue {
  border-left-color: var(--accent, var(--pool));
}
.row--off {
  opacity: 0.7;
}
.row__main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
}
.row__name {
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.row__meta {
  color: var(--text-dim);
  font-size: 0.78rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.row__error {
  color: var(--crit-t);
  font-size: 0.78rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.row__bar {
  width: 4rem;
  height: 0.35rem;
  border-radius: 999px;
  background: color-mix(in srgb, var(--text-dim) 25%, transparent);
  overflow: hidden;
  flex: none;
}
.row__bar > span {
  display: block;
  height: 100%;
  background: var(--pool);
}
.badge {
  flex: none;
  font-size: 0.72rem;
  font-weight: 600;
  padding: 0.15rem 0.55rem;
  border-radius: 999px;
  text-transform: uppercase;
  letter-spacing: 0.03em;
  white-space: nowrap;
}
.badge--blue {
  background: color-mix(in srgb, var(--pool) 18%, transparent);
  color: var(--text);
}
</style>
