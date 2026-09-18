<script setup lang="ts">
import PoolCard from './components/PoolCard.vue'
import { usePoolSocket } from './composables/usePoolSocket'

const { pools, connected } = usePoolSocket()
</script>

<template>
  <div class="app">
    <header class="app__header">
      <h1>naswarden</h1>
      <span class="conn" :class="{ 'conn--live': connected }">
        {{ connected ? 'live' : 'reconnecting…' }}
      </span>
    </header>

    <main class="grid">
      <PoolCard v-for="pool in pools" :key="pool.name" :pool="pool" />
      <p v-if="pools.length === 0" class="empty">Waiting for data…</p>
    </main>
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

.app__header h1 {
  font-size: 1.5rem;
  font-family: ui-monospace, monospace;
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

.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 1rem;
}

.empty {
  color: var(--text-dim);
}
</style>
