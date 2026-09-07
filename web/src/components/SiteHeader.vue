<script setup lang="ts">
import AppIcon from './AppIcon.vue';
withDefaults(defineProps<{ name?: string; reader?: boolean; home?: boolean }>(), { name: 'Markdown 文档库', reader: true, home: false });
defineEmits<{ search: []; menu: []; home: []; directory: [] }>();
</script>
<template>
  <a class="skip-link" href="#main-content">跳至主要内容</a>
  <header class="site-header">
    <a class="wordmark" href="/" @click="reader && ($event.preventDefault(), $emit('home'))">{{ name }}<span>/ MARKDOWN</span></a>
    <nav class="header-nav" aria-label="主导航">
      <a v-if="home" href="#directory" @click.prevent="$emit('directory')">文档目录</a>
      <button v-else-if="reader" class="header-search" type="button" @click="$emit('search')"><AppIcon name="search" /><span>搜索文档…</span><kbd>Ctrl K</kbd></button>
      <a :href="reader ? '/admin' : '/'">{{ reader ? '管理后台' : '返回文档' }}</a>
      <button v-if="reader && !home" class="icon-button mobile-menu" aria-label="打开文档导航" type="button" @click="$emit('menu')"><AppIcon name="menu-2" /></button>
    </nav>
  </header>
</template>
