<script setup lang="ts">
import { computed } from 'vue'
import type { ProtectionTask } from '../composables/usePoolSocket'
import { durationText, kindLabel, retentionText, scheduleText, taskBadge } from '../composables/protection'

const props = defineProps<{ task: ProtectionTask }>()

const badge = computed(() => taskBadge(props.task))
const schedule = computed(() => scheduleText(props.task.schedule, props.task.window))
const retention = computed(() => retentionText(props.task.retention))
const when = (ts: number) =>
  new Date(ts * 1000).toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
const isCron = computed(() => !!props.task.schedule && schedule.value !== props.task.schedule)
</script>

<template>
  <div class="drawer__section">
    <h3>{{ kindLabel(task.kind) }}</h3>
    <div class="drawer__kv">
      <span>State</span>
      <span class="badge" :class="`badge--${badge.tone}`">{{ badge.label }}</span>
    </div>
    <div v-if="task.state === 'OK' && task.overdue" class="drawer__kv">
      <span>Overdue</span>
      <span>A scheduled run since the last one did not happen</span>
    </div>
    <div v-if="!task.enabled" class="drawer__kv"><span>Enabled</span><span>No</span></div>
    <div v-if="task.direction" class="drawer__kv"><span>Direction</span><span>{{ task.direction === 'PULL' ? 'Pull (remote to this NAS)' : 'Push (this NAS to remote)' }}</span></div>
  </div>

  <div v-if="task.state === 'FAILED' && task.error" class="drawer__section">
    <h3>Error</h3>
    <pre class="err">{{ task.error }}</pre>
  </div>

  <div v-if="task.state === 'RUNNING'" class="drawer__section">
    <h3>Progress</h3>
    <div v-if="task.progress_percent !== undefined" class="drawer__kv"><span>Done</span><span>{{ Math.round(task.progress_percent) }}%</span></div>
    <p v-if="task.progress_description" class="note">{{ task.progress_description }}</p>
  </div>

  <div class="drawer__section">
    <h3>Schedule</h3>
    <div class="drawer__kv"><span>Runs</span><span>{{ schedule || 'No schedule' }}</span></div>
    <div v-if="isCron" class="drawer__kv"><span>Cron</span><code>{{ task.schedule }}</code></div>
    <div v-if="retention" class="drawer__kv"><span>Keeps</span><span>{{ retention }}</span></div>
  </div>

  <div class="drawer__section">
    <h3>Last run</h3>
    <p v-if="task.time_started === undefined && task.time_finished === undefined" class="drawer__empty">TrueNAS has no run recorded for this task.</p>
    <template v-else>
      <div v-if="task.time_started !== undefined" class="drawer__kv"><span>Started</span><span>{{ when(task.time_started) }}</span></div>
      <div v-if="task.time_finished !== undefined" class="drawer__kv"><span>{{ task.kind === 'snapshot' ? 'Taken' : 'Ended' }}</span><span>{{ when(task.time_finished) }}</span></div>
      <div v-if="task.duration_seconds !== undefined" class="drawer__kv"><span>Duration</span><span>{{ durationText(task.duration_seconds) }}</span></div>
      <div v-if="task.last_snapshot" class="drawer__kv"><span>Snapshot</span><code class="wrap">{{ task.last_snapshot }}</code></div>
    </template>
  </div>

  <div class="drawer__section">
    <h3>{{ task.kind === 'snapshot' ? 'Dataset' : 'Covers' }}</h3>
    <div class="drawer__kv"><span>{{ task.kind === 'snapshot' ? 'Dataset' : 'Local path' }}</span><code class="wrap">{{ task.source }}</code></div>
    <div v-if="task.kind === 'snapshot'" class="drawer__kv"><span>Child datasets</span><span>{{ task.recursive ? 'Included' : 'Not included' }}</span></div>
    <div v-if="task.remote" class="drawer__kv"><span>Remote</span><code class="wrap">{{ task.remote }}</code></div>
    <template v-if="task.exclude.length > 0">
      <div class="drawer__kv"><span>Excluded</span><span /></div>
      <ul class="excl">
        <li v-for="e in task.exclude" :key="e"><code class="wrap">{{ e }}</code></li>
      </ul>
    </template>
  </div>
</template>

<style scoped>
.err {
  margin: 0;
  padding: 0.6rem 0.75rem;
  border: 1px solid var(--crit);
  border-radius: 6px;
  background: color-mix(in srgb, var(--crit-t) 10%, transparent);
  color: var(--text);
  font-size: 0.8rem;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.note {
  margin: 0.3rem 0 0;
  color: var(--text-dim);
  font-size: 0.8rem;
}
.wrap {
  overflow-wrap: anywhere;
  text-align: right;
}
.excl {
  margin: 0;
  padding-left: 1rem;
  font-size: 0.8rem;
}
.badge {
  font-size: 0.72rem;
  font-weight: 600;
  padding: 0.15rem 0.55rem;
  border-radius: 999px;
  text-transform: uppercase;
  letter-spacing: 0.03em;
}
.badge--blue {
  background: color-mix(in srgb, var(--pool) 18%, transparent);
  color: var(--text);
}
</style>
