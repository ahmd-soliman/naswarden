<script setup lang="ts">
import type { Alert } from '../composables/usePoolSocket'

defineProps<{ alerts: Alert[] }>()

function formatDate(ts: number): string {
  if (!ts) return 'Unknown'
  const date = new Date(ts * 1000)
  return date.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}

function timeAgo(ts: number): string {
  if (!ts) return ''
  const diffSec = Math.max(0, Math.floor(Date.now() / 1000 - ts))
  if (diffSec < 60) return `${diffSec}s ago`
  if (diffSec < 3600) return `${Math.floor(diffSec / 60)}m ago`
  if (diffSec < 86400) return `${Math.floor(diffSec / 3600)}h ago`
  return `${Math.floor(diffSec / 86400)}d ago`
}

function levelClass(level: string): string {
  switch (level.toUpperCase()) {
    case 'CRITICAL':
    case 'ERROR':
      return 'badge--red'
    case 'WARNING':
      return 'badge--yellow'
    case 'INFO':
      return 'badge--blue'
    default:
      return 'badge--gray'
  }
}
</script>

<template>
  <div class="drawer__section">
    <h3>Active Host Alerts</h3>
    <p class="drawer__hint">
      Active notifications retrieved directly from TrueNAS middleware. Manage or dismiss notifications in TrueNAS SCALE via the host top-bar bell.
    </p>

    <div v-if="alerts.length === 0" class="drawer__empty">
      No active alerts reported on this host.
    </div>

    <div v-else class="alerts-list">
      <div v-for="alert in alerts" :key="alert.id" class="alert-item">
        <div class="alert-item__header">
          <div class="alert-item__tags">
            <span class="badge" :class="levelClass(alert.level)">{{ alert.level }}</span>
            <span v-if="alert.source" class="alert-item__source">{{ alert.source }}</span>
          </div>
          <span v-if="alert.datetime" class="alert-item__time" :title="formatDate(alert.datetime)">
            {{ timeAgo(alert.datetime) }}
          </span>
        </div>
        <div class="alert-item__message">
          {{ alert.formatted }}
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.drawer__hint {
  font-size: 0.8rem;
  color: var(--text-dim);
  margin-bottom: 0.75rem;
  line-height: 1.4;
}

.alerts-list {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.alert-item {
  background: var(--bg-card, rgba(255, 255, 255, 0.03));
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 0.75rem;
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
}

.alert-item__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}

.alert-item__tags {
  display: flex;
  align-items: center;
  gap: 0.4rem;
}

.alert-item__source {
  font-size: 0.8rem;
  font-weight: 600;
  color: var(--text-dim);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.alert-item__time {
  font-size: 0.8rem;
  color: var(--text-dim);
  font-family: ui-monospace, monospace;
}

.alert-item__message {
  font-size: 0.85rem;
  color: var(--text);
  line-height: 1.4;
  word-break: break-word;
}

.badge {
  font-size: 0.8rem;
  font-weight: 700;
  padding: 0.15rem 0.5rem;
  border-radius: 999px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.badge--red {
  background: rgba(239, 68, 68, 0.15);
  color: var(--crit-t);
  border: 1px solid rgba(239, 68, 68, 0.3);
}

.badge--yellow {
  background: rgba(234, 179, 8, 0.15);
  color: var(--warn-t);
  border: 1px solid rgba(234, 179, 8, 0.3);
}

.badge--blue {
  background: rgba(59, 130, 246, 0.15);
  color: var(--info-t);
  border: 1px solid rgba(59, 130, 246, 0.3);
}

.badge--gray {
  background: rgba(156, 163, 175, 0.15);
  color: #9ca3af;
  border: 1px solid rgba(156, 163, 175, 0.3);
}
</style>
