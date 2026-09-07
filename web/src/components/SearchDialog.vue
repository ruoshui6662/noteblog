<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue';
import AppIcon from './AppIcon.vue';
import { documentURL, type SearchResult } from '../types';
const props = defineProps<{ open: boolean }>();
const emit = defineEmits<{ close: []; select: [path: string] }>();
const dialog = ref<HTMLDialogElement>();
const input = ref<HTMLInputElement>();
const query = ref('');
const results = ref<SearchResult[]>([]);
const state = ref<'idle' | 'loading' | 'ready' | 'error'>('idle');
let timer: ReturnType<typeof setTimeout>;
let controller: AbortController | undefined;
let request = 0;
let returnFocus: HTMLElement | null = null;
let oldOverflow = '';
function cancelSearch() { clearTimeout(timer); controller?.abort(); request++; }
async function search() {
  cancelSearch();
  const text = query.value.trim();
  results.value = [];
  if (!text) { state.value = 'idle'; return; }
  const sequence = request;
  controller = new AbortController();
  state.value = 'loading';
  try {
    const response = await fetch(`/api/v1/search?q=${encodeURIComponent(text)}`, { signal: controller.signal });
    if (!response.ok) throw new Error();
    const data = await response.json();
    if (sequence !== request) return;
    results.value = data ?? []; state.value = 'ready';
  } catch { if (sequence === request) state.value = 'error'; }
}
watch(query, () => { cancelSearch(); results.value = []; state.value = query.value.trim() ? 'loading' : 'idle'; timer = setTimeout(search, 200); });
watch(() => props.open, async open => {
  if (open) {
    returnFocus = document.activeElement as HTMLElement;
    oldOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    query.value = ''; results.value = []; state.value = 'idle';
    dialog.value?.showModal(); await nextTick(); input.value?.focus();
  } else {
    cancelSearch(); dialog.value?.close(); document.body.style.overflow = oldOverflow;
    returnFocus?.focus();
  }
});
onBeforeUnmount(() => { cancelSearch(); if (props.open) document.body.style.overflow = oldOverflow; });
</script>
<template>
  <dialog ref="dialog" class="search-dialog" aria-label="搜索文档" @cancel.prevent="emit('close')" @click="$event.target === dialog && emit('close')">
    <div class="search-dialog__inner">
      <form class="search-dialog__form" role="search" @submit.prevent="search">
        <AppIcon name="search" /><label class="sr-only" for="global-search">搜索文档、标题或关键词</label>
        <input id="global-search" ref="input" v-model="query" type="search" placeholder="搜索文档、标题或关键词" autocomplete="off">
        <button class="icon-button" type="button" aria-label="关闭搜索" @click="emit('close')"><AppIcon name="x" /></button>
      </form>
      <div class="search-dialog__results" :aria-busy="state === 'loading'">
        <p v-if="state === 'idle'" class="search-hint">输入关键词，查找标题与正文中的内容。</p>
        <p v-else-if="state === 'loading'" class="search-hint" role="status">正在查找…</p>
        <div v-else-if="state === 'error'" class="search-hint" role="alert">暂时无法搜索。<button class="text-button" type="button" @click="search">重新尝试</button></div>
        <p v-else-if="!results.length" class="search-hint" role="status">没有找到「{{ query }}」，试试更简短的关键词。</p>
        <template v-else>
          <p class="result-count" role="status">找到 {{ results.length }} 篇文档</p>
          <a v-for="result in results" :key="result.path" class="search-result-link" :href="documentURL(result.path)" @click.prevent="emit('select', result.path); emit('close')"><AppIcon name="file-text" /><span><strong>{{ result.title }}</strong><small>{{ result.snippet || result.description || result.path }}</small></span><AppIcon name="arrow-right" /></a>
        </template>
      </div>
      <footer class="search-dialog__footer">搜索当前可访问的文档<span><kbd>Esc</kbd> 关闭</span></footer>
    </div>
  </dialog>
</template>
