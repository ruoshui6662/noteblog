<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue';
import SiteHeader from './SiteHeader.vue';
import AppIcon from './AppIcon.vue';
import DocumentTree from './DocumentTree.vue';
import SearchDialog from './SearchDialog.vue';
import { documentNodes, documentURL, type Doc, type TreeNode } from '../types';

const siteName = ref('Markdown 文档库');
const tree = ref<TreeNode[]>([]);
const doc = ref<Doc | null>(null);
const home = ref(!window.location.pathname.startsWith('/docs/'));
const state = ref<'loading' | 'ready' | 'error'>('loading');
const error = ref('');
const searchOpen = ref(false);
const allDocuments = ref(false);
const activeHeading = ref('');
const article = ref<HTMLElement>();
const drawer = ref<HTMLDialogElement>();
const drawerOpen = ref(false);
const copyMessage = ref('');
let returnFocus: HTMLElement | null = null;
let oldOverflow = '';
let controller: AbortController | undefined;
let request = 0;
let frame = 0;
let copyTimer: ReturnType<typeof setTimeout> | undefined;
const leaves = computed(() => documentNodes(tree.value));
const selectedIndex = computed(() => leaves.value.findIndex(node => node.path === doc.value?.path));
const previous = computed(() => leaves.value[selectedIndex.value - 1]);
const next = computed(() => selectedIndex.value >= 0 ? leaves.value[selectedIndex.value + 1] : undefined);
const groups = computed(() => {
  const categories = tree.value.filter(node => node.kind === 'category').map(node => ({ ...node, documents: documentNodes(node.children ?? []) }));
  const rootDocs = tree.value.filter(node => node.kind === 'document');
  if (rootDocs.length) categories.unshift({ kind: 'category', path: '', title: '文档', documents: rootDocs });
  return categories;
});
const breadcrumbs = computed(() => {
  const result: string[] = [];
  function visit(nodes: TreeNode[]): boolean {
    for (const node of nodes) {
      if (node.path === doc.value?.path) return true;
      if (node.children && visit(node.children)) { result.unshift(node.title); return true; }
    }
    return false;
  }
  visit(tree.value); return result;
});
const headings = computed(() => (doc.value?.headings ?? []).filter(h => !(h.level === 1 && h.text.trim() === doc.value?.title.trim())));
const articleHTML = computed(() => {
  if (!doc.value) return '';
  const parsed = new DOMParser().parseFromString(doc.value.html, 'text/html');
  const first = parsed.body.firstElementChild;
  if (first?.tagName === 'H1' && first.textContent?.trim() === doc.value.title.trim()) first.remove();
  return parsed.body.innerHTML;
});

function openDrawer() {
  returnFocus = document.activeElement as HTMLElement;
  oldOverflow = document.body.style.overflow;
  document.body.style.overflow = 'hidden';
  drawerOpen.value = true; drawer.value?.showModal();
}
function closeDrawer() {
  if (!drawerOpen.value) return;
  drawer.value?.close(); drawerOpen.value = false;
  document.body.style.overflow = oldOverflow; returnFocus?.focus();
}
function syncHeading() {
  cancelAnimationFrame(frame);
  frame = requestAnimationFrame(() => {
    let current = headings.value[0]?.id ?? '';
    for (const heading of headings.value) {
      const element = document.getElementById(heading.id);
      if (element && element.getBoundingClientRect().top <= 160) current = heading.id;
    }
    activeHeading.value = current;
  });
}
function jumpTo(id: string, replace = false) {
  const element = document.getElementById(id);
  if (!element) return;
  activeHeading.value = id;
  const url = window.location.pathname + '#' + encodeURIComponent(id);
  if (replace) window.history.replaceState({}, '', url);
  else window.history.pushState({}, '', url);
  element.scrollIntoView({ behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth' });
}
async function decorateArticle() {
  await nextTick();
  article.value?.querySelectorAll('pre').forEach(pre => {
    const button = document.createElement('button');
    button.type = 'button'; button.className = 'code-copy'; button.textContent = '复制';
    button.setAttribute('aria-label', '复制代码'); pre.append(button);
  });
  article.value?.querySelectorAll<HTMLAnchorElement>('a[target="_blank"]').forEach(link => link.rel = 'noopener noreferrer');
  if (window.location.hash) {
    try { jumpTo(decodeURIComponent(window.location.hash.slice(1)), true); } catch { /* Invalid anchor leaves article readable. */ }
  }
  syncHeading();
}
async function loadRoute() {
  controller?.abort(); controller = new AbortController(); const sequence = ++request;
  home.value = !window.location.pathname.startsWith('/docs/');
  error.value = ''; doc.value = null; copyMessage.value = ''; activeHeading.value = '';
  if (home.value) { state.value = 'ready'; document.title = siteName.value; return; }
  state.value = 'loading';
  try {
    const path = window.location.pathname.slice(6).split('/').map(decodeURIComponent).join('/');
    const response = await fetch(`/api/v1/docs/${path.split('/').map(encodeURIComponent).join('/')}`, { signal: controller.signal });
    if (!response.ok) throw new Error(response.status === 404 ? '这篇文档不存在，或暂未公开。' : '暂时无法读取文档，请稍后重试。');
    const data = await response.json() as Doc;
    if (sequence !== request) return;
    doc.value = data; state.value = 'ready'; document.title = `${data.title} · ${siteName.value}`;
    await decorateArticle();
  } catch (reason) { if (sequence === request) { state.value = 'error'; error.value = reason instanceof URIError ? '文档地址格式不正确。' : reason instanceof Error ? reason.message : '文档加载失败'; } }
}
async function initialize() {
  state.value = 'loading'; error.value = '';
  try {
    const [siteResponse, treeResponse] = await Promise.all([fetch('/api/v1/site'), fetch('/api/v1/tree')]);
    if (!siteResponse.ok || !treeResponse.ok) throw new Error();
    const site = await siteResponse.json(); siteName.value = site.name || siteName.value;
    tree.value = (await treeResponse.json()) ?? [];
    await loadRoute();
  } catch { state.value = 'error'; error.value = '暂时无法连接文档库，请检查服务后重试。'; }
}
async function navigate(path?: string, hash = '') {
  closeDrawer(); searchOpen.value = false;
  window.history.pushState({}, '', path ? documentURL(path) + hash : '/');
  window.scrollTo({ top: 0, behavior: 'instant' });
  await loadRoute();
  await nextTick(); document.getElementById('main-content')?.focus({ preventScroll: true });
}
async function showDirectory() {
  if (!home.value) await navigate();
  await nextTick();
  requestAnimationFrame(() => {
    const directory = document.getElementById('directory');
    if (!directory) return;
    // Use the page scroll position explicitly so this also works in WebViews where
    // scrollIntoView may target an unexpected ancestor. Leave room for the sticky header.
    const top = Math.max(0, directory.getBoundingClientRect().top + window.scrollY - 96);
    window.scrollTo({ top, behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth' });
  });
}
async function onArticleClick(event: MouseEvent) {
  const target = event.target as HTMLElement;
  const button = target.closest<HTMLButtonElement>('.code-copy');
  if (button) {
    try { await navigator.clipboard.writeText(button.parentElement?.querySelector('code')?.textContent ?? ''); button.textContent = '已复制'; copyMessage.value = '代码已复制到剪贴板'; }
    catch { button.textContent = '复制失败'; copyMessage.value = '无法访问剪贴板，请手动选择代码复制。'; }
    clearTimeout(copyTimer); copyTimer = setTimeout(() => { button.textContent = '复制'; }, 2000); return;
  }
  const link = target.closest<HTMLAnchorElement>('a');
  if (!link || event.ctrlKey || event.metaKey || event.shiftKey || event.altKey || link.target === '_blank' || link.hasAttribute('download')) return;
  const url = new URL(link.href, window.location.href);
  if (url.origin !== window.location.origin) return;
  if (url.pathname === window.location.pathname && url.hash) { event.preventDefault(); try { jumpTo(decodeURIComponent(url.hash.slice(1))); } catch {} }
  else if (url.pathname.startsWith('/docs/')) { event.preventDefault(); try { await navigate(url.pathname.slice(6).split('/').map(decodeURIComponent).join('/'), url.hash); } catch {} }
}
function keyboard(event: KeyboardEvent) {
  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') { event.preventDefault(); closeDrawer(); searchOpen.value = !searchOpen.value; }
}
function historyChanged() {
  closeDrawer(); searchOpen.value = false;
  const currentPath = doc.value ? documentURL(doc.value.path) : '';
  if (currentPath === window.location.pathname && window.location.hash) {
    try { document.getElementById(decodeURIComponent(window.location.hash.slice(1)))?.scrollIntoView(); } catch {}
    syncHeading(); return;
  }
  void loadRoute();
}
onMounted(() => { void initialize(); window.addEventListener('keydown', keyboard); window.addEventListener('popstate', historyChanged); window.addEventListener('scroll', syncHeading, { passive: true }); });
onBeforeUnmount(() => { controller?.abort(); request++; closeDrawer(); cancelAnimationFrame(frame); clearTimeout(copyTimer); window.removeEventListener('keydown', keyboard); window.removeEventListener('popstate', historyChanged); window.removeEventListener('scroll', syncHeading); });
</script>

<template>
  <div class="reader-shell">
    <SiteHeader :name="siteName" :home="home" @home="navigate()" @search="searchOpen = true" @menu="openDrawer" @directory="showDirectory" />
    <main v-if="home" id="main-content" class="home-main" tabindex="-1">
      <section class="knowledge-hero" aria-labelledby="home-heading">
        <img class="knowledge-hero__art" src="/images/knowledge-hero.webp" alt="" width="1536" height="640" fetchpriority="high">
        <div class="knowledge-hero__content">
          <h1 id="home-heading">每一份知识，都有来处。</h1>
          <p>让 Markdown 成为清晰、可查找、可持续积累的文档。</p>
          <button class="hero-search" type="button" @click="searchOpen = true"><AppIcon name="search" /><span>搜索文档、标题或关键词</span><kbd>Ctrl K</kbd></button>
        </div>
      </section>
      <section id="directory" class="home-directory" aria-labelledby="directory-heading" :aria-busy="state === 'loading'">
        <header class="section-heading"><h2 id="directory-heading">探索文档</h2><button v-if="leaves.length" class="text-button" type="button" :aria-pressed="allDocuments" @click="allDocuments = !allDocuments">{{ allDocuments ? '按分类浏览' : '全部文档' }}<AppIcon name="arrow-right" /></button></header>
        <p v-if="state === 'loading'" class="page-state" role="status">正在整理文档目录…</p>
        <div v-else-if="state === 'error'" class="page-state" role="alert"><h3>暂时无法连接</h3><p>{{ error }}</p><button class="primary-button" type="button" @click="initialize">重新连接</button></div>
        <div v-else-if="!leaves.length" class="page-state"><h3>知识库，从第一篇开始。</h3><p>这里将展示已公开的文档。管理员可以添加内容，读者稍后再来看看。</p><a class="primary-button" href="/admin">进入管理后台<AppIcon name="arrow-right" /></a></div>
        <div v-else-if="allDocuments" class="all-documents"><a v-for="item in leaves" :key="item.path" class="directory-link" :href="documentURL(item.path)" @click.prevent="navigate(item.path)"><span>{{ item.title }}</span><AppIcon name="arrow-right" /></a></div>
        <div v-else class="category-grid">
          <section v-for="(group, index) in groups" :key="group.path" class="directory-category">
            <span class="category-number">{{ String(index + 1).padStart(2, '0') }}</span><h3>{{ group.title }}</h3>
            <p class="category-description">{{ group.description || (index === 0 ? '从这里开始，找到清晰的说明与指引。' : '按主题浏览，让需要的知识触手可及。') }}</p>
            <a v-for="item in group.documents" :key="item.path" class="directory-link" :href="documentURL(item.path)" @click.prevent="navigate(item.path)"><span>{{ item.title }}</span><AppIcon name="arrow-right" /></a>
          </section>
        </div>
      </section>
      <footer class="home-footer">{{ siteName }}<span>自托管 · Markdown 文档库</span></footer>
    </main>
    <div v-else class="reading-layout">
      <aside class="reading-sidebar" aria-label="文档导航"><DocumentTree :nodes="tree" :selected="doc?.path ?? ''" @select="navigate" /><p class="sidebar-footer"><AppIcon name="world" />公开文档</p></aside>
      <main id="main-content" class="reading-article" tabindex="-1" :aria-busy="state === 'loading'">
        <div v-if="state === 'loading'" class="page-state" role="status">正在打开文档…</div>
        <div v-else-if="state === 'error'" class="page-state" role="alert"><p class="eyebrow">文档暂不可用</p><h1>还没有找到这一页。</h1><p>{{ error }}</p><button class="primary-button" type="button" @click="initialize">重新尝试</button><button class="text-button" type="button" @click="navigate()">返回首页</button></div>
        <template v-else-if="doc">
          <nav class="reading-breadcrumb" aria-label="面包屑"><a href="/" @click.prevent="navigate()">文档库</a><template v-for="(part, index) in breadcrumbs" :key="index"><span aria-hidden="true">/</span><span>{{ part }}</span></template></nav>
          <h1>{{ doc.title }}</h1><p v-if="doc.description" class="reading-lead">{{ doc.description }}</p>
          <img class="reading-banner" src="/images/reading-banner.webp" alt="" width="1536" height="1024">
          <details v-if="headings.length" class="mobile-toc"><summary>本页内容</summary><nav><a v-for="heading in headings" :key="heading.id" :href="`#${encodeURIComponent(heading.id)}`" @click.prevent="jumpTo(heading.id)">{{ heading.text }}</a></nav></details>
          <article ref="article" class="document-body" v-html="articleHTML" @click="onArticleClick" />
          <nav class="article-pagination" aria-label="相邻文档"><a v-if="previous" :href="documentURL(previous.path)" @click.prevent="navigate(previous.path)"><span><AppIcon name="arrow-left" />上一篇</span><strong>{{ previous.title }}</strong></a><span v-else></span><a v-if="next" :href="documentURL(next.path)" @click.prevent="navigate(next.path)"><span>下一篇<AppIcon name="arrow-right" /></span><strong>{{ next.title }}</strong></a></nav>
        </template>
      </main>
      <aside class="reading-toc" aria-label="本页目录"><p>本页内容</p><nav v-if="headings.length"><a v-for="heading in headings" :key="heading.id" :href="`#${encodeURIComponent(heading.id)}`" :aria-current="activeHeading === heading.id ? 'location' : undefined" :class="{ 'toc-child': heading.level > 2 }" @click.prevent="jumpTo(heading.id)">{{ heading.text }}</a></nav><small v-else>这篇文档没有分节标题</small></aside>
    </div>
    <SearchDialog :open="searchOpen" @close="searchOpen = false" @select="navigate" />
    <dialog ref="drawer" class="nav-drawer" aria-label="文档导航" @cancel.prevent="closeDrawer" @click="$event.target === drawer && closeDrawer()"><div class="nav-drawer__inner"><header><strong>文档目录</strong><button class="icon-button" aria-label="关闭文档导航" type="button" @click="closeDrawer"><AppIcon name="x" /></button></header><DocumentTree :nodes="tree" :selected="doc?.path ?? ''" @select="navigate" /><p v-if="!leaves.length">暂无公开文档</p></div></dialog>
    <p class="sr-only" role="status">{{ copyMessage }}</p>
  </div>
</template>
