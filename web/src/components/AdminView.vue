<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import SiteHeader from "./SiteHeader.vue";
import AppIcon from "./AppIcon.vue";
import type { Category } from "../types";
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
const adminCategories = ref<Category[]>([]);
const adminCategoryPath = ref("");
const adminCategoryTitle = ref("");
const adminCategoryDescription = ref("");
const adminCategoryOrder = ref(0);
const adminCategoryCollapsed = ref(false);
const adminCategoryBusy = ref(false);
const adminCategoryError = ref("");
const adminCategoryIsNew = ref(true);
const adminActiveCategoryPath = ref("");
const adminPanel = ref<"preview" | "properties">("preview");
const adminExplorerQuery = ref("");
const adminNewMenuOpen = ref(false);
const adminCategoryMenuPath = ref("");
const adminDocumentMenuPath = ref("");
const adminOpenPaths = ref<string[]>([]);
const adminSavedFingerprint = ref("");
const adminStructureBusy = ref(false);
const adminStructureError = ref("");

const filteredAdminDocuments = computed(() => {
  const query = adminExplorerQuery.value.trim().toLocaleLowerCase();
  if (!query) return adminDocuments.value;
  return adminDocuments.value.filter(document => `${document.title} ${document.path}`.toLocaleLowerCase().includes(query));
});
const adminRootDocuments = computed(() => filteredAdminDocuments.value.filter(document => !document.path.includes("/")));
const adminDocumentDirty = computed(() => adminSavedFingerprint.value !== adminDocumentFingerprint.value);
const adminPreviewHTML = computed(() => adminRenderPreview(adminDocumentTitle.value, adminDocumentContent.value));

const adminDocumentFingerprint = computed(() => JSON.stringify({
  path: adminSelectedPath.value,
  title: adminDocumentTitle.value,
  directory: adminDirectory.value,
  metadata: adminDocumentFrontMatter.value,
  content: adminDocumentContent.value,
}));

function adminCategoriesFromDocuments(documents: AdminDocumentSummary[]): Category[] {
  const paths = new Set<string>();
  for (const document of documents) {
    const parts = document.path.split("/");
    parts.pop();
    for (let index = 1; index <= parts.length; index += 1) paths.add(parts.slice(0, index).join("/"));
  }
  return [...paths].sort((left, right) => left.localeCompare(right, "zh-CN")).map((path, index) => ({
    path,
    title: path.split("/").at(-1)?.replace(/[-_]+/g, " ") || path,
    description: "根据文档路径识别的目录",
    order: index + 1,
    collapsed: false,
  }));
}


const adminDirectoryOptions = computed(() => {
  const directories = new Set<string>([""]);
  for (const category of adminCategories.value) {
    const parts = category.path.split("/");
    for (let index = 1; index <= parts.length; index += 1) directories.add(parts.slice(0, index).join("/"));
  }
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

function adminDocumentsInCategory(path: string) {
  return filteredAdminDocuments.value.filter(document => adminDirectoryFromPath(document.path) === path);
}

function adminOpenDocument(path: string) {
  adminNewMenuOpen.value = false;
  adminCategoryMenuPath.value = "";
  adminDocumentMenuPath.value = "";
  if (!adminOpenPaths.value.includes(path)) adminOpenPaths.value = [...adminOpenPaths.value, path];
  void selectAdminDocument(path);
}

function closeAdminDocument(path: string) {
  if (path === adminSelectedPath.value && adminDocumentDirty.value && !window.confirm("这篇文档还有未保存修改，确定关闭吗？")) return;
  adminOpenPaths.value = adminOpenPaths.value.filter(item => item !== path);
  if (path !== adminSelectedPath.value) return;
  const fallback = adminOpenPaths.value.at(-1);
  if (fallback) void selectAdminDocument(fallback);
  else startNewAdminDocument();
}

function adminDocumentTargetPath(path: string, value: string, suffix = "") {
  const input = value.trim().replaceAll("\\", "/").replace(/^\/+|\/+$/g, "");
  if (!input) return "";
  const directory = adminDirectoryFromPath(path);
  const name = input.includes("/") ? input : `${directory ? `${directory}/` : ""}${input}`;
  if (/\.(md|markdown)$/i.test(name)) return name;
  return `${name}${suffix || ".md"}`;
}

function adminReplaceOpenPath(sourcePath: string, targetPath: string) {
  adminOpenPaths.value = adminOpenPaths.value.map(path => path === sourcePath ? targetPath : path);
  if (adminSelectedPath.value === sourcePath) adminSelectedPath.value = targetPath;
  if (adminEditingPath.value === sourcePath) adminEditingPath.value = targetPath;
}

function adminReplaceOpenPrefix(sourcePath: string, targetPath: string) {
  const replace = (path: string) => path === sourcePath || path.startsWith(`${sourcePath}/`)
    ? `${targetPath}${path.slice(sourcePath.length)}`
    : path;
  adminOpenPaths.value = adminOpenPaths.value.map(replace);
  adminSelectedPath.value = replace(adminSelectedPath.value);
  adminEditingPath.value = replace(adminEditingPath.value);
}

async function adminStructureCall(url: string, payload: Record<string, string>) {
  adminStructureBusy.value = true;
  adminStructureError.value = "";
  try {
    const response = await fetch(url, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(payload) });
    const result = (await response.json().catch(() => ({}))) as { error?: { message?: string } };
    if (!response.ok) throw new Error(result.error?.message ?? "结构操作失败，请刷新后重试。");
    return result;
  } catch (error) {
    adminStructureError.value = error instanceof Error ? error.message : "结构操作失败，请刷新后重试。";
    return null;
  } finally {
    adminStructureBusy.value = false;
  }
}

async function moveAdminDocument(path: string) {
  adminDocumentMenuPath.value = "";
  const document = adminDocuments.value.find(item => item.path === path);
  if (!document) return;
  const input = window.prompt("移动到目标路径（可输入完整路径）", document.path);
  if (!input || input.trim() === document.path) return;
  const targetPath = adminDocumentTargetPath(path, input);
  if (!targetPath) return;
  const result = await adminStructureCall("/api/v1/admin/docs/move", { source_path: path, target_path: targetPath, expected_hash: document.hash });
  if (!result) return;
  adminReplaceOpenPath(path, targetPath);
  adminMessage.value = "文档已移动。";
  await loadAdminDocuments(targetPath);
  await loadAdminCategories();
}

async function duplicateAdminDocument(path: string) {
  adminDocumentMenuPath.value = "";
  const document = adminDocuments.value.find(item => item.path === path);
  if (!document) return;
  const base = path.replace(/\.(md|markdown)$/i, "-copy.md");
  const input = window.prompt("复制到目标路径", base);
  if (!input) return;
  const targetPath = adminDocumentTargetPath(path, input);
  if (!targetPath) return;
  const result = await adminStructureCall("/api/v1/admin/docs/duplicate", { source_path: path, target_path: targetPath, expected_hash: document.hash });
  if (!result) return;
  adminMessage.value = "文档副本已创建。";
  await loadAdminDocuments(targetPath);
  await loadAdminCategories();
}

async function moveAdminCategory(path: string) {
  adminCategoryMenuPath.value = "";
  const input = window.prompt("移动到目标目录路径", path);
  if (!input || input.trim() === path) return;
  const targetPath = input.trim().replaceAll("\\", "/").replace(/^\/+|\/+$/g, "");
  if (!targetPath) return;
  const result = await adminStructureCall("/api/v1/admin/categories/move", { source_path: path, target_path: targetPath });
  if (!result) return;
  adminReplaceOpenPrefix(path, targetPath);
  adminMessage.value = "目录已移动。";
  await loadAdminCategories(targetPath);
  await loadAdminDocuments();
}

async function deleteAdminCategory(path: string) {
  adminCategoryMenuPath.value = "";
  if (!window.confirm(`确定删除目录「${path}」吗？目录必须为空。`)) return;
  adminStructureBusy.value = true;
  adminStructureError.value = "";
  try {
    const response = await fetch(`/api/v1/admin/categories/${path.split("/").map(encodeURIComponent).join("/")}`, { method: "DELETE" });
    const result = (await response.json().catch(() => ({}))) as { error?: { message?: string } };
    if (!response.ok) throw new Error(result.error?.message ?? "目录删除失败，请刷新后重试。");
    adminMessage.value = "目录已删除。";
    adminActiveCategoryPath.value = "";
    await loadAdminCategories();
    await loadAdminDocuments();
  } catch (error) {
    adminStructureError.value = error instanceof Error ? error.message : "目录删除失败，请刷新后重试。";
  } finally {
    adminStructureBusy.value = false;
  }
}

function openNewDocument(directory = adminDirectory.value) {
  adminNewMenuOpen.value = false;
  adminCategoryMenuPath.value = "";
  adminDocumentMenuPath.value = "";
  startNewAdminDocument();
  adminDirectory.value = directory;
}

function openCategorySettings(path: string) {
  adminCategoryMenuPath.value = "";
  adminPanel.value = "properties";
  selectAdminCategory(path);
}

function adminEscape(value: string) {
  return value.replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;").replaceAll('"', "&quot;").replaceAll("'", "&#39;");
}

function adminRenderPreview(title: string, source: string) {
  const lines = source.replace(/\r\n/g, "\n").split("\n");
  const output = [`<h1>${adminEscape(title || "未命名文档")}</h1>`];
  let paragraph: string[] = [];
  let listOpen = false;
  let codeOpen = false;
  const flushParagraph = () => {
    if (paragraph.length) {
      output.push(`<p>${paragraph.join(" ")}</p>`);
      paragraph = [];
    }
  };
  const closeList = () => {
    if (listOpen) { output.push("</ul>"); listOpen = false; }
  };
  lines.forEach(line => {
    const trimmed = line.trim();
    if (trimmed.startsWith("```")) {
      flushParagraph(); closeList();
      if (!codeOpen) { output.push("<pre><code>"); codeOpen = true; } else { output.push("</code></pre>"); codeOpen = false; }
      return;
    }
    if (codeOpen) { output.push(`${adminEscape(line)}\n`); return; }
    if (!trimmed) { flushParagraph(); closeList(); return; }
    const heading = trimmed.match(/^(#{2,6})\s+(.+)$/);
    if (heading) { flushParagraph(); closeList(); const level = heading[1].length; output.push(`<h${level}>${adminEscape(heading[2])}</h${level}>`); return; }
    const item = trimmed.match(/^[-*+]\s+(.+)$/);
    if (item) { flushParagraph(); if (!listOpen) { output.push("<ul>"); listOpen = true; } output.push(`<li>${adminEscape(item[1])}</li>`); return; }
    closeList(); paragraph.push(adminEscape(trimmed));
  });
  flushParagraph(); closeList(); if (codeOpen) output.push("</code></pre>");
  return output.join("");
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
      await loadAdminCategories();
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
      await loadAdminCategories();
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
  if (nextPath) {
    if (!adminOpenPaths.value.includes(nextPath)) adminOpenPaths.value = [...adminOpenPaths.value, nextPath];
    await selectAdminDocument(nextPath);
  }
  else startNewAdminDocument();
}

async function loadAdminCategories(preferredPath = "") {
  adminCategoryError.value = "";
  const response = await fetch("/api/v1/admin/categories");
  if (response.status === 401) {
    adminMode.value = "login";
    adminUser.value = null;
    return;
  }
  if (!response.ok) {
    if (response.status === 404) {
      adminCategories.value = adminCategoriesFromDocuments(adminDocuments.value);
      const fallbackPath = preferredPath || adminCategoryPath.value || adminCategories.value[0]?.path || "";
      if (fallbackPath) selectAdminCategory(fallbackPath);
      else startNewAdminCategory();
      return;
    }
    throw new Error("无法读取目录列表");
  }
  adminCategories.value = (await response.json()) as Category[];
  const nextPath = preferredPath || adminCategoryPath.value || adminCategories.value[0]?.path || "";
  if (nextPath) selectAdminCategory(nextPath);
  else startNewAdminCategory();
}

function selectAdminCategory(path: string) {
  const category = adminCategories.value.find(item => item.path === path);
  if (!category) return;
  adminPanel.value = "properties";
  adminDocumentMenuPath.value = "";
  adminActiveCategoryPath.value = path;
  adminCategoryPath.value = category.path;
  adminCategoryIsNew.value = false;
  adminCategoryTitle.value = category.title;
  adminCategoryDescription.value = category.description ?? "";
  adminCategoryOrder.value = category.order;
  adminCategoryCollapsed.value = category.collapsed;
  adminCategoryError.value = "";
}

function startNewAdminCategory() {
  adminActiveCategoryPath.value = "";
  adminCategoryPath.value = "";
  adminCategoryIsNew.value = true;
  adminCategoryTitle.value = "新目录";
  adminCategoryDescription.value = "";
  adminCategoryOrder.value = adminCategories.value.length + 1;
  adminCategoryCollapsed.value = false;
  adminCategoryError.value = "";
}

async function saveAdminCategory() {
  const path = adminCategoryPath.value.trim().replace(/^\/+|\/+$/g, "");
  const title = adminCategoryTitle.value.trim();
  if (!path || !title) {
    adminCategoryError.value = "请输入目录路径和显示名称。";
    return;
  }
  adminCategoryBusy.value = true;
  adminCategoryError.value = "";
  try {
    const response = await fetch("/api/v1/admin/categories", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ path, title, description: adminCategoryDescription.value.trim(), order: Number(adminCategoryOrder.value) || 0, collapsed: adminCategoryCollapsed.value }),
    });
    const payload = (await response.json().catch(() => ({}))) as { error?: { message?: string } };
    if (!response.ok) {
      adminCategoryError.value = payload.error?.message ?? "目录保存失败。";
      return;
    }
    adminMessage.value = "目录设置已保存，文件夹和 _category.yml 已写入数据目录。";
    await loadAdminCategories(path);
    await loadAdminDocuments();
  } catch {
    adminCategoryError.value = "无法连接文档服务。";
  } finally {
    adminCategoryBusy.value = false;
  }
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
    adminActiveCategoryPath.value = "";
    adminCategoryIsNew.value = false;
    adminSavedFingerprint.value = adminDocumentFingerprint.value;
    adminPanel.value = "preview";
  } catch (error) {
    adminDocumentError.value = error instanceof Error ? error.message : "无法读取文档内容";
  } finally {
    adminDocumentBusy.value = false;
  }
}

function startNewAdminDocument() {
  adminActiveCategoryPath.value = "";
  adminCategoryIsNew.value = false;
  adminSelectedPath.value = "";
  adminEditingPath.value = "";
  adminDocumentTitle.value = "新文档";
  adminDirectory.value = "";
  adminDocumentFrontMatter.value = "title: 新文档";
  adminDocumentContent.value = "开始写作。\n";
  adminDocumentHash.value = "";
  adminDocumentError.value = "";
  adminSavedFingerprint.value = "";
  adminPanel.value = "preview";
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

async function deleteAdminDocumentAt(path: string, expectedHash: string) {
  adminDocumentMenuPath.value = "";
  if (!path || !window.confirm(`确定删除「${path}」吗？`)) return;
  adminDocumentBusy.value = true;
  adminDocumentError.value = "";
  try {
    const response = await fetch(`/api/v1/admin/docs/${path.split("/").map(encodeURIComponent).join("/")}`, {
      method: "DELETE",
      headers: { "If-Match": `"${expectedHash}"` },
    });
    const payload = (await response.json().catch(() => ({}))) as { error?: { message?: string } };
    if (!response.ok) {
      adminDocumentError.value = payload.error?.message ?? "删除失败，请刷新后重试。";
      return;
    }
    adminMessage.value = "文档已删除。";
    adminOpenPaths.value = adminOpenPaths.value.filter(item => item !== path);
    if (adminSelectedPath.value === path) {
      adminSelectedPath.value = "";
      await loadAdminDocuments();
    } else {
      await loadAdminDocuments(adminSelectedPath.value);
    }
  } catch {
    adminDocumentError.value = "无法连接文档服务。";
  } finally {
    adminDocumentBusy.value = false;
  }
}

async function deleteAdminDocument() {
  await deleteAdminDocumentAt(adminSelectedPath.value, adminDocumentHash.value);
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
    <SiteHeader v-if="adminMode !== 'authenticated'" :reader="false" />
    <main class="admin-main" :class="{ 'admin-main--workspace': adminMode === 'authenticated' }">
      <section v-if="adminMode === 'checking'" class="admin-card" aria-live="polite">
        <p class="loading-state">正在检查登录状态…</p>
      </section>
      <section v-else-if="adminMode === 'authenticated'" class="admin-workspace" aria-label="文档管理工作区">
        <header class="admin-workspace__topbar">
          <div class="admin-workspace__brand"><a href="/" aria-label="返回文档首页">RUOSHUI 知识库</a><span class="admin-workspace__crumb">管理端</span></div>
          <label class="admin-command-search"><AppIcon name="search" /><span class="sr-only">搜索文档或命令</span><input v-model="adminExplorerQuery" type="search" placeholder="搜索文档或输入命令…" /></label>
          <div class="admin-workspace__actions">
            <span class="admin-save-state" :class="{ 'admin-save-state--dirty': adminDocumentDirty }"><AppIcon :name="adminDocumentDirty ? 'file-text' : 'check'" />{{ adminDocumentDirty ? '未保存' : '已保存' }}</span>
            <button class="admin-workspace__quiet" type="button" @click="adminPanel = 'preview'">预览</button>
            <button class="admin-workspace__quiet" type="button" @click="adminPanel = 'properties'">属性</button>
            <button class="admin-workspace__quiet" type="button" :disabled="adminBusy" @click="adminLogout">退出</button>
            <button class="admin-workspace__primary" type="button" :disabled="adminDocumentBusy || !adminDocumentTitle.trim()" @click="saveAdminDocument">{{ adminDocumentBusy ? '保存中…' : '保存更改' }}</button>
          </div>
        </header>
        <p v-if="adminMessage" class="admin-workspace__message" role="status">{{ adminMessage }}</p>
        <p v-if="adminStructureError" class="admin-workspace__message admin-workspace__message--error" role="alert">{{ adminStructureError }}</p>
        <div class="admin-workspace__body">
          <aside class="admin-explorer" aria-label="文件浏览器">
            <div class="admin-explorer__header"><strong>文档库</strong><button class="admin-explorer__icon" type="button" aria-label="新建菜单" @click="adminNewMenuOpen = !adminNewMenuOpen">新建</button></div>
            <div v-if="adminNewMenuOpen" class="admin-explorer__menu"><button type="button" @click="openNewDocument()">新建文档</button><button type="button" @click="startNewAdminCategory(); adminNewMenuOpen = false">新建目录</button></div>
            <div class="admin-explorer__quick"><button type="button" class="is-active">全部文档 <span>{{ adminDocuments.length }}</span></button><button type="button">最近编辑</button></div>
            <div class="admin-explorer__section"><div class="admin-explorer__section-title"><span>目录</span><button type="button" aria-label="新建目录" @click="startNewAdminCategory">＋</button></div>
              <div v-for="category in adminCategories" :key="category.path" class="admin-explorer__category">
                <div class="admin-explorer__category-row" :class="{ 'is-active': adminActiveCategoryPath === category.path }"><button type="button" class="admin-explorer__category-name" @click="selectAdminCategory(category.path)"><AppIcon name="chevron-down" />{{ category.title }}</button><button type="button" class="admin-explorer__more" aria-label="目录操作" @click.stop="adminCategoryMenuPath = adminCategoryMenuPath === category.path ? '' : category.path">更多</button></div>
                <div v-if="adminCategoryMenuPath === category.path" class="admin-explorer__context"><button type="button" @click="openCategorySettings(category.path)">目录设置</button><button type="button" @click="openNewDocument(category.path)">在此新建文档</button><button type="button" @click="moveAdminCategory(category.path)">移动目录</button><button type="button" @click="deleteAdminCategory(category.path)">删除空目录</button></div>
                <div v-for="document in adminDocumentsInCategory(category.path)" :key="document.path" class="admin-explorer__document-row"><button type="button" class="admin-explorer__document" :class="{ 'is-active': adminSelectedPath === document.path }" @click="adminOpenDocument(document.path)"><AppIcon name="file-text" /><span>{{ document.title }}</span><small v-if="document.draft">草稿</small></button><button type="button" class="admin-explorer__document-more" aria-label="文档操作" @click.stop="adminDocumentMenuPath = adminDocumentMenuPath === document.path ? '' : document.path">更多</button><div v-if="adminDocumentMenuPath === document.path" class="admin-explorer__context"><button type="button" @click="adminOpenDocument(document.path); adminDocumentMenuPath = ''">打开文档</button><button type="button" @click="moveAdminDocument(document.path)">移动文档</button><button type="button" @click="duplicateAdminDocument(document.path)">复制文档</button><button type="button" @click="deleteAdminDocumentAt(document.path, document.hash)">删除文档</button></div></div>
              </div>
              <div v-if="adminRootDocuments.length" class="admin-explorer__category"><span class="admin-explorer__category-name admin-explorer__category-name--root">根目录</span><div v-for="document in adminRootDocuments" :key="document.path" class="admin-explorer__document-row"><button type="button" class="admin-explorer__document" :class="{ 'is-active': adminSelectedPath === document.path }" @click="adminOpenDocument(document.path)"><AppIcon name="file-text" /><span>{{ document.title }}</span><small v-if="document.draft">草稿</small></button><button type="button" class="admin-explorer__document-more" aria-label="文档操作" @click.stop="adminDocumentMenuPath = adminDocumentMenuPath === document.path ? '' : document.path">更多</button><div v-if="adminDocumentMenuPath === document.path" class="admin-explorer__context"><button type="button" @click="moveAdminDocument(document.path)">移动文档</button><button type="button" @click="duplicateAdminDocument(document.path)">复制文档</button><button type="button" @click="deleteAdminDocumentAt(document.path, document.hash)">删除文档</button></div></div></div>
              <p v-if="!adminCategories.length && !adminRootDocuments.length" class="admin-explorer__empty">暂无文档或目录</p>
            </div>
            <div class="admin-explorer__footer"><button type="button" @click="adminPanel = 'properties'">设置</button></div>
          </aside>
          <main class="admin-editor-pane">
            <div class="admin-tabs" role="tablist" aria-label="打开的文档"><button v-for="path in adminOpenPaths" :key="path" type="button" role="tab" :aria-selected="adminSelectedPath === path" :class="{ 'is-active': adminSelectedPath === path }" @click="adminOpenDocument(path)">{{ adminDocuments.find(item => item.path === path)?.title || path }}<span role="button" tabindex="0" aria-label="关闭标签" @click.stop="closeAdminDocument(path)"><AppIcon name="x" /></span></button><button type="button" class="admin-tabs__new" @click="openNewDocument()">新标签</button></div>
            <div class="admin-editor-pane__path"><span>{{ adminDirectory || '文档根目录' }}</span><span aria-hidden="true">/</span><strong>{{ adminSelectedPath || adminGeneratedPath }}</strong></div>
            <form class="admin-editor-form admin-editor-form--workspace" @submit.prevent="saveAdminDocument">
              <input id="admin-document-title" v-model="adminDocumentTitle" class="admin-editor-title" required placeholder="未命名文档" :disabled="adminDocumentBusy" aria-label="文档标题">
              <div class="admin-editor-toolbar"><span>Markdown</span><span>支持标题、列表、代码块和链接</span></div>
              <textarea id="admin-document-content" v-model="adminDocumentContent" class="admin-editor-textarea" spellcheck="false" :disabled="adminDocumentBusy" aria-label="Markdown 内容"></textarea>
              <p v-if="adminDocumentError" class="admin-feedback admin-feedback--error" role="alert">{{ adminDocumentError }}</p>
              <div class="admin-editor-status"><span>{{ adminDocumentContent.length }} 字符</span><span>{{ adminDocumentDirty ? '修改尚未保存' : '所有修改已保存' }}</span><div><button class="admin-workspace__quiet" type="button" @click="adminPanel = 'preview'">打开预览</button><button class="admin-workspace__primary" type="submit" :disabled="adminDocumentBusy || !adminDocumentTitle.trim()">{{ adminDocumentBusy ? '保存中…' : '保存更改' }}</button></div></div>
            </form>
          </main>
          <aside class="admin-inspector" aria-label="文档辅助面板">
            <div class="admin-inspector__tabs"><button type="button" :class="{ 'is-active': adminPanel === 'preview' }" @click="adminPanel = 'preview'">预览</button><button type="button" :class="{ 'is-active': adminPanel === 'properties' }" @click="adminPanel = 'properties'">属性</button></div>
            <div v-if="adminPanel === 'preview'" class="admin-preview"><div class="admin-preview__meta"><span>实时预览</span><span>{{ adminDocumentDirty ? '草稿' : '已保存' }}</span></div><article class="admin-preview__body" v-html="adminPreviewHTML"></article></div>
            <div v-else-if="adminActiveCategoryPath || adminCategoryIsNew" class="admin-inspector__content"><p class="admin-eyebrow">目录设置</p><h2>{{ adminCategoryIsNew ? '新建目录' : adminCategoryTitle }}</h2><form class="admin-category-form admin-category-form--inspector" @submit.prevent="saveAdminCategory"><label for="admin-category-path">目录路径</label><input id="admin-category-path" v-model="adminCategoryPath" placeholder="例如：getting-started" :disabled="adminCategoryBusy || !adminCategoryIsNew"><label for="admin-category-title">显示名称</label><input id="admin-category-title" v-model="adminCategoryTitle" required placeholder="例如：快速开始" :disabled="adminCategoryBusy"><label for="admin-category-description">目录说明</label><textarea id="admin-category-description" v-model="adminCategoryDescription" rows="3" placeholder="在首页分类卡片中显示" :disabled="adminCategoryBusy"></textarea><div class="admin-category-options"><label for="admin-category-order">排序</label><input id="admin-category-order" v-model.number="adminCategoryOrder" type="number" min="0" step="1" :disabled="adminCategoryBusy"><label class="admin-checkbox"><input v-model="adminCategoryCollapsed" type="checkbox" :disabled="adminCategoryBusy"> 默认折叠</label></div><p v-if="adminCategoryError" class="admin-feedback admin-feedback--error" role="alert">{{ adminCategoryError }}</p><button class="admin-workspace__primary admin-workspace__primary--wide" type="submit" :disabled="adminCategoryBusy">{{ adminCategoryBusy ? '保存中…' : '保存目录设置' }}</button></form></div>
            <div v-else class="admin-inspector__content"><p class="admin-eyebrow">文档属性</p><h2>{{ adminDocumentTitle || '未命名文档' }}</h2><label class="admin-property-label" for="admin-document-directory">所属目录</label><select id="admin-document-directory" v-model="adminDirectory" :disabled="Boolean(adminSelectedPath) || adminDocumentBusy"><option v-for="directory in adminDirectoryOptions" :key="directory" :value="directory">{{ directory || '文档根目录' }}</option></select><p class="admin-field-help">新文档会根据标题生成文件名；已存在文档保持原有路径。</p><label class="admin-property-label" for="admin-document-front-matter">文档元数据</label><textarea id="admin-document-front-matter" v-model="adminDocumentFrontMatter" rows="8" :disabled="adminDocumentBusy" placeholder="description: 文档说明&#10;tags: [指南]"></textarea><p class="admin-path-preview"><span>文件标识</span><code>{{ adminSelectedPath || adminGeneratedPath }}</code></p><button v-if="adminSelectedPath" class="admin-danger-button" type="button" :disabled="adminDocumentBusy" @click="deleteAdminDocument">删除文档</button></div>
          </aside>
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
