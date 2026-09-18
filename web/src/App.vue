<script setup lang="ts">
import { computed } from 'vue'
import PoolCard from './components/PoolCard.vue'
import DatasetCard from './components/DatasetCard.vue'
import ContainerCard from './components/ContainerCard.vue'
import { usePoolSocket } from './composables/usePoolSocket'

const { pools, datasets, containers, connected } = usePoolSocket()

// Highest utilization first -- the datasets closest to trouble should be
// the first thing you see, not buried alphabetically.
const sortedDatasets = computed(() =>
  [...datasets.value].sort((a, b) => b.used / b.quota - a.used / a.quota),
)

// Running containers first, then alphabetical within each group.
const sortedContainers = computed(() =>
  [...containers.value].sort((a, b) => {
    if (a.state === 'running' && b.state !== 'running') return -1
    if (a.state !== 'running' && b.state === 'running') return 1
    return a.name.localeCompare(b.name)
  }),
)
</script>

<template>
  <div class="app">
    <header class="app__header">
      <div class="app__brand">
        <img src="/favicon.svg" alt="" class="app__logo" />
        <h1>naswarden</h1>
      </div>
      <span class="conn" :class="{ 'conn--live': connected }">
        {{ connected ? 'live' : 'reconnecting…' }}
      </span>
    </header>

    <section>
      <h2>Pools</h2>
      <div class="grid">
        <PoolCard v-for="pool in pools" :key="pool.name" :pool="pool" />
        <p v-if="pools.length === 0" class="empty">Waiting for data…</p>
      </div>
    </section>

    <section>
      <h2>Dataset quotas</h2>
      <div class="grid grid--datasets">
        <DatasetCard v-for="dataset in sortedDatasets" :key="dataset.name" :dataset="dataset" />
        <p v-if="datasets.length === 0" class="empty">No datasets with a quota configured.</p>
      </div>
    </section>

    <section v-if="containers.length > 0">
      <h2>Containers</h2>
      <div class="grid grid--datasets">
        <ContainerCard v-for="container in sortedContainers" :key="container.name" :container="container" />
      </div>
    </section>
  </div>
</template>

<style scoped>
.app {
  max-width: 960px;
  margin: 0 auto;
  padding: 2rem 1.5rem;
}

.app__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 1.5rem;
}

.app__brand {
  display: flex;
  align-items: center;
  gap: 0.6rem;
}

.app__logo {
  width: 28px;
  height: 28px;
}

.app__header h1 {
  font-size: 1.5rem;
  font-family: ui-monospace, monospace;
  margin: 0;
}

.conn {
  font-size: 0.8rem;
  color: var(--text-dim);
}

.conn--live::before {
  content: '●';
  color: #22c55e;
  margin-right: 0.4rem;
}

.conn:not(.conn--live)::before {
  content: '●';
  color: #eab308;
  margin-right: 0.4rem;
}

section {
  margin-bottom: 2rem;
}

section h2 {
  font-size: 0.95rem;
  font-weight: 600;
  color: var(--text-dim);
  text-transform: uppercase;
  letter-spacing: 0.04em;
  margin: 0 0 0.75rem;
}

.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 1rem;
}

.grid--datasets {
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
}

.empty {
  color: var(--text-dim);
  font-size: 0.9rem;
}
</style>
