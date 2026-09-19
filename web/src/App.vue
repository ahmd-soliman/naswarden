<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import ServerCard from './components/ServerCard.vue'
import PoolCard from './components/PoolCard.vue'
import DatasetCard from './components/DatasetCard.vue'
import VMCard from './components/VMCard.vue'
import ContainerCard from './components/ContainerCard.vue'
import StackCard from './components/StackCard.vue'
import AppIcon from './components/AppIcon.vue'
import SearchBox from './components/SearchBox.vue'
import DetailDrawer from './components/DetailDrawer.vue'
import ServerDetails from './components/ServerDetails.vue'
import PoolDetails from './components/PoolDetails.vue'
import DatasetDetails from './components/DatasetDetails.vue'
import VMDetails from './components/VMDetails.vue'
import ContainerDetails from './components/ContainerDetails.vue'
import StackDetails from './components/StackDetails.vue'
import { usePoolSocket } from './composables/usePoolSocket'
import type { Container, Dataset, Pool, ServerInfo, VM } from './composables/usePoolSocket'
import { buildStacks, containerIconCandidates, iconCandidates, isCleanStop, isStopped } from './composables/useStacks'
import type { Stack } from './composables/useStacks'
import { vmIconCandidates } from './composables/useVMs'
import { matchesContainer, matchesDataset, matchesPool, matchesStack, matchesVM, normalizeQuery } from './composables/search'

const { server, pools, datasets, containers, vms, connected, updatedAt, staleSources } = usePoolSocket()

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
// A source that failed its last refresh still shows its previous data (so
// cards don't vanish) -- say so instead of calling everything "live".
const isPartial = computed(() => connected.value && !isStale.value && staleSources.value.length > 0)
const connLabel = computed(() =>
  !connected.value ? 'reconnecting…' : isStale.value ? 'stale' : isPartial.value ? 'partial' : 'live',
)
const ageLabel = computed(() => {
  const s = ageSeconds.value
  if (s === null) return ''
  const age = s < 60 ? `${s}s` : s < 3600 ? `${Math.floor(s / 60)}m` : `${Math.floor(s / 3600)}h`
  const note = staleSources.value.length ? ` · ${staleSources.value.join(', ')} not updating` : ''
  return `updated ${age} ago${note}`
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
const hasVMs = computed(() => vms.value.length > 0)

// 'all' shows every section at once (the default, glanceable overview);
// picking a rail item filters down to just that section. Filtering never
// hides data the "All" view wouldn't already show -- it's a convenience,
// not a different data set.
const activeSection = ref<'all' | 'server' | 'pools' | 'datasets' | 'vms' | 'stacks' | 'containers'>('all')

function selectSection(name: typeof activeSection.value) {
  activeSection.value = name
  scrollSection.value = name === 'all' ? scrollSection.value : name
}

// ---- search ------------------------------------------------------------
// One box filters every list at once by name (stacks also by member name,
// containers also by stack and image, VMs also by type/OS/IP). It applies
// within whichever sections the rail currently shows, and hides a section
// that has no match.
const query = ref('')
const q = computed(() => normalizeQuery(query.value))
const filteredPools = computed(() => pools.value.filter((p) => matchesPool(p, q.value)))
const filteredDatasets = computed(() => sortedDatasets.value.filter((d) => matchesDataset(d, q.value)))
const filteredVMs = computed(() => vms.value.filter((v) => matchesVM(v, q.value)))
const filteredStacks = computed(() => stacks.value.filter((s) => matchesStack(s, q.value)))
const filteredContainers = computed(() => sortedContainers.value.filter((c) => matchesContainer(c, q.value)))

type Searchable = 'pools' | 'datasets' | 'vms' | 'stacks' | 'containers'
const matchCount = computed<Record<Searchable, number>>(() => ({
  pools: filteredPools.value.length,
  datasets: filteredDatasets.value.length,
  vms: filteredVMs.value.length,
  stacks: filteredStacks.value.length,
  containers: filteredContainers.value.length,
}))

// Would this section be shown if the query matched something in it? Containers
// is normally tab-only (Stacks is the landing view) but is revealed while
// searching so a container is findable without knowing its stack.
function inScope(name: Searchable) {
  if (name === 'containers') return activeSection.value === 'containers' || (activeSection.value === 'all' && !!q.value)
  return activeSection.value === 'all' || activeSection.value === name
}
function sectionVisible(name: Searchable) {
  return inScope(name) && (!q.value || matchCount.value[name] > 0)
}
const resultCount = computed(() =>
  (['pools', 'datasets', 'vms', 'stacks', 'containers'] as Searchable[])
    .filter(inScope)
    .reduce((sum, name) => sum + matchCount.value[name], 0),
)

// Screen-reader announcement, debounced so it does not chatter per keystroke.
const announcement = ref('')
let announceTimer: ReturnType<typeof setTimeout> | undefined
watch([q, resultCount], () => {
  clearTimeout(announceTimer)
  announceTimer = setTimeout(() => {
    announcement.value = q.value ? `${resultCount.value} ${resultCount.value === 1 ? 'result' : 'results'}` : ''
  }, 300)
})
onBeforeUnmount(() => clearTimeout(announceTimer))

// While in "All" mode, track which section is currently scrolled into
// view and highlight the matching rail item -- without hiding any other
// section. Paused while a specific section is filtered in, since every
// other section is already hidden then.
const scrollSection = ref<'server' | 'pools' | 'datasets' | 'vms' | 'stacks' | 'containers'>('server')
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
// Containers/VMs sections only exist in the DOM once telemetry arrives --
// re-observe when that changes so the rail highlight tracks them too.
watch([hasContainers, hasStacks, hasVMs], () => nextTick(initObserver))
onBeforeUnmount(() => observer?.disconnect())

const poolAlerts = computed(() => {
  const crit = pools.value.filter((p) => !p.healthy).length
  const warn = pools.value.filter((p) => p.healthy && p.warning).length
  return { count: crit + warn, level: crit > 0 ? 'crit' : 'warn' }
})

const datasetAlerts = computed(() => {
  const crit = datasets.value.filter((d) => d.quota > 0 && d.used / d.quota >= 0.9).length
  const warn = datasets.value.filter((d) => d.quota > 0 && d.used / d.quota >= 0.7 && d.used / d.quota < 0.9).length
  return { count: crit + warn, level: crit > 0 ? 'crit' : 'warn' }
})

const vmAlerts = computed(() => {
  const crit = vms.value.filter((v) => {
    const s = v.status.toLowerCase()
    return s !== 'running' && s !== 'stopped'
  }).length
  return { count: crit, level: 'crit' }
})

const stackAlerts = computed(() => {
  const red = stacks.value.filter((s) => s.health === 'red').length
  const yellow = stacks.value.filter((s) => s.health === 'yellow').length
  return { count: red + yellow, level: red > 0 ? 'crit' : 'warn' }
})

const containerAlerts = computed(() => {
  const crit = containers.value.filter((c) => !isCleanStop(c) && c.state !== 'running').length
  return { count: crit, level: 'crit' }
})

function railClass(section: 'server' | 'pools' | 'datasets' | 'vms' | 'stacks' | 'containers') {
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
  | { kind: 'vm'; data: VM }
  | { kind: 'stack'; data: Stack }
  // fromStack: opened by drilling down from that stack's drawer, so the
  // drawer offers a way back to it
  | { kind: 'container'; data: Container; fromStack?: string }

type SelectionTarget =
  | { kind: 'server' }
  | { kind: 'pool'; name: string }
  | { kind: 'dataset'; name: string }
  | { kind: 'vm'; name: string }
  | { kind: 'stack'; name: string }
  | { kind: 'container'; name: string; fromStack?: string }

const selectedTarget = ref<SelectionTarget | null>(null)

type SelectedData = Selected['data']

const targetKey = (t: SelectionTarget) => (t.kind === 'server' ? 'server' : `${t.kind}:${t.name}`)

function findLive(t: SelectionTarget): SelectedData | undefined {
  switch (t.kind) {
    case 'server':
      return server.value ?? undefined
    case 'pool':
      return pools.value.find((p) => p.name === t.name)
    case 'dataset':
      return datasets.value.find((d) => d.name === t.name)
    case 'vm':
      return vms.value.find((v) => v.name === t.name)
    case 'stack':
      return stacks.value.find((s) => s.name === t.name)
    case 'container':
      return containers.value.find((c) => c.name === t.name)
  }
}

// The last live object of the open target, so that if it is briefly missing
// from one refresh the drawer does not flicker or unmount. Kept by a
// watcher rather than inside the computed below, which must stay pure.
let lastLive: { key: string; data: SelectedData } | null = null
watch(
  [selectedTarget, server, pools, datasets, vms, stacks, containers],
  () => {
    const t = selectedTarget.value
    if (!t) {
      lastLive = null
      return
    }
    const live = findLive(t)
    if (live) lastLive = { key: targetKey(t), data: live }
    else if (lastLive?.key !== targetKey(t)) lastLive = null
  },
  { flush: 'sync' },
)

// Live-updating selected view: resolves against the latest reactive
// websocket telemetry so the open drawer updates in real time.
const selected = computed<Selected | null>(() => {
  const t = selectedTarget.value
  if (!t) return null
  const data = findLive(t) ?? (lastLive?.key === targetKey(t) ? lastLive.data : undefined)
  if (!data) return null
  return { kind: t.kind, data, fromStack: t.kind === 'container' ? t.fromStack : undefined } as Selected
})

const drawerTitle = computed(() => {
  if (!selected.value) return ''
  return selected.value.kind === 'server' ? selected.value.data.hostname : selected.value.data.name
})
const drawerBack = computed(() =>
  selectedTarget.value?.kind === 'container' ? selectedTarget.value.fromStack : undefined,
)

function openStack(name: string) {
  selectedTarget.value = { kind: 'stack', name }
}
function openContainer(name: string, fromStack?: string) {
  selectedTarget.value = { kind: 'container', name, fromStack }
}
// The stack card stays highlighted while one of its containers is open in the drawer.
function stackActive(name: string) {
  const t = selectedTarget.value
  return (t?.kind === 'stack' && t.name === name) || (t?.kind === 'container' && t.fromStack === name)
}
</script>

<template>
  <div class="app">
    <header class="app__topbar">
      <div class="app__brand">
        <img src="/favicon.svg" alt="" class="app__logo" />
        <h1>NasWarden</h1>
      </div>
      <SearchBox v-model="query" class="app__search" />
      <div class="conn-group">
        <span class="conn__age">{{ ageLabel }}</span>
        <!-- only the state is announced to screen readers, not the ticking age -->
        <span class="conn" :class="{ 'conn--live': connected && !isStale && !isPartial, 'conn--stale': isStale || isPartial }" role="status">
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
        <button
class="rail__item rail__item--pool" :class="railClass('pools')" :aria-current="activeSection === 'pools' ? 'true' : undefined"
          @click="selectSection('pools')">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><ellipse cx="12" cy="5" rx="8" ry="3"/><path d="M4 5v14c0 1.7 3.6 3 8 3s8-1.3 8-3V5"/><path d="M4 12c0 1.7 3.6 3 8 3s8-1.3 8-3"/></svg>
          Pools
          <span v-if="poolAlerts.count > 0" class="rail__badge" :class="`rail__badge--${poolAlerts.level}`" :title="`${poolAlerts.count} pool attention needed`">
            {{ poolAlerts.count }}
          </span>
        </button>
        <button
          class="rail__item rail__item--dataset"
          :class="railClass('datasets')"
          :aria-current="activeSection === 'datasets' ? 'true' : undefined"
          @click="selectSection('datasets')"
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7z"/></svg>
          Datasets
          <span v-if="datasetAlerts.count > 0" class="rail__badge" :class="`rail__badge--${datasetAlerts.level}`" :title="`${datasetAlerts.count} dataset quota warnings`">
            {{ datasetAlerts.count }}
          </span>
        </button>
        <button
          v-if="hasVMs"
          class="rail__item rail__item--vm"
          :class="railClass('vms')"
          :aria-current="activeSection === 'vms' ? 'true' : undefined"
          @click="selectSection('vms')"
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="3" width="20" height="14" rx="2"/><path d="M8 21h8"/><path d="M12 17v4"/></svg>
          VMs
          <span v-if="vmAlerts.count > 0" class="rail__badge" :class="`rail__badge--${vmAlerts.level}`" :title="`${vmAlerts.count} VMs in error or non-standard state`">
            {{ vmAlerts.count }}
          </span>
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
          <span v-if="stackAlerts.count > 0" class="rail__badge" :class="`rail__badge--${stackAlerts.level}`" :title="`${stackAlerts.count} stacks with issues`">
            {{ stackAlerts.count }}
          </span>
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
          <span v-if="containerAlerts.count > 0" class="rail__badge" :class="`rail__badge--${containerAlerts.level}`" :title="`${containerAlerts.count} crashed/failed containers`">
            {{ containerAlerts.count }}
          </span>
        </button>
      </nav>

      <main class="app__content">
        <p v-if="q && resultCount === 0" class="empty">No matches for “{{ query.trim() }}”.</p>
        <p class="sr-only" role="status" aria-live="polite">{{ announcement }}</p>

        <section v-if="server" v-show="activeSection === 'all' || activeSection === 'server'" data-section="server">
          <h2>Server</h2>
          <ServerCard
            :server="server"
            :active="selectedTarget?.kind === 'server'"
            @select="selectedTarget = { kind: 'server' }"
          />
        </section>

        <section v-show="sectionVisible('pools')" data-section="pools">
          <h2>Pools</h2>
          <div class="grid">
            <PoolCard
              v-for="pool in filteredPools"
              :key="pool.name"
              :pool="pool"
              :active="selectedTarget?.kind === 'pool' && selectedTarget.name === pool.name"
              @select="selectedTarget = { kind: 'pool', name: pool.name }"
            />
            <p v-if="pools.length === 0" class="empty">Waiting for data…</p>
          </div>
        </section>

        <section v-show="sectionVisible('datasets')" data-section="datasets">
          <h2>Dataset quotas</h2>
          <div class="grid grid--datasets">
            <DatasetCard
              v-for="dataset in filteredDatasets"
              :key="dataset.name"
              :dataset="dataset"
              :active="selectedTarget?.kind === 'dataset' && selectedTarget.name === dataset.name"
              @select="selectedTarget = { kind: 'dataset', name: dataset.name }"
            />
            <p v-if="datasets.length === 0" class="empty">No datasets with a quota configured.</p>
          </div>
        </section>

        <section v-if="hasVMs" v-show="sectionVisible('vms')" data-section="vms">
          <h2>Virtual Machines & Containers</h2>
          <div class="grid">
            <VMCard
              v-for="vm in filteredVMs"
              :key="vm.name"
              :vm="vm"
              :active="selectedTarget?.kind === 'vm' && selectedTarget.name === vm.name"
              @select="selectedTarget = { kind: 'vm', name: vm.name }"
            />
          </div>
        </section>

        <section v-if="hasStacks" v-show="sectionVisible('stacks')" data-section="stacks">
          <h2>Stacks</h2>
          <div class="grid">
            <StackCard
              v-for="stack in filteredStacks"
              :key="stack.name"
              :stack="stack"
              :active="stackActive(stack.name)"
              @select="openStack(stack.name)"
            />
          </div>
        </section>

        <!-- Stacks is the default landing view, so the flat list lives on its own tab -->
        <section v-if="hasContainers" v-show="sectionVisible('containers')" data-section="containers">
          <h2>Containers</h2>
          <div class="grid grid--datasets">
            <ContainerCard
              v-for="container in filteredContainers"
              :key="container.name"
              :container="container"
              :active="selectedTarget?.kind === 'container' && selectedTarget.name === container.name"
              @select="openContainer(container.name)"
            />
          </div>
        </section>
      </main>
    </div>

    <DetailDrawer
      :open="selected !== null"
      :title="drawerTitle"
      :back="drawerBack"
      @close="selectedTarget = null"
      @back="drawerBack && openStack(drawerBack)"
    >
      <template v-if="selected?.kind === 'stack'" #title-icon>
        <AppIcon :candidates="iconCandidates(selected.data)" :size="22" />
      </template>
      <template v-else-if="selected?.kind === 'vm'" #title-icon>
        <AppIcon :candidates="vmIconCandidates(selected.data)" :size="22" />
      </template>
      <template v-else-if="selected?.kind === 'container'" #title-icon>
        <AppIcon :candidates="containerIconCandidates(selected.data)" :size="22" />
      </template>
      <ServerDetails v-if="selected?.kind === 'server'" :server="selected.data" />
      <PoolDetails v-else-if="selected?.kind === 'pool'" :pool="selected.data" />
      <DatasetDetails v-else-if="selected?.kind === 'dataset'" :dataset="selected.data" />
      <VMDetails v-else-if="selected?.kind === 'vm'" :vm="selected.data" />
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

.app__search {
  margin: 0 1rem;
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

.rail__badge {
  margin-left: auto;
  font-size: 0.7rem;
  font-weight: 700;
  line-height: 1;
  padding: 0.15rem 0.45rem;
  border-radius: 999px;
  font-variant-numeric: tabular-nums;
  flex-shrink: 0;
}

.rail__badge--warn {
  background: var(--warn);
  color: #111;
}

.rail__badge--crit {
  background: var(--crit);
  color: #fff;
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
.rail__item--vm.active svg,
.rail__item--vm.scroll-active svg {
  color: var(--vm);
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
  .app__topbar {
    flex-wrap: wrap;
    row-gap: 0.75rem;
  }
  .app__search {
    order: 3;
    flex-basis: 100%;
    max-width: none;
    margin: 0;
  }
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
