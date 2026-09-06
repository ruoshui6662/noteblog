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

type AdminUser = {
  username: string;
  created_at: string;
};

type AdminDocumentSummary = {
  path: string;
  title: string;
  description?: string;
  hash: string;
  draft: boolean;
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
const isAdminRoute = computed(() => window.location.pathname === "/admin" || window.location.pathname.startsWith("/admin/"));
const adminMode = ref<"checking" | "setup" | "login" | "authenticated">("checking");
const adminUser = ref<AdminUser | null>(null);
const adminUsername = ref("");
const adminPassword = ref("");
const adminPasswordConfirmation = ref("");
const adminBusy = ref(false);
const adminError = ref("");
const adminMessage = ref("");
const adminDocuments = ref<AdminDocumentSummary[]>([]);
const adminSelectedPath = ref("");
const adminEditingPath = ref("");
const adminDocumentContent = ref("");
const adminDocumentHash = ref("");
const adminDocumentBusy = ref(false);
const adminDocumentError = ref("");

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

async function loadAdmin() {
  adminMode.value = "checking";
  adminError.value = "";
  try {
    const response = await fetch("/api/v1/auth/me");
    if (response.ok) {
      const payload = (await response.json()) as { user: AdminUser };
      adminUser.value = payload.user;
      adminMode.value = "authenticated";
      await loadAdminDocuments();
      return;
    }
    adminMode.value = "setup";
  } catch {
    adminMode.value = "login";
    adminError.value = "无法连接认证服务，请确认容器已经更新到最新镜像。";
  }
}

async function submitAdminAuth(mode: "setup" | "login") {
  adminError.value = "";
  adminMessage.value = "";
  if (!adminUsername.value.trim() || !adminPassword.value) {
    adminError.value = "请输入用户名和密码。";
    return;
  }
  if (mode === "setup" && adminPassword.value !== adminPasswordConfirmation.value) {
    adminError.value = "两次输入的密码不一致。";
    return;
  }
  adminBusy.value = true;
  try {
    const response = await fetch(`/api/v1/auth/${mode}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ username: adminUsername.value.trim(), password: adminPassword.value }),
    });
    const payload = (await response.json().catch(() => ({}))) as { user?: AdminUser; error?: { message?: string } };
    if (!response.ok) {
      if (mode === "setup" && response.status === 409) {
        adminMode.value = "login";
        adminMessage.value = "管理员已经初始化，请直接登录。";
      } else {
        adminError.value = payload.error?.message ?? "操作失败，请稍后重试。";
      }
      return;
    }
    if (mode === "setup") {
      adminMode.value = "login";
      adminPassword.value = "";
      adminPasswordConfirmation.value = "";
      adminMessage.value = "初始化成功，请使用刚设置的密码登录。";
    } else if (payload.user) {
      adminUser.value = payload.user;
      adminMode.value = "authenticated";
      adminPassword.value = "";
      await loadAdminDocuments();
    }
  } catch {
    adminError.value = "无法连接认证服务，请确认容器已经更新到最新镜像。";
  } finally {
    adminBusy.value = false;
  }
}

async function loadAdminDocuments(preferredPath = "") {
  adminDocumentError.value = "";
  const response = await fetch("/api/v1/admin/docs");
  if (response.status === 401) {
    adminMode.value = "login";
    adminUser.value = null;
    return;
  }
  if (!response.ok) throw new Error("无法读取文档列表");
  adminDocuments.value = (await response.json()) as AdminDocumentSummary[];
  const nextPath = preferredPath || adminSelectedPath.value || adminDocuments.value[0]?.path || "";
  if (nextPath) await selectAdminDocument(nextPath);
  else startNewAdminDocument();
}

async function selectAdminDocument(path: string) {
  adminDocumentError.value = "";
  adminDocumentBusy.value = true;
  try {
    const response = await fetch(`/api/v1/admin/docs/${path.split("/").map(encodeURIComponent).join("/")}`);
    if (!response.ok) throw new Error("无法读取文档内容");
    const payload = (await response.json()) as { path: string; content: string; hash: string };
    adminSelectedPath.value = payload.path;
    adminEditingPath.value = payload.path;
    adminDocumentContent.value = payload.content;
    adminDocumentHash.value = payload.hash;
  } catch (error) {
    adminDocumentError.value = error instanceof Error ? error.message : "无法读取文档内容";
  } finally {
    adminDocumentBusy.value = false;
  }
}

function startNewAdminDocument() {
  adminSelectedPath.value = "";
  adminEditingPath.value = "new-document.md";
  adminDocumentContent.value = "---\ntitle: 新文档\n---\n\n开始写作。\n";
  adminDocumentHash.value = "";
  adminDocumentError.value = "";
}

async function saveAdminDocument() {
  const path = adminEditingPath.value.trim();
  if (!path) {
    adminDocumentError.value = "请输入 Markdown 文件路径。";
    return;
  }
  adminDocumentBusy.value = true;
  adminDocumentError.value = "";
  try {
    const isNew = !adminSelectedPath.value;
    const url = isNew ? "/api/v1/admin/docs" : `/api/v1/admin/docs/${adminSelectedPath.value.split("/").map(encodeURIComponent).join("/")}`;
    const headers: Record<string, string> = { "Content-Type": "application/json" };
    if (!isNew) headers["If-Match"] = `"${adminDocumentHash.value}"`;
    const response = await fetch(url, {
      method: isNew ? "POST" : "PUT",
      headers,
      body: JSON.stringify({ path: isNew ? path : undefined, content: adminDocumentContent.value }),
    });
    const payload = (await response.json().catch(() => ({}))) as { path?: string; hash?: string; error?: { message?: string } };
    if (!response.ok) {
      adminDocumentError.value = payload.error?.message ?? "保存失败，请刷新后重试。";
      return;
    }
    adminMessage.value = "文档已保存。";
    adminSelectedPath.value = payload.path ?? path;
    adminEditingPath.value = payload.path ?? path;
    adminDocumentHash.value = payload.hash ?? response.headers.get("ETag")?.replace(/^W\//, "").replaceAll('"', "") ?? "";
    await loadAdminDocuments(adminSelectedPath.value);
  } catch {
    adminDocumentError.value = "无法连接文档服务。";
  } finally {
    adminDocumentBusy.value = false;
  }
}

async function deleteAdminDocument() {
  if (!adminSelectedPath.value || !window.confirm(`确定删除「${adminSelectedPath.value}」吗？`)) return;
  adminDocumentBusy.value = true;
  adminDocumentError.value = "";
  try {
    const response = await fetch(`/api/v1/admin/docs/${adminSelectedPath.value.split("/").map(encodeURIComponent).join("/")}`, {
      method: "DELETE",
      headers: { "If-Match": `"${adminDocumentHash.value}"` },
    });
    const payload = (await response.json().catch(() => ({}))) as { error?: { message?: string } };
    if (!response.ok) {
      adminDocumentError.value = payload.error?.message ?? "删除失败，请刷新后重试。";
      return;
    }
    adminMessage.value = "文档已删除。";
    adminSelectedPath.value = "";
    await loadAdminDocuments();
  } catch {
    adminDocumentError.value = "无法连接文档服务。";
  } finally {
    adminDocumentBusy.value = false;
  }
}

async function adminLogout() {
  adminBusy.value = true;
  try {
    await fetch("/api/v1/auth/logout", { method: "POST" });
  } finally {
    adminUser.value = null;
    adminMode.value = "login";
    adminBusy.value = false;
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
  if (isAdminRoute.value) void loadAdmin();
  else void loadApp();
});

onBeforeUnmount(() => {
	window.removeEventListener("popstate", handlePopState);
	if (searchTimer) clearTimeout(searchTimer);
});
</script>

<template>
  <div v-if="isAdminRoute" class="admin-shell">
    <header class="app-header">
      <div class="app-header__inner">
        <a class="brand" href="/" aria-label="返回文档首页">
          <span class="brand__mark" aria-hidden="true">M</span>
          <span>{{ site?.name ?? "Markdown 文档库" }}</span>
        </a>
        <a class="admin-back-link" href="/">返回文档</a>
      </div>
    </header>
    <main class="admin-main">
      <section v-if="adminMode === 'checking'" class="admin-card" aria-live="polite">
        <p class="loading-state">正在检查登录状态…</p>
      </section>
      <section v-else-if="adminMode === 'authenticated'" class="admin-card admin-card--workspace">
        <div class="admin-card__header">
          <div>
            <p class="admin-eyebrow">管理员 · {{ adminUser?.username }}</p>
            <h1>管理文档</h1>
          </div>
          <button class="admin-link-button" type="button" :disabled="adminBusy" @click="adminLogout">退出登录</button>
        </div>
        <p v-if="adminMessage" class="admin-feedback" role="status">{{ adminMessage }}</p>
        <div class="admin-editor">
          <aside class="admin-document-list" aria-label="文档列表">
            <button class="admin-new-button" type="button" :disabled="adminDocumentBusy" @click="startNewAdminDocument">＋ 新建文档</button>
            <button
              v-for="document in adminDocuments"
              :key="document.path"
              type="button"
              class="admin-document-item"
              :class="{ 'admin-document-item--active': adminSelectedPath === document.path }"
              @click="selectAdminDocument(document.path)"
            >
              <strong>{{ document.title }}</strong>
              <span>{{ document.path }}<template v-if="document.draft"> · 草稿</template></span>
            </button>
            <p v-if="!adminDocuments.length" class="sidebar__empty">暂无文档</p>
          </aside>
          <form class="admin-editor-form" @submit.prevent="saveAdminDocument">
            <label for="admin-document-path">文件路径</label>
            <input id="admin-document-path" v-model="adminEditingPath" required placeholder="例如：guides/intro.md" :disabled="Boolean(adminSelectedPath)">
            <label for="admin-document-content">Markdown 内容</label>
            <textarea id="admin-document-content" v-model="adminDocumentContent" rows="20" spellcheck="false" :disabled="adminDocumentBusy"></textarea>
            <p v-if="adminDocumentError" class="admin-feedback admin-feedback--error" role="alert">{{ adminDocumentError }}</p>
            <div class="admin-editor-actions">
              <button class="admin-button" type="submit" :disabled="adminDocumentBusy">{{ adminDocumentBusy ? '处理中…' : '保存文档' }}</button>
              <button v-if="adminSelectedPath" class="admin-danger-button" type="button" :disabled="adminDocumentBusy" @click="deleteAdminDocument">删除</button>
            </div>
          </form>
        </div>
      </section>
      <section v-else class="admin-card">
        <p class="admin-eyebrow">Markdown 文档库 · 管理员</p>
        <h1>{{ adminMode === 'setup' ? '初始化管理员' : '管理员登录' }}</h1>
        <p class="article__lead">
          {{ adminMode === 'setup' ? '首次部署时创建唯一管理员账号。' : '登录后管理文档内容。' }}
        </p>
        <form class="admin-form" @submit.prevent="submitAdminAuth(adminMode === 'setup' ? 'setup' : 'login')">
          <label for="admin-username">用户名</label>
          <input id="admin-username" v-model="adminUsername" autocomplete="username" required minlength="3" maxlength="64">
          <label for="admin-password">密码</label>
          <input id="admin-password" v-model="adminPassword" type="password" autocomplete="new-password" required>
          <template v-if="adminMode === 'setup'">
            <label for="admin-password-confirmation">确认密码</label>
            <input id="admin-password-confirmation" v-model="adminPasswordConfirmation" type="password" autocomplete="new-password" required>
          </template>
          <p v-if="adminError" class="admin-feedback admin-feedback--error" role="alert">{{ adminError }}</p>
          <p v-if="adminMessage" class="admin-feedback" role="status">{{ adminMessage }}</p>
          <button class="admin-button" type="submit" :disabled="adminBusy">
            {{ adminBusy ? '处理中…' : adminMode === 'setup' ? '创建管理员' : '登录' }}
          </button>
        </form>
        <button class="admin-link-button" type="button" @click="adminMode = adminMode === 'setup' ? 'login' : 'setup'; adminError = ''; adminMessage = ''">
          {{ adminMode === 'setup' ? '已有管理员？去登录' : '首次部署？初始化管理员' }}
        </button>
      </section>
    </main>
  </div>
  <div v-else class="app-shell">
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
