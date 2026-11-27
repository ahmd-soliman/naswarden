<script setup lang="ts">
import { computed } from 'vue'
import type { Pool, Service, Share } from '../composables/usePoolSocket'
import { serviceStateText, serviceTone, shareIssue } from '../composables/services'

const props = defineProps<{ service: Service; shares: Share[]; pools: Pool[] }>()

const tone = computed(() => serviceTone(props.service))
const badgeClass = computed(() => ({ ok: 'badge--green', crit: 'badge--red', gray: 'badge--gray' })[tone.value])
// Only SMB and NFS have shares to list.
const hasShares = computed(() => props.service.name === 'cifs' || props.service.name === 'nfs')
</script>

<template>
  <div class="drawer__section">
    <h3>Service</h3>
    <div class="drawer__kv">
      <span>State</span>
      <span class="badge" :class="badgeClass">{{ serviceStateText(service) }}</span>
    </div>
    <div class="drawer__kv"><span>Start on boot</span><span>{{ service.enable ? 'Yes' : 'No' }}</span></div>
  </div>

  <div v-if="hasShares" class="drawer__section">
    <h3>Shares</h3>
    <p v-if="shares.length === 0" class="drawer__empty">No shares are configured for this service.</p>
    <ul v-else class="shares">
      <li v-for="(sh, i) in shares" :key="`${sh.name}:${sh.path}:${i}`" class="share">
        <div class="share__head">
          <strong v-if="sh.name">{{ sh.name }}</strong>
          <span v-if="sh.read_only" class="drawer__ro">read-only</span>
          <span v-if="!sh.enabled" class="share__off">disabled</span>
        </div>
        <code class="share__path">{{ sh.path || '(no path)' }}</code>
        <p v-if="shareIssue(sh, pools)" class="share__issue">⚠ {{ shareIssue(sh, pools) }}</p>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.shares {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
}
.share {
  padding: 0.5rem 0.65rem;
  border: 1px solid var(--border);
  border-radius: 6px;
}
.share__head {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  flex-wrap: wrap;
}
.share__off {
  color: var(--text-dim);
  font-size: 0.75rem;
  text-transform: uppercase;
}
.share__path {
  display: block;
  margin-top: 0.15rem;
  color: var(--text-dim);
  font-size: 0.8rem;
  overflow-wrap: anywhere;
}
.share__issue {
  margin: 0.3rem 0 0;
  color: var(--crit-t);
  font-size: 0.8rem;
}
</style>
