<script setup lang="ts">
import AdminView from "./components/AdminView.vue";
import ReaderView from "./components/ReaderView.vue";
const isAdmin = window.location.pathname === "/admin" || window.location.pathname.startsWith("/admin/");
</script>

<template>
  <AdminView v-if="isAdmin" />
  <ReaderView v-else />
</template>
