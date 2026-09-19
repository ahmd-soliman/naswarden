<script setup lang="ts">
import { computed } from 'vue'
import type { Pool, ReplicationTask } from '../composables/usePoolSocket'

const props = defineProps<{
  pool: Pool
  replications?: ReplicationTask[]
}>()

const poolReplications = computed(() => {
  if (!props.replications) return []
  return props.replications.filter(
    (t) => t.target_pool === props.pool.name || t.source_pools.includes(props.pool.name),
  )
})

function tempClass(c: number): string {
  if (c < 45) return 'temp-badge--cool'
  if (c < 55) return 'temp-badge--warm'
  return 'temp-badge--hot'
}

function repStatusClass(task: ReplicationTask): string {
  const status = (task.job_state || task.state).toUpperCase()
  if (status === 'SUCCESS' || status === 'FINISHED') return 'badge--green'
  if (status === 'RUNNING') return 'badge--blue'
  if (status === 'ERROR' || status === 'FAILED') return 'badge--red'
  return 'badge--gray'
}

function formatDuration(sec: number): string {
  if (sec < 60) return `${sec}s`
  const mins = Math.floor(sec / 60)
  const remSec = sec % 60
  if (mins < 60) return `${mins}m ${remSec}s`
  const hours = Math.floor(mins / 60)
  const remMins = mins % 60
  return `${hours}h ${remMins}m`
}

function formatDate(ts: number): string {
  const date = new Date(ts * 1000)
  return date.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}
</script>

<template>
  <div class="drawer__section">
    <h3>Layout</h3>
    <div class="drawer__kv"><span>Fragmentation</span><span>{{ pool.fragmentation }}%</span></div>
    <div v-for="vdev in pool.vdevs" :key="vdev.name" class="drawer__kv">
      <span>{{ vdev.name }}</span><span>{{ vdev.type }} ({{ vdev.children.length }} disks)</span>
    </div>
  </div>

  <div class="drawer__section">
    <h3>Disk members</h3>
    <div v-if="pool.vdevs.length === 0" class="drawer__empty">No disk info available.</div>
    <template v-for="vdev in pool.vdevs" :key="vdev.name">
      <div v-for="disk in vdev.children" :key="disk.disk" class="drawer__kv">
        <span class="disk-col">
          <span class="disk-name">{{ disk.disk }}</span>
          <span v-if="disk.standby" class="temp-badge temp-badge--standby" title="Disk is spun down (standby)">
            Standby
          </span>
          <span
            v-else-if="disk.temperature_c !== undefined && disk.temperature_c !== null"
            class="temp-badge"
            :class="tempClass(disk.temperature_c)"
            :title="`Drive temperature: ${disk.temperature_c}°C`"
          >
            {{ disk.temperature_c }}°C
          </span>
        </span>
        <span>
          {{ disk.status }}
          <template v-if="disk.read_errors + disk.write_errors + disk.checksum_errors > 0">
            ({{ disk.read_errors + disk.write_errors + disk.checksum_errors }} errors)
          </template>
        </span>
      </div>
    </template>
  </div>

  <div v-if="poolReplications.length > 0" class="drawer__section">
    <h3>Replication Tasks</h3>
    <div class="rep-list">
      <div v-for="task in poolReplications" :key="task.id" class="rep-card">
        <div class="rep-card__header">
          <span class="rep-card__name">{{ task.name }}</span>
          <span class="badge" :class="repStatusClass(task)">{{ task.job_state || task.state }}</span>
        </div>
        <div class="rep-card__body">
          <div class="drawer__kv">
            <span>Target</span>
            <span class="rep-code">{{ task.target_dataset }}</span>
          </div>
          <div v-if="task.last_snapshot" class="drawer__kv">
            <span>Last Snapshot</span>
            <span class="rep-code rep-snapshot" :title="task.last_snapshot">{{ task.last_snapshot }}</span>
          </div>
          <div v-if="task.duration_seconds !== undefined" class="drawer__kv">
            <span>Duration</span>
            <span>{{ formatDuration(task.duration_seconds) }}</span>
          </div>
          <div v-if="task.time_finished" class="drawer__kv">
            <span>Finished</span>
            <span>{{ formatDate(task.time_finished) }}</span>
          </div>
          <div v-if="task.progress_description" class="rep-card__progress">
            {{ task.progress_description }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.disk-col {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
}

.disk-name {
  font-family: ui-monospace, monospace;
}

.temp-badge {
  font-size: 0.7rem;
  font-weight: 600;
  padding: 0.1rem 0.4rem;
  border-radius: 4px;
  font-family: ui-monospace, monospace;
}

.temp-badge--cool {
  background: rgba(34, 197, 94, 0.15);
  color: #22c55e;
  border: 1px solid rgba(34, 197, 94, 0.3);
}

.temp-badge--warm {
  background: rgba(234, 179, 8, 0.15);
  color: #eab308;
  border: 1px solid rgba(234, 179, 8, 0.3);
}

.temp-badge--hot {
  background: rgba(239, 68, 68, 0.15);
  color: #ef4444;
  border: 1px solid rgba(239, 68, 68, 0.3);
}

.temp-badge--standby {
  background: rgba(148, 163, 184, 0.12);
  color: #94a3b8;
  border: 1px solid rgba(148, 163, 184, 0.25);
  font-style: italic;
}

.rep-list {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.rep-card {
  background: var(--bg-card, rgba(255, 255, 255, 0.03));
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 0.75rem;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.rep-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}

.rep-card__name {
  font-weight: 600;
  font-size: 0.9rem;
}

.rep-card__body {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.rep-code {
  font-family: ui-monospace, monospace;
  font-size: 0.8rem;
}

.rep-snapshot {
  max-width: 180px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rep-card__progress {
  margin-top: 0.25rem;
  font-size: 0.75rem;
  color: var(--text-dim);
  font-family: ui-monospace, monospace;
  background: rgba(0, 0, 0, 0.2);
  padding: 0.3rem 0.5rem;
  border-radius: 4px;
  word-break: break-all;
}

.badge {
  font-size: 0.7rem;
  font-weight: 700;
  padding: 0.15rem 0.5rem;
  border-radius: 999px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.badge--green {
  background: rgba(34, 197, 94, 0.15);
  color: #22c55e;
  border: 1px solid rgba(34, 197, 94, 0.3);
}

.badge--blue {
  background: rgba(59, 130, 246, 0.15);
  color: #3b82f6;
  border: 1px solid rgba(59, 130, 246, 0.3);
}

.badge--red {
  background: rgba(239, 68, 68, 0.15);
  color: #ef4444;
  border: 1px solid rgba(239, 68, 68, 0.3);
}

.badge--gray {
  background: rgba(156, 163, 175, 0.15);
  color: #9ca3af;
  border: 1px solid rgba(156, 163, 175, 0.3);
}
</style>
