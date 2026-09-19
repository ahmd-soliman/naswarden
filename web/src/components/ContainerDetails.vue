<script setup lang="ts">
import { computed } from 'vue'
import type { Container } from '../composables/usePoolSocket'

const props = defineProps<{ container: Container }>()

const binds = computed(() => props.container.mounts.filter((m) => m.type === 'bind'))
const volumes = computed(() => props.container.mounts.filter((m) => m.type === 'volume'))

function formatStartedAt(iso: string): string {
  if (!iso) return 'unknown'
  return new Date(iso).toLocaleString()
}
</script>

<template>
  <div class="drawer__section">
    <h3>Overview</h3>
    <div class="drawer__kv"><span>Started</span><span>{{ formatStartedAt(container.started_at) }}</span></div>
    <div class="drawer__kv"><span>Restart policy</span><span>{{ container.restart_policy || 'none' }}</span></div>
    <div class="drawer__kv"><span>Command</span><span>{{ container.command || '--' }}</span></div>
  </div>

  <div class="drawer__section">
    <h3>Bind mounts</h3>
    <div v-if="binds.length === 0" class="drawer__empty">No bind mounts.</div>
    <div v-for="m in binds" :key="m.destination" class="drawer__mount">
      {{ m.source }}<span class="drawer__mount-arrow">&rarr;</span>{{ m.destination }}
      <span v-if="m.read_only" class="drawer__ro">read-only</span>
    </div>
  </div>

  <div class="drawer__section">
    <h3>Volumes</h3>
    <div v-if="volumes.length === 0" class="drawer__empty">No named volumes.</div>
    <div v-for="m in volumes" :key="m.destination" class="drawer__mount">
      {{ m.source }}<span class="drawer__mount-arrow">&rarr;</span>{{ m.destination }}
      <span v-if="m.read_only" class="drawer__ro">read-only</span>
    </div>
  </div>

  <div class="drawer__section">
    <h3>Networks</h3>
    <div v-if="container.networks.length === 0" class="drawer__empty">No network info.</div>
    <div v-for="n in container.networks" :key="n.name" class="drawer__kv">
      <span>{{ n.name }}</span><span>{{ n.ip }}</span>
    </div>
  </div>

  <div class="drawer__section">
    <h3>Ports</h3>
    <div v-if="container.ports.length === 0" class="drawer__empty">No published ports.</div>
    <div v-for="p in container.ports" :key="p" class="drawer__kv"><span></span><span>{{ p }}</span></div>
  </div>
</template>
