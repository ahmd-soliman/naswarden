<script setup lang="ts">
import { computed } from 'vue'
import { isCleanStop, memberBadge } from '../composables/useStacks'
import type { Stack } from '../composables/useStacks'

const props = defineProps<{ stack: Stack }>()
defineEmits<{ 'open-container': [name: string] }>()

const composeFiles = computed(() => props.stack.composeFile.split(',').filter(Boolean))
const cleanStops = computed(() => props.stack.members.filter(isCleanStop).length)

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
  <div class="drawer__section">
    <h3>Overview</h3>
    <div class="drawer__kv"><span>Compose project</span><span>{{ stack.name }}</span></div>
    <div v-for="f in composeFiles" :key="f" class="drawer__kv"><span>Compose file</span><span>{{ f }}</span></div>
    <div class="drawer__kv">
      <span>Containers</span>
      <span>
        <template v-if="stack.health === 'gray'">
          All {{ stack.members.length }} stopped<template v-if="cleanStops > 0"> ({{ cleanStops }} cleanly)</template>
        </template>
        <template v-else>
          {{ stack.running }} of {{ stack.total }} running<template v-if="cleanStops > 0">
            &middot; {{ cleanStops }} stopped cleanly</template
          >
        </template>
      </span>
    </div>
    <div class="drawer__kv"><span>CPU (total)</span><span>{{ stack.cpu.toFixed(1) }}%</span></div>
    <div class="drawer__kv"><span>Memory (total)</span><span>{{ formatBytes(stack.mem) }}</span></div>
  </div>

  <div class="drawer__section">
    <h3>Containers</h3>
    <button
      v-for="m in stack.members"
      :key="m.name"
      class="drawer__row"
      @click="$emit('open-container', m.name)"
    >
      <span class="drawer__row-main">
        <span class="drawer__row-name">{{ m.name }}</span>
        <span class="drawer__row-meta">
          <template v-if="m.state === 'running'">CPU {{ m.cpu_percent.toFixed(1) }}% &middot; {{ formatBytes(m.mem_used) }}</template>
          <template v-else>{{ m.status }}</template>
        </span>
      </span>
      <span class="drawer__row-end">
        <span class="badge" :class="memberBadge(m).cls">{{ memberBadge(m).label }}</span>
        <span class="drawer__chev" aria-hidden="true">&rsaquo;</span>
      </span>
    </button>
  </div>
</template>
