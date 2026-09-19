<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import ServerCard from './components/ServerCard.vue'
import PoolCard from './components/PoolCard.vue'
import DatasetCard from './components/DatasetCard.vue'
import ContainerCard from './components/ContainerCard.vue'
import StackCard from './components/StackCard.vue'
import AppIcon from './components/AppIcon.vue'
import DetailDrawer from './components/DetailDrawer.vue'
import ServerDetails from './components/ServerDetails.vue'
import PoolDetails from './components/PoolDetails.vue'
import DatasetDetails from './components/DatasetDetails.vue'
import ContainerDetails from './components/ContainerDetails.vue'
import StackDetails from './components/StackDetails.vue'
import { usePoolSocket } from './composables/usePoolSocket'
import type { Container, Dataset, Pool, ServerInfo } from './composables/usePoolSocket'
import { buildStacks, iconCandidates, isStopped } from './composables/useStacks'
import type { Stack } from './composables/useStacks'

const { server, pools, datasets, containers, connected, updatedAt } = usePoolSocket()

// Data freshness. The backend refreshes every 60s; if it stops getting data
// (TrueNAS unreachable, refresh failing) it broadcasts nothing, so a browser
// that is still connected would keep showing old numbers as "live". Show the
// snapshot's age and call it stale after 2.5 missed refresh intervals.
const REFRESH_SECONDS = 60
const STALE_AFTER_SECONDS = REFRESH_SECONDS * 2.5
const nowMs = ref(Date.now())
let clock: ReturnType<typeof setInterval> | undefined
onMounted(() => (clock = setInterval(() => (nowMs.value = Date.now()), 1000)))
onBeforeUnmount(() => clearInterval(clock))

const ageSeconds = computed(() =>
  updatedAt.value === null ? null : Math.max(0, Math.floor(nowMs.value / 1000 - updatedAt.value)),
)
const isStale = computed(() => connected.value && ageSeconds.value !== null && ageSeconds.value > STALE_AFTER_SECONDS)
const connLabel = computed(() => (!connected.value ? 'reconnecting…' : isStale.value ? 'stale' : 'live'))
const ageLabel = computed(() => {
  const s = ageSeconds.value
  if (s === null) return ''
  if (s < 60) return `updated ${s}s ago`
  if (s < 3600) return `updated ${Math.floor(s / 60)}m ago`
  return `updated ${Math.floor(s / 3600)}h ago`
})

// Highest utilization first -- the datasets closest to trouble should be
// the first thing you see, not buried alphabetically.
const sortedDatasets = computed(() =>
  [...datasets.value].sort((a, b) => b.used / b.quota - a.used / a.quota),
)

// Stacks are built from EVERY container, including stopped members (that is
// how a stack can say "2/3 running"); the flat Containers tab lists only the
// ones that are up, since stopped ones are noise there.
const stacks = computed(() => buildStacks(containers.value))
const hasStacks = computed(() => stacks.value.length > 0)
const liveContainers = computed(() => containers.value.filter((c) => !isStopped(c)))

// Running containers first, then alphabetical within each group.
const sortedContainers = computed(() =>
  [...liveContainers.value].sort((a, b) => {
    if (a.state === 'running' && b.state !== 'running') return -1
    if (a.state !== 'running' && b.state === 'running') return 1
    return a.name.localeCompare(b.name)
  }),
)

const hasContainers = computed(() => liveContainers.value.length > 0)

// 'all' shows every section at once (the default, glanceable overview);
// picking a rail item filters down to just that section. Filtering never
// hides data the "All" view wouldn't already show -- it's a convenience,
// not a different data set.
const activeSection = ref<'all' | 'server' | 'pools' | 'datasets' | 'stacks' | 'containers'>('all')

function selectSection(name: typeof activeSection.value) {
  activeSection.value = name
  scrollSection.value = name === 'all' ? scrollSection.value : name
}

// While in "All" mode, track which section is currently scrolled into
// view and highlight the matching rail item -- without hiding any other
// section. Paused while a specific section is filtered in, since every
// other section is already hidden then.
const scrollSection = ref<'server' | 'pools' | 'datasets' | 'stacks' | 'containers'>('server')
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
watch([hasContainers, hasStacks], () => nextTick(initObserver))
onBeforeUnmount(() => observer?.disconnect())

function railClass(section: 'server' | 'pools' | 'datasets' | 'stacks' | 'containers') {
  return {
    active: activeSection.value === section,
    'scroll-active': activeSection.value === 'all' && scrollSection.value === section,
  }
}

// Click-to-expand detail drawer -- one shared drawer, filled in per
// entity type. Only one card's detail can be open at a time.
type Selected =
  | { kind: 'server'; data: ServerInfo }
  | { kind: 'pool'; data: Pool }
  | { kind: 'dataset'; data: Dataset }
  | { kind: 'stack'; data: Stack }
  // fromStack: opened by drilling down from that stack's drawer, so the
  // drawer offers a way back to it
  | { kind: 'container'; data: Container; fromStack?: string }

const selected = ref<Selected | null>(null)

const drawerTitle = computed(() => {
  if (!selected.value) return ''
  return selected.value.kind === 'server' ? selected.value.data.hostname : selected.value.data.name
})
const drawerBack = computed(() =>
  selected.value?.kind === 'container' ? selected.value.fromStack : undefined,
)

function openStack(name: string) {
  const stack = stacks.value.find((s) => s.name === name)
  if (stack) selected.value = { kind: 'stack', data: stack }
}
function openContainer(name: string, fromStack?: string) {
  const container = containers.value.find((c) => c.name === name)
  if (container) selected.value = { kind: 'container', data: container, fromStack }
}
// The stack card stays highlighted while one of its containers is open in the drawer.
function stackActive(name: string) {
  const s = selected.value
  return (s?.kind === 'stack' && s.data.name === name) || (s?.kind === 'container' && s.fromStack === name)
}
</script>

<template>
  <div class="app">
    <header class="app__topbar">
      <div class="app__brand">
        <img src="/favicon.svg" alt="" class="app__logo" />
        <h1>naswarden</h1>
      </div>
      <div class="conn-group">
        <span class="conn__age">{{ ageLabel }}</span>
        <!-- only the state is announced to screen readers, not the ticking age -->
        <span class="conn" :class="{ 'conn--live': connected && !isStale, 'conn--stale': isStale }" role="status">
          {{ connLabel }}
        </span>
      </div>
    </header>

    <div class="app__layout">
      <nav class="rail" aria-label="Sections">
        <button
          class="rail__item"
          :class="{ active: activeSection === 'all' }"
          :aria-current="activeSection === 'all' ? 'true' : undefined"
          @click="selectSection('all')"
        >
          All
        </button>
        <button
          v-if="server"
          class="rail__item rail__item--server"
          :class="railClass('server')"
          :aria-current="activeSection === 'server' ? 'true' : undefined"
          @click="selectSection('server')"
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="2" width="20" height="8" rx="2"/><rect x="2" y="14" width="20" height="8" rx="2"/><path d="M6 6h.01M6 18h.01"/></svg>
          Server
        </button>
        <button class="rail__item rail__item--pool" :class="railClass('pools')" :aria-current="activeSection === 'pools' ? 'true' : undefined"
          @click="selectSection('pools')">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><ellipse cx="12" cy="5" rx="8" ry="3"/><path d="M4 5v14c0 1.7 3.6 3 8 3s8-1.3 8-3V5"/><path d="M4 12c0 1.7 3.6 3 8 3s8-1.3 8-3"/></svg>
          Pools
        </button>
        <button
          class="rail__item rail__item--dataset"
          :class="railClass('datasets')"
          :aria-current="activeSection === 'datasets' ? 'true' : undefined"
          @click="selectSection('datasets')"
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7z"/></svg>
          Datasets
        </button>
        <button
          v-if="hasStacks"
          class="rail__item rail__item--stack"
          :class="railClass('stacks')"
          :aria-current="activeSection === 'stacks' ? 'true' : undefined"
          @click="selectSection('stacks')"
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="12 2 2 7 12 12 22 7 12 2"/><polyline points="2 17 12 22 22 17"/><polyline points="2 12 12 17 22 12"/></svg>
          Stacks
        </button>
        <button
          v-if="hasContainers"
          class="rail__item rail__item--container"
          :class="railClass('containers')"
          :aria-current="activeSection === 'containers' ? 'true' : undefined"
          @click="selectSection('containers')"
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 8l-9-5-9 5 9 5 9-5z"/><path d="M3 8v8l9 5 9-5V8"/><path d="M12 13v8"/></svg>
          Containers
        </button>
      </nav>

      <main class="app__content">
        <section v-if="server" data-section="server" v-show="activeSection === 'all' || activeSection === 'server'">
          <h2>Server</h2>
          <ServerCard
            :server="server"
            :active="selected?.kind === 'server'"
            @select="selected = { kind: 'server', data: server }"
          />
        </section>

        <section data-section="pools" v-show="activeSection === 'all' || activeSection === 'pools'">
          <h2>Pools</h2>
          <div class="grid">
            <PoolCard
              v-for="pool in pools"
              :key="pool.name"
              :pool="pool"
              :active="selected?.kind === 'pool' && selected.data.name === pool.name"
              @select="selected = { kind: 'pool', data: pool }"
            />
            <p v-if="pools.length === 0" class="empty">Waiting for data…</p>
          </div>
        </section>

        <section data-section="datasets" v-show="activeSection === 'all' || activeSection === 'datasets'">
          <h2>Dataset quotas</h2>
          <div class="grid grid--datasets">
            <DatasetCard
              v-for="dataset in sortedDatasets"
              :key="dataset.name"
              :dataset="dataset"
              :active="selected?.kind === 'dataset' && selected.data.name === dataset.name"
              @select="selected = { kind: 'dataset', data: dataset }"
            />
            <p v-if="datasets.length === 0" class="empty">No datasets with a quota configured.</p>
          </div>
        </section>

        <section v-if="hasStacks" data-section="stacks" v-show="activeSection === 'all' || activeSection === 'stacks'">
          <h2>Stacks</h2>
          <div class="grid">
            <StackCard
              v-for="stack in stacks"
              :key="stack.name"
              :stack="stack"
              :active="stackActive(stack.name)"
              @select="selected = { kind: 'stack', data: stack }"
            />
          </div>
        </section>

        <!-- Stacks is the default landing view, so the flat list lives on its own tab -->
        <section v-if="hasContainers" data-section="containers" v-show="activeSection === 'containers'">
          <h2>Containers</h2>
          <div class="grid grid--datasets">
            <ContainerCard
              v-for="container in sortedContainers"
              :key="container.name"
              :container="container"
              :active="selected?.kind === 'container' && selected.data.name === container.name"
              @select="selected = { kind: 'container', data: container }"
            />
          </div>
        </section>
      </main>
    </div>

    <DetailDrawer
      :open="selected !== null"
      :title="drawerTitle"
      :back="drawerBack"
      @close="selected = null"
      @back="drawerBack && openStack(drawerBack)"
    >
      <template v-if="selected?.kind === 'stack'" #title-icon>
        <AppIcon :candidates="iconCandidates(selected.data)" :size="22" />
      </template>
      <ServerDetails v-if="selected?.kind === 'server'" :server="selected.data" />
      <PoolDetails v-else-if="selected?.kind === 'pool'" :pool="selected.data" />
      <DatasetDetails v-else-if="selected?.kind === 'dataset'" :dataset="selected.data" />
      <StackDetails
        v-else-if="selected?.kind === 'stack'"
        :stack="selected.data"
        @open-container="(name: string) => openContainer(name, selected?.kind === 'stack' ? selected.data.name : undefined)"
      />
      <ContainerDetails
        v-else-if="selected?.kind === 'container'"
        :container="selected.data"
        @open-stack="openStack"
      />
    </DetailDrawer>
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

.conn-group {
  display: flex;
  align-items: baseline;
  gap: 0.75rem;
}

.conn__age {
  font-size: 0.8rem;
  color: var(--text-dim);
  font-variant-numeric: tabular-nums;
}

.conn.conn--stale {
  color: var(--warn-t);
}

.conn {
  font-size: 0.8rem;
  color: var(--text-dim);
}

.conn--live::before {
  content: '●';
  color: var(--ok);
  margin-right: 0.4rem;
}

.conn:not(.conn--live)::before {
  content: '●';
  color: var(--warn);
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
.rail__item--stack.active svg,
.rail__item--stack.scroll-active svg {
  color: var(--stack);
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
    flex: none;
    flex-direction: row;
    position: static;
    width: 100%;
    overflow-x: auto;
    padding-bottom: 0.25rem;
  }
  .rail__item {
    flex: 0 0 auto;
    width: auto;
    white-space: nowrap;
  }
}
</style>
