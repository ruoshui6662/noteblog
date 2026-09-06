<script setup lang="ts">
import { onMounted, ref } from "vue";

type Site = {
  name: string;
  stage: string;
  environment: string;
};

const site = ref<Site | null>(null);
const status = ref<"loading" | "ready" | "error">("loading");

onMounted(async () => {
  try {
    const response = await fetch("/api/v1/site");
    if (!response.ok) throw new Error("Site metadata request failed");
    site.value = (await response.json()) as Site;
    status.value = "ready";
  } catch {
    status.value = "error";
  }
});
</script>

<template>
  <div class="app-shell">
    <header class="app-header">
      <div class="app-header__inner">
        <a class="brand" href="#main-content" aria-label="跳到主要内容">
          <span class="brand__mark" aria-hidden="true">M</span>
          <span>{{ site?.name ?? "Markdown 文档库" }}</span>
        </a>
        <span class="stage-badge">{{ site?.environment ?? "连接中" }} · {{ site?.stage ?? "M0" }}</span>
      </div>
    </header>

    <div class="docs-layout">
      <aside class="sidebar" aria-label="文档导航">
        <p class="sidebar__label">开发导航</p>
        <nav>
          <a class="nav-item nav-item--active" href="#main-content" aria-current="page">工程基线</a>
          <span class="nav-item nav-item--disabled">内容扫描（下一阶段）</span>
          <span class="nav-item nav-item--disabled">权限过滤（下一阶段）</span>
        </nav>
      </aside>

      <main id="main-content" class="article" tabindex="-1">
        <p class="breadcrumb">文档库 <span aria-hidden="true">/</span> 工程基线</p>
        <h1>工程基线已启动</h1>
        <p class="article__lead">前端、后端与本地数据目录已经建立。下一阶段将接入真实 Markdown 文档树。</p>

        <section class="notice" aria-live="polite">
          <h2>服务状态</h2>
          <p v-if="status === 'loading'">正在连接服务…</p>
          <p v-else-if="status === 'ready'">服务已就绪：{{ site?.environment }} 环境。</p>
          <p v-else>无法连接服务。请检查服务状态和运行日志，然后刷新页面。</p>
        </section>

        <section>
          <h2>本阶段完成内容</h2>
          <ul>
            <li>Go 健康检查与数据目录初始化。</li>
            <li>Vue + Vite 本地开发入口与 API 代理。</li>
            <li>源自样式基准的三层令牌和阅读布局骨架。</li>
          </ul>
        </section>
      </main>

      <aside class="toc" aria-label="本页目录">
        <p class="toc__label">本页</p>
        <a href="#main-content" aria-current="location">工程基线</a>
      </aside>
    </div>
  </div>
</template>
