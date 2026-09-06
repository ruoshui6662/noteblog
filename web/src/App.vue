<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";

type Site = {
  name: string;
  stage: string;
  environment: string;
};

type TreeNode = {
  kind: "category" | "document";
  path: string;
  title: string;
  children?: TreeNode[];
};

type Heading = {
  level: number;
  id: string;
  text: string;
};

type Document = {
  path: string;
  title: string;
  description?: string;
  html: string;
  headings?: Heading[];
};

type SearchResult = {
  path: string;
  title: string;
  description?: string;
  snippet: string;
};

const site = ref<Site | null>(null);
const tree = ref<TreeNode[]>([]);
const currentDocument = ref<Document | null>(null);
const status = ref<"loading" | "ready" | "empty" | "error">("loading");
const errorMessage = ref("");
const searchQuery = ref("");
const searchResults = ref<SearchResult[]>([]);
const searchStatus = ref<"idle" | "loading" | "ready" | "error">("idle");
let searchTimer: ReturnType<typeof setTimeout> | undefined;
let searchRequest = 0;

const flattenedTree = computed(() => {
  const result: Array<TreeNode & { depth: number }> = [];
  const visit = (nodes: TreeNode[], depth: number) => {
    for (const node of nodes) {
      result.push({ ...node, depth });
      if (node.children) visit(node.children, depth + 1);
    }
  };
  visit(tree.value, 0);
  return result;
});

const selectedPath = computed(() => currentDocument.value?.path ?? "");

function documentURL(path: string) {
  return `/docs/${path.split("/").map(encodeURIComponent).join("/")}`;
}

function firstDocument(nodes: TreeNode[]): string | null {
  for (const node of nodes) {
    if (node.kind === "document") return node.path;
    if (node.children) {
      const nested = firstDocument(node.children);
      if (nested) return nested;
    }
  }
  return null;
}

function routeDocumentPath() {
  if (!window.location.pathname.startsWith("/docs/")) return null;
  const value = window.location.pathname.slice("/docs/".length);
  if (!value) return null;
  try {
    return value.split("/").map(decodeURIComponent).join("/");
  } catch {
    return null;
  }
}

async function loadDocument(path: string | null, replace = false) {
  if (!path) {
    currentDocument.value = null;
    status.value = tree.value.length ? "ready" : "empty";
    return;
  }
  status.value = "loading";
  errorMessage.value = "";
  try {
    const response = await fetch(`/api/v1/docs/${path.split("/").map(encodeURIComponent).join("/")}`);
    if (!response.ok) throw new Error("文档加载失败");
    currentDocument.value = (await response.json()) as Document;
    status.value = "ready";
    const target = documentURL(currentDocument.value.path);
    if (replace) window.history.replaceState({}, "", target);
    globalThis.document.title = `${currentDocument.value.title} · ${site.value?.name ?? "Markdown 文档库"}`;
  } catch (error) {
    currentDocument.value = null;
    status.value = "error";
    errorMessage.value = error instanceof Error ? error.message : "文档加载失败";
  }
}

async function selectDocument(path: string) {
	searchQuery.value = "";
	searchResults.value = [];
	searchStatus.value = "idle";
	await loadDocument(path);
  if (status.value === "ready") {
    window.history.pushState({}, "", documentURL(path));
    window.scrollTo({ top: 0, behavior: "smooth" });
  }
}

function queueSearch() {
	if (searchTimer) clearTimeout(searchTimer);
	const query = searchQuery.value.trim();
	if (!query) {
		searchResults.value = [];
		searchStatus.value = "idle";
		return;
	}
	searchTimer = setTimeout(() => void runSearch(query), 180);
}

async function runSearch(query = searchQuery.value.trim()) {
	if (!query) return;
	const request = ++searchRequest;
	searchStatus.value = "loading";
	try {
		const response = await fetch(`/api/v1/search?q=${encodeURIComponent(query)}`);
		if (!response.ok) throw new Error("搜索失败");
		const results = (await response.json()) as SearchResult[];
		if (request !== searchRequest) return;
		searchResults.value = results;
		searchStatus.value = "ready";
	} catch (error) {
		if (request !== searchRequest) return;
		searchStatus.value = "error";
		errorMessage.value = error instanceof Error ? error.message : "搜索失败";
	}
}

async function goHome() {
	searchQuery.value = "";
	searchResults.value = [];
	searchStatus.value = "idle";
	window.history.pushState({}, "", "/");
	await loadApp();
}

async function loadApp() {
  status.value = "loading";
  errorMessage.value = "";
  try {
    const [siteResponse, treeResponse] = await Promise.all([
      fetch("/api/v1/site"),
      fetch("/api/v1/tree"),
    ]);
    if (!siteResponse.ok || !treeResponse.ok) throw new Error("服务请求失败");
    site.value = (await siteResponse.json()) as Site;
    tree.value = (await treeResponse.json()) as TreeNode[];
    const requested = routeDocumentPath();
    const target = requested ?? firstDocument(tree.value);
    await loadDocument(target, requested === null && target !== null);
    if (!target) status.value = "empty";
  } catch (error) {
    status.value = "error";
    errorMessage.value = error instanceof Error ? error.message : "服务请求失败";
  }
}

function handlePopState() {
	if (routeDocumentPath()) {
		void loadDocument(routeDocumentPath());
	} else {
		void loadApp();
	}
}

onMounted(() => {
  window.addEventListener("popstate", handlePopState);
  void loadApp();
});

onBeforeUnmount(() => {
	window.removeEventListener("popstate", handlePopState);
	if (searchTimer) clearTimeout(searchTimer);
});
</script>

<template>
  <div class="app-shell">
    <header class="app-header">
      <div class="app-header__inner">
        <a class="brand" href="/" aria-label="返回文档首页" @click.prevent="goHome">
          <span class="brand__mark" aria-hidden="true">M</span>
          <span>{{ site?.name ?? "Markdown 文档库" }}</span>
        </a>
        <span class="stage-badge">{{ site?.environment ?? "连接中" }} · {{ site?.stage ?? "M1" }}</span>
      </div>
    </header>

    <div class="docs-layout">
      <aside class="sidebar" aria-label="文档导航">
        <p class="sidebar__label">文档导航</p>
        <form class="search-form" role="search" @submit.prevent="runSearch()">
          <label class="sr-only" for="document-search">搜索文档</label>
          <input
            id="document-search"
            v-model="searchQuery"
            type="search"
            placeholder="搜索文档"
            autocomplete="off"
            @input="queueSearch"
          >
        </form>
        <p v-if="searchStatus === 'loading'" class="search-status" role="status">搜索中…</p>
        <p v-else-if="searchStatus === 'error'" class="search-status search-status--error" role="alert">{{ errorMessage }}</p>
        <nav v-else-if="searchQuery.trim()" class="tree" aria-label="搜索结果">
          <a
            v-for="result in searchResults"
            :key="result.path"
            class="tree-item tree-item--document search-result"
            :href="documentURL(result.path)"
            @click.prevent="selectDocument(result.path)"
          >
            <strong>{{ result.title }}</strong>
            <span>{{ result.snippet || result.description || result.path }}</span>
          </a>
          <p v-if="searchStatus === 'ready' && !searchResults.length" class="sidebar__empty">没有匹配文档</p>
        </nav>
        <nav v-else-if="flattenedTree.length" class="tree" aria-label="公开文档">
          <template v-for="node in flattenedTree" :key="`${node.kind}:${node.path}`">
            <p
              v-if="node.kind === 'category'"
              class="tree-item tree-item--category"
              :style="{ paddingLeft: `${node.depth * 0.75 + 0.75}rem` }"
            >
              {{ node.title }}
            </p>
            <a
              v-else
              class="tree-item tree-item--document"
              :class="{ 'tree-item--active': selectedPath === node.path }"
              :style="{ paddingLeft: `${node.depth * 0.75 + 0.75}rem` }"
              :href="documentURL(node.path)"
              :aria-current="selectedPath === node.path ? 'page' : undefined"
              @click.prevent="selectDocument(node.path)"
            >
              {{ node.title }}
            </a>
          </template>
        </nav>
        <p v-else class="sidebar__empty">暂无公开文档</p>
      </aside>

      <main id="main-content" class="article" tabindex="-1">
        <template v-if="status === 'loading'">
          <p class="loading-state" role="status">正在加载文档…</p>
        </template>
        <template v-else-if="status === 'error'">
          <section class="notice notice--error" role="alert">
            <h1>暂时无法加载</h1>
            <p>{{ errorMessage }}</p>
            <button type="button" @click="loadApp">重新连接</button>
          </section>
        </template>
        <template v-else-if="status === 'empty'">
          <p class="breadcrumb">文档库</p>
          <h1>还没有公开文档</h1>
          <p class="article__lead">把 Markdown 文件放入飞牛数据卷的 <code>content</code> 目录后刷新页面。</p>
          <section class="notice" aria-live="polite">
            <h2>阅读端已就绪</h2>
            <p>系统会自动读取文件名、Front Matter 和 Markdown 标题生成文档树。</p>
          </section>
        </template>
        <template v-else-if="currentDocument">
          <p class="breadcrumb">文档库 <span aria-hidden="true">/</span> {{ currentDocument.path }}</p>
          <h1>{{ currentDocument.title }}</h1>
          <p v-if="currentDocument.description" class="article__lead">{{ currentDocument.description }}</p>
          <article class="document-body" v-html="currentDocument.html" />
        </template>
      </main>

      <aside class="toc" aria-label="本页目录">
        <p class="toc__label">本页</p>
        <template v-if="currentDocument?.headings?.length">
          <a
            v-for="heading in currentDocument.headings"
            :key="heading.id"
            :href="`#${heading.id}`"
            :style="{ paddingLeft: `${Math.max(1, heading.level - 1) * 0.75}rem` }"
          >
            {{ heading.text }}
          </a>
        </template>
        <span v-else class="toc__empty">暂无目录</span>
      </aside>
    </div>
  </div>
</template>
