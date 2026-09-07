<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import SiteHeader from "./SiteHeader.vue";
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
const adminDocumentTitle = ref("");
const adminDirectory = ref("");
const adminDocumentFrontMatter = ref("");
const adminDocumentContent = ref("");
const adminDocumentHash = ref("");
const adminDocumentBusy = ref(false);
const adminDocumentError = ref("");


const adminDirectoryOptions = computed(() => {
  const directories = new Set<string>([""]);
  for (const document of adminDocuments.value) {
    const parts = document.path.split("/");
    parts.pop();
    for (let index = 1; index <= parts.length; index += 1) directories.add(parts.slice(0, index).join("/"));
  }
  return [...directories].sort((left, right) => left.localeCompare(right, "zh-CN"));
});

const adminGeneratedPath = computed(() => {
  if (adminSelectedPath.value) return adminSelectedPath.value;
  const slug = adminSlugFromTitle(adminDocumentTitle.value) || "new-document";
  const directory = adminDirectory.value.replace(/^\/+|\/+$/g, "");
  const prefix = directory ? `${directory}/` : "";
  const occupied = new Set(adminDocuments.value.map((document) => document.path));
  let candidate = `${prefix}${slug}.md`;
  let suffix = 2;
  while (occupied.has(candidate)) {
    candidate = `${prefix}${slug}-${suffix}.md`;
    suffix += 1;
  }
  return candidate;
});


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
    const contentParts = adminSplitContent(payload.content);
    adminSelectedPath.value = payload.path;
    adminEditingPath.value = payload.path;
    adminDirectory.value = adminDirectoryFromPath(payload.path);
    adminDocumentTitle.value = adminTitleFromContent(payload.content, adminDocuments.value.find((item) => item.path === payload.path)?.title);
    adminDocumentFrontMatter.value = contentParts.metadata;
    adminDocumentContent.value = contentParts.body;
    adminDocumentHash.value = payload.hash;
  } catch (error) {
    adminDocumentError.value = error instanceof Error ? error.message : "无法读取文档内容";
  } finally {
    adminDocumentBusy.value = false;
  }
}

function startNewAdminDocument() {
  adminSelectedPath.value = "";
  adminEditingPath.value = "";
  adminDocumentTitle.value = "新文档";
  adminDirectory.value = "";
  adminDocumentFrontMatter.value = "title: 新文档";
  adminDocumentContent.value = "开始写作。\n";
  adminDocumentHash.value = "";
  adminDocumentError.value = "";
}

function adminDirectoryFromPath(path: string) {
  const separator = path.lastIndexOf("/");
  return separator === -1 ? "" : path.slice(0, separator);
}

function adminSlugFromTitle(title: string) {
  return title
    .trim()
    .toLocaleLowerCase()
    .replace(/\s+/g, "-")
    .replace(/[^\p{L}\p{N}_-]+/gu, "-")
    .replace(/-+/g, "-")
    .replace(/^-|-$/g, "");
}

function adminSplitContent(content: string) {
  const frontMatter = content.match(/^---\r?\n([\s\S]*?)\r?\n---(?:\r?\n|$)/);
  return { metadata: frontMatter?.[1] ?? "", body: frontMatter ? content.slice(frontMatter[0].length) : content };
}

function adminTitleFromContent(content: string, fallback = "") {
  const frontMatter = content.match(/^---\r?\n([\s\S]*?)\r?\n---(?:\r?\n|$)/);
  const titleLine = frontMatter?.[1].match(/^title\s*:\s*(.+?)\s*$/m)?.[1]?.trim();
  if (titleLine) {
    if (titleLine.startsWith("'") && titleLine.endsWith("'")) return titleLine.slice(1, -1).replaceAll("''", "'").trim();
    if (titleLine.startsWith('"') && titleLine.endsWith('"')) return titleLine.slice(1, -1).trim();
    return titleLine;
  }
  const heading = content.match(/^#\s+(.+?)\s*$/m)?.[1]?.trim();
  return heading || fallback || "未命名文档";
}

function adminContentWithTitle(content: string, title: string, metadata: string) {
  const safeTitle = title.trim().replace(/[\r\n]+/g, " ");
  const yamlTitle = `'${safeTitle.replaceAll("'", "''")}'`;
  const nextMetadata = /^title\s*:/m.test(metadata)
    ? metadata.replace(/^title\s*:.+$/m, `title: ${yamlTitle}`)
    : `title: ${yamlTitle}\n${metadata}`;
  return `---\n${nextMetadata}\n---\n${content}`;
}

async function saveAdminDocument() {
  const path = (adminSelectedPath.value || adminGeneratedPath.value).trim();
  if (!path) {
    adminDocumentError.value = "无法生成文档标识，请检查标题。";
    return;
  }
  const title = adminDocumentTitle.value.trim();
  if (!title) {
    adminDocumentError.value = "请输入文档标题。";
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
      body: JSON.stringify({ path: isNew ? path : undefined, content: adminContentWithTitle(adminDocumentContent.value, title, adminDocumentFrontMatter.value) }),
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


onMounted(() => void loadAdmin());
</script>

<template>
  <div class="admin-shell">
    <SiteHeader :reader="false" />
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
            <label for="admin-document-title">页面标题</label>
            <input id="admin-document-title" v-model="adminDocumentTitle" required placeholder="例如：部署指南" :disabled="adminDocumentBusy">
            <p class="admin-field-help">阅读页面显示的标题，保存时会写入 Markdown 的 <code>title</code> 元数据。</p>
            <label for="admin-document-directory">所属目录</label>
            <select id="admin-document-directory" v-model="adminDirectory" :disabled="Boolean(adminSelectedPath) || adminDocumentBusy">
              <option v-for="directory in adminDirectoryOptions" :key="directory" :value="directory">{{ directory || '文档根目录' }}</option>
            </select>
            <p class="admin-field-help">新文档会根据标题自动生成文件名；已存在文档保持原有位置和访问地址。</p>
            <p class="admin-path-preview"><span>系统文件标识</span><code>{{ adminGeneratedPath }}</code></p>
            <label for="admin-document-content">Markdown 内容</label>
            <textarea id="admin-document-content" v-model="adminDocumentContent" rows="20" spellcheck="false" :disabled="adminDocumentBusy"></textarea>
            <p class="admin-field-help">这里只编辑正文；标题和其他元数据会在保存时保留并自动同步。</p>
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
          <input id="admin-password" v-model="adminPassword" type="password" :autocomplete="adminMode === 'setup' ? 'new-password' : 'current-password'" required>
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

</template>
