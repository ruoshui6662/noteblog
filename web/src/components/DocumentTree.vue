<script setup lang="ts">
import { ref, watch } from 'vue';
import AppIcon from './AppIcon.vue';
import { documentURL, type TreeNode } from '../types';
const props = defineProps<{ nodes: TreeNode[]; selected: string; depth?: number }>();
defineEmits<{ select: [path: string] }>();
const closed = ref(new Set<string>());
watch(() => props.nodes, nodes => { closed.value = new Set(nodes.filter(n => n.collapsed && !props.selected.startsWith(n.path + '/')).map(n => n.path)); }, { immediate: true });
watch(() => props.selected, path => { for (const node of props.nodes) if (path.startsWith(node.path + '/')) closed.value.delete(node.path); });
</script>
<template>
  <ul class="doc-tree" :class="{ 'doc-tree--nested': depth }">
    <li v-for="node in nodes" :key="node.path">
      <template v-if="node.kind === 'category'">
        <button type="button" class="category-toggle" :aria-expanded="!closed.has(node.path)" @click="closed.has(node.path) ? closed.delete(node.path) : closed.add(node.path)"><span>{{ node.title }}</span><AppIcon name="chevron-down" /></button>
        <DocumentTree v-if="!closed.has(node.path)" :nodes="node.children ?? []" :selected="selected" :depth="(depth ?? 0) + 1" @select="$emit('select', $event)" />
      </template>
      <a v-else :href="documentURL(node.path)" :aria-current="node.path === selected ? 'page' : undefined" @click.prevent="$emit('select', node.path)">{{ node.title }}</a>
    </li>
  </ul>
</template>
