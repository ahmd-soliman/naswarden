<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import ServerCard from './components/ServerCard.vue'
import PoolCard from './components/PoolCard.vue'
import DatasetCard from './components/DatasetCard.vue'
import ContainerCard from './components/ContainerCard.vue'
import { usePoolSocket } from './composables/usePoolSocket'

const { server, pools, datasets, containers, connected } = usePoolSocket()

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

const hasContainers = computed(() => containers.value.length > 0)

// 'all' shows every section at once (the default, glanceable overview);
// picking a rail item filters down to just that section. Filtering never
// hides data the "All" view wouldn't already show -- it's a convenience,
// not a different data set.
const activeSection = ref<'all' | 'server' | 'pools' | 'datasets' | 'containers'>('all')

function selectSection(name: typeof activeSection.value) {
  activeSection.value = name
  scrollSection.value = name === 'all' ? scrollSection.value : name
}

// While in "All" mode, track which section is currently scrolled into
// view and highlight the matching rail item -- without hiding any other
// section. Paused while a specific section is filtered in, since every
// other section is already hidden then.
const scrollSection = ref<'server' | 'pools' | 'datasets' | 'containers'>('server')
let observer: IntersectionObserver | null = null

function initObserver() {
  observer?.disconnect()
  observer = new IntersectionObserver(
    (entries) => {
      if (activeSection.value !== 'all') return
      for (const entry of entries) {
        if (entry.isIntersecting) {
          scrollSection.value = (entry.target as HTMLElement).dataset.section as typeof scrollSection.value
        }
      }
    },
    { rootMargin: '-35% 0px -55% 0px', threshold: 0 },
  )
  document.querySelectorAll('[data-section]').forEach((el) => observer!.observe(el))
}

onMounted(() => nextTick(initObserver))
// Containers section only exists in the DOM once containers show up --
// re-observe when that changes so the rail highlight tracks it too.
watch(hasContainers, () => nextTick(initObserver))
onBeforeUnmount(() => observer?.disconnect())

function railClass(section: 'server' | 'pools' | 'datasets' | 'containers') {
  return {
    active: activeSection.value === section,
    'scroll-active': activeSection.value === 'all' && scrollSection.value === section,
  }
}
</script>

<template>
  <div class="app">
    <header class="app__topbar">
      <div class="app__brand">
        <img src="/favicon.svg" alt="" class="app__logo" />
        <h1>naswarden</h1>
      </div>
      <span class="conn" :class="{ 'conn--live': connected }">
        {{ connected ? 'live' : 'reconnecting…' }}
      </span>
    </header>

    <div class="app__layout">
      <nav class="rail">
        <button class="rail__item" :class="{ active: activeSection === 'all' }" @click="selectSection('all')">
          All
        </button>
        <button
          v-if="server"
          class="rail__item rail__item--server"
          :class="railClass('server')"
          @click="selectSection('server')"
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="2" width="20" height="8" rx="2"/><rect x="2" y="14" width="20" height="8" rx="2"/><path d="M6 6h.01M6 18h.01"/></svg>
          Server
        </button>
        <button class="rail__item rail__item--pool" :class="railClass('pools')" @click="selectSection('pools')">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><ellipse cx="12" cy="5" rx="8" ry="3"/><path d="M4 5v14c0 1.7 3.6 3 8 3s8-1.3 8-3V5"/><path d="M4 12c0 1.7 3.6 3 8 3s8-1.3 8-3"/></svg>
          Pools
        </button>
        <button
          class="rail__item rail__item--dataset"
          :class="railClass('datasets')"
          @click="selectSection('datasets')"
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7z"/></svg>
          Datasets
        </button>
        <button
          v-if="hasContainers"
          class="rail__item rail__item--container"
          :class="railClass('containers')"
          @click="selectSection('containers')"
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 8l-9-5-9 5 9 5 9-5z"/><path d="M3 8v8l9 5 9-5V8"/><path d="M12 13v8"/></svg>
          Containers
        </button>
      </nav>

      <main class="app__content">
        <section v-if="server" data-section="server" v-show="activeSection === 'all' || activeSection === 'server'">
          <h2>Server</h2>
          <ServerCard :server="server" />
        </section>

        <section data-section="pools" v-show="activeSection === 'all' || activeSection === 'pools'">
          <h2>Pools</h2>
          <div class="grid">
            <PoolCard v-for="pool in pools" :key="pool.name" :pool="pool" />
            <p v-if="pools.length === 0" class="empty">Waiting for data…</p>
          </div>
        </section>

        <section data-section="datasets" v-show="activeSection === 'all' || activeSection === 'datasets'">
          <h2>Dataset quotas</h2>
          <div class="grid grid--datasets">
            <DatasetCard v-for="dataset in sortedDatasets" :key="dataset.name" :dataset="dataset" />
            <p v-if="datasets.length === 0" class="empty">No datasets with a quota configured.</p>
          </div>
        </section>

        <section
          v-if="hasContainers"
          data-section="containers"
          v-show="activeSection === 'all' || activeSection === 'containers'"
        >
          <h2>Containers</h2>
          <div class="grid grid--datasets">
            <ContainerCard v-for="container in sortedContainers" :key="container.name" :container="container" />
          </div>
        </section>
      </main>
    </div>
  </div>
</template>

<style scoped>
.app {
  max-width: 1400px;
  margin: 0 auto;
  padding: 2rem 1.5rem;
}

.app__topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-bottom: 1.25rem;
  margin-bottom: 1.5rem;
  border-bottom: 1px solid var(--border);
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

.app__topbar h1 {
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

.app__layout {
  display: flex;
  gap: 2rem;
  align-items: flex-start;
}

.rail {
  flex: 0 0 150px;
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  position: sticky;
  top: 1.5rem;
}

.rail__item {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  background: transparent;
  border: 1px solid transparent;
  color: var(--text-dim);
  padding: 0.55rem 0.75rem;
  border-radius: 8px;
  font-size: 0.85rem;
  cursor: pointer;
  text-align: left;
  width: 100%;
  font-family: inherit;
}

.rail__item svg {
  width: 16px;
  height: 16px;
  flex-shrink: 0;
}

.rail__item.active,
.rail__item.scroll-active {
  background: var(--card-bg);
  border-color: var(--border);
  color: var(--text);
}

.rail__item--server.active svg,
.rail__item--server.scroll-active svg {
  color: var(--server);
}
.rail__item--pool.active svg,
.rail__item--pool.scroll-active svg {
  color: var(--pool);
}
.rail__item--dataset.active svg,
.rail__item--dataset.scroll-active svg {
  color: var(--dataset);
}
.rail__item--container.active svg,
.rail__item--container.scroll-active svg {
  color: var(--container);
}

.app__content {
  flex: 1;
  min-width: 0;
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

@media (max-width: 700px) {
  .app__layout {
    flex-direction: column;
  }
  .rail {
    flex-direction: row;
    position: static;
    width: 100%;
    overflow-x: auto;
    padding-bottom: 0.25rem;
  }
  .rail__item {
    flex: 0 0 auto;
    white-space: nowrap;
  }
}
</style>
