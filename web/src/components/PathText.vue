<script setup lang="ts">
defineProps<{ text: string }>()
</script>

<!-- Renders a ZFS/file path with a line-break opportunity after each "/"
     (browsers do not break there on their own, so long dataset paths split
     mid-word). <wbr> adds no character, so copy/paste still yields the
     exact path. -->
<template>
  <template v-for="(part, i) in text.split('/')" :key="i"
    ><template v-if="i > 0">/<wbr /></template>{{ part }}</template
  >
</template>
