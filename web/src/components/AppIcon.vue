<script setup lang="ts">
defineProps<{ name: string }>();
</script>
<template><img class="app-icon" :src="`/icons/${name}.svg`" alt="" aria-hidden="true" width="20" height="20"></template>
