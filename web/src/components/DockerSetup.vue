<script setup lang="ts">
import type { DockerStatus } from '../composables/usePoolSocket'

// Shown in place of Stacks and Containers when NasWarden has no Docker access,
// so the feature is discoverable from the dashboard itself (a TrueNAS catalog
// install starts without it). With a startup error (e.g. the socket is mounted
// but the group is missing), the reason comes first.
defineProps<{ status: DockerStatus }>()

const GUIDE = 'https://github.com/ahmd-soliman/naswarden#2-docker-stacks-and-containers-optional'
</script>

<template>
  <div class="setup" :class="{ 'setup--error': status.state === 'error' }">
    <p class="setup__title">
      <strong>{{ status.state === 'error' ? 'Docker is not available.' : 'Docker not connected.' }}</strong>
      Stacks and containers appear here once NasWarden can read Docker.
    </p>
    <p v-if="status.error" class="setup__error">{{ status.error }}</p>
    <p>In the TrueNAS app, choose <strong>Edit</strong>:</p>
    <ol>
      <li>User and Group Configuration → <strong>Group</strong>: <code>999</code></li>
      <li>
        Storage → <strong>Additional Storage</strong> → Host Path <code>/var/run/docker.sock</code>, Mount Path
        <code>/var/run/docker.sock</code>, Read Only
      </li>
    </ol>
    <p class="setup__note">
      The socket gives NasWarden full access to Docker, though it only ever reads. For access that is limited to
      reading, use a socket proxy with <code>DOCKER_PROXY_URL</code> instead.
      <a :href="GUIDE" target="_blank" rel="noopener">Setup guide</a>
    </p>
  </div>
</template>

<style scoped>
.setup {
  padding: 0.9rem 1.1rem;
  border: 1px dashed var(--border);
  border-radius: 8px;
  background: var(--card-bg);
  color: var(--text-dim);
  font-size: 0.9rem;
  max-width: 46rem;
}
.setup--error {
  border-style: solid;
  border-color: color-mix(in srgb, var(--warn-t) 45%, transparent);
}
.setup p {
  margin: 0 0 0.5rem;
}
.setup__title strong {
  color: var(--text);
}
.setup__error {
  color: var(--warn-t);
  overflow-wrap: anywhere;
}
.setup ol {
  margin: 0 0 0.6rem;
  padding-left: 1.3rem;
}
.setup li {
  margin-bottom: 0.2rem;
}
.setup code {
  font-size: 0.85em;
  color: var(--text);
}
.setup__note {
  margin-bottom: 0;
  font-size: 0.85rem;
}
.setup a {
  color: var(--stack);
  white-space: nowrap;
}
</style>
