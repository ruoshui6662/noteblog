# Markdown 文档展示系统开发计划手册

> 文档状态：开发基线 1.0  
> 编制日期：2026-09-06  
> 适用范围：Windows 本地开发、Docker 部署、GitHub Actions 发布、飞牛 fnOS 安装与后续 FPK 封装  
> 样式基准：`ui/docs-site-template.zip`

## 1. 手册目的

本手册是项目后续设计、开发、测试和发布的共同执行依据。它把产品目标、技术架构和 `ui` 目录中的样式原型转换为可实现、可测试、可验收的开发约束。

用词约定：

- **必须**：发布前不可省略，否则视为未完成。
- **应该**：默认执行；只有记录理由后才能偏离。
- **可以**：按阶段和优先级选择实现。

发生冲突时，按以下优先级处理：

1. 数据安全、访问安全和可恢复性。
2. 本手册中的产品与架构约束。
3. `ui/docs-site-template.zip` 的视觉和交互基准。
4. 具体框架或组件库的默认行为。

## 2. 产品定义

项目定位为一个轻量、自托管、Markdown 文件原生的文档展示与管理系统。

- **阅读端**：匿名或授权用户浏览文档，采用左侧导航、中央正文、右侧文章目录的三栏布局。
- **管理端**：管理员创建、编辑、上传、移动、排序和发布 Markdown 文档及附件。

这里的“静态文档展示”指阅读端以只读、低交互和高缓存方式工作，并不表示整个系统是纯静态站点。由于存在登录、上传和在线编辑，生产形态必须包含后端服务。后续可以额外提供“导出纯静态站点”能力。

### 2.1 第一阶段目标

- Windows 上可直接启动前后端开发环境。
- 目录中的 Markdown 文件能自动形成左侧文档树。
- Markdown 能稳定、安全地渲染为文档正文。
- 提供单管理员登录和基础管理后台。
- 未登录访客只能看到管理员明确公开的文档或分类，默认拒绝访问。
- 管理员登录后能查看和管理全部公开、私有、草稿与归档内容。
- 支持创建、编辑、上传 Markdown 和图片附件。
- 支持中文标题、中文路径和中文搜索。
- 可构建为单容器 Docker 镜像。
- 容器删除重建后，挂载目录中的数据保持完整。
- GitHub Actions 能测试并发布 AMD64、ARM64 镜像到 GHCR。
- 能在飞牛 fnOS 上通过 Docker Compose 安装运行。

### 2.2 第一阶段不做

- 实时多人协作编辑和 Notion 式块编辑器。
- 复杂组织、空间、细粒度文档权限和审批流。
- Elasticsearch、Meilisearch 等独立搜索服务。
- 任意 Office 文件在线预览。
- 自动执行 Markdown 中的脚本或不受控 HTML。

## 3. 已确定的架构决策

### 3.1 技术栈

| 层级 | 选型 | 说明 |
|---|---|---|
| 阅读端与管理端 | Vue 3 + TypeScript + Vite | 组件化、构建轻、适合嵌入 Go 服务 |
| 路由 | Vue Router | 阅读端和管理端统一前端路由 |
| 编辑器 | CodeMirror 6 | Markdown 源码编辑、快捷键、可扩展且比 Monaco 轻 |
| 后端 | Go 当前稳定版 | 单二进制、低资源、多架构友好 |
| HTTP 路由 | 标准库或 `chi` | 保持依赖少，不采用重量级服务框架 |
| Markdown | Goldmark + GFM 扩展 | 后端统一解析，避免预览和正式渲染不一致 |
| 代码高亮 | Chroma | 服务端生成代码高亮 HTML |
| 数据库 | SQLite，优先纯 Go 驱动 | 保存用户、会话、设置和操作记录 |
| 文档存储 | 磁盘 Markdown | 文档正文不锁定在数据库中 |
| 容器 | 单容器、多阶段构建 | Go 同时提供 API 和嵌入后的前端资源 |
| 镜像仓库 | GitHub Container Registry | 与 GitHub Actions 和 Releases 统一 |

### 3.2 样式原型与生产技术栈的边界

`ui/docs-site-template.zip` 是视觉与交互规范，不是生产代码基础。原型的 `app/page.tsx` 将示例数据、页面内容、状态和组件集中在一个文件中，适合演示，但不适合作为可维护的管理系统。

开发时必须保留原型的布局、层级、色彩和关键交互，同时完成以下重构：

- React 状态改为 Vue 组件与组合式函数。
- 硬编码示例文档改为后端 API 数据。
- Tailwind 工具类中的视觉值沉淀为设计令牌。
- 阅读内容改为真实 Markdown 渲染结果。
- 搜索从“过滤菜单标题”升级为标题、标签、章节和正文搜索。
- 页面切换改为可复制、可刷新、可前进后退的真实 URL。
- 补全加载、错误、空数据、无权限和离线状态。

## 4. 总体系统结构

```text
浏览器
├─ 阅读端 /docs/*
└─ 管理端 /admin/*
        │ HTTPS / REST JSON
        ▼
Go 单体服务
├─ 静态资源服务（嵌入 Vue 构建产物）
├─ 公共文档 API
├─ 管理 API 与登录会话
├─ Markdown 解析、目录提取和缓存
├─ 文件树、搜索索引和文件监听
└─ 健康检查、日志和备份接口
        │
        ├─ /data/content   Markdown 文档
        ├─ /data/media     图片与附件
        ├─ /data/backups   备份包
        └─ /data/app.db    用户、权限、设置、会话、审计信息
```

运行原则：

- Markdown 文件是文档正文的事实源。
- SQLite 不保存唯一一份正文。
- 前端构建产物通过 `go:embed` 嵌入后端。
- 生产环境不需要 Node.js、Nginx、Redis 或外部数据库。
- 公共读取接口与管理写入接口必须隔离。
- 服务启动时扫描内容目录，运行时监听外部文件变化。
- 所有写入必须限制在配置的数据根目录中。

## 5. 项目目录规划

```text
/
├─ cmd/server/                 # Go 程序入口
├─ internal/
│  ├─ api/                     # HTTP 路由、请求和响应模型
│  ├─ auth/                    # 用户、密码、会话、CSRF
│  ├─ authorization/           # 主体、动作、资源、继承与批量过滤
│  ├─ config/                  # 环境变量与配置加载
│  ├─ content/                 # 文件扫描、读写、移动、排序
│  ├─ markdown/                # 解析、渲染、目录和链接处理
│  ├─ search/                  # 索引和搜索评分
│  ├─ storage/                 # SQLite 与迁移
│  └─ web/                     # 前端嵌入与 SPA fallback
├─ web/
│  ├─ src/
│  │  ├─ api/ assets/ components/ composables/
│  │  ├─ layouts/ pages/ router/ types/
│  │  └─ styles/tokens/
│  └─ tests/
├─ migrations/                # SQLite 迁移文件
├─ examples/content/          # 演示 Markdown 数据
├─ deploy/                    # Dockerfile 与 Compose
├─ packaging/fnos/            # 后续 FPK 工程
├─ scripts/                   # Windows 开发与验证脚本
├─ docs/
├─ ui/                        # 样式原型，只作规范参考
├─ .github/workflows/
├─ go.mod
└─ README.md
```

运行时数据不得提交到 Git。开发数据目录使用 `var/` 或 `.local/` 并加入 `.gitignore`。

## 6. UI 样式实施规范

### 6.1 原型基线

| 区域 | 原型值 | 实施要求 |
|---|---:|---|
| 顶部导航 | 固定高度 56px | 全站固定，滚动时保持可见 |
| 页面最大宽度 | 90rem / 1440px | 大屏居中，避免内容无限拉伸 |
| 左侧栏 | 280px | 桌面固定宽度，独立纵向滚动 |
| 正文最大宽度 | 48rem / 768px | 保证长文阅读行长 |
| 右侧目录 | 224px | 仅超宽屏显示，滚动时 sticky |
| 正文横向内边距 | 24/40/64px | 随断点递增 |
| 正文顶部内边距 | 40px | 与面包屑形成稳定节奏 |
| 主页面背景 | 白色 | 第一版以亮色主题为验收基准 |
| 左侧栏背景 | Gray 50 | 与正文形成轻微层级，不使用重阴影 |
| 主强调色 | Indigo | 选中、链接、焦点和品牌图标统一使用 |
| 代码块 | Gray 900 | 顶栏 Gray 800、边框 Gray 700 |

### 6.2 响应式规则

- `< 768px`：顶部搜索移入左侧抽屉；正文左右内边距为 24px。
- `< 1024px`：左侧栏变为左侧滑入抽屉；打开时显示 20% 黑色遮罩。
- `>= 1024px`：左侧栏为 sticky 桌面导航；正文内边距增加到 64px。
- `< 1280px`：隐藏右侧文章目录。
- `>= 1280px`：显示 224px 右侧文章目录。
- 移动抽屉必须支持 Escape 关闭、焦点管理和背景滚动管理。
- 不得仅依赖鼠标悬停完成任何核心功能。

### 6.3 字体与排版

第一版使用系统无衬线字体栈，中文优先使用系统可用的中文 UI 字体，不从公网 CDN 加载字体。

| 用途 | 大小 | 字重 | 行高 |
|---|---:|---:|---:|
| 页面 H1 | 30px，桌面 36px | 700 | 1.25 |
| 正文 H2 | 24px | 700 | 1.3 |
| 正文 H3 | 20px | 600 | 1.4 |
| 页面副标题 | 18px | 400 | 1.625 |
| 正文 | 16px | 400 | 1.625 |
| 导航与表格 | 14px | 400/500 | 1.5 |
| 分组标题与徽标 | 12px | 600 | 1.5 |
| 代码 | 14px | 400 | 1.625 |

### 6.4 三层设计令牌

所有生产组件必须使用“原始值 → 语义值 → 组件值”三层令牌。组件代码不得直接写十六进制颜色。

```text
web/src/styles/tokens/
├─ primitives.css
├─ semantic.css
├─ components.css
└─ index.css
```

原始令牌记录：Indigo、Gray 和状态色阶；4px 基础间距；12 至 36px 字号；4、8、12px 圆角；150、200、300ms 动效；全局 z-index 序列。

语义令牌至少定义：

```css
--color-background;
--color-foreground;
--color-surface-subtle;
--color-surface-elevated;
--color-border;
--color-border-subtle;
--color-text-primary;
--color-text-secondary;
--color-text-muted;
--color-primary;
--color-primary-hover;
--color-primary-active;
--color-primary-subtle;
--color-focus-ring;
--color-success;
--color-warning;
--color-danger;
--color-info;
```

组件令牌至少覆盖顶栏、左侧栏、文章、搜索框、代码块、右侧目录、对话框和编辑器。

暗色模式通过覆盖语义令牌实现，不复制组件样式。第一版先完成亮色视觉一致性；暗色模式在组件结构稳定后进入 v0.2。

### 6.5 状态、动效与可访问性

- 每个交互组件覆盖 default、hover、focus-visible、active、disabled、loading，以及适用的 error、empty 状态。
- 状态优先级为 `disabled > loading > active > focus > hover > default`。
- 焦点使用 2px 环和 2px offset。
- 正常文本对比度不低于 4.5:1；大型文本和 UI 边界不低于 3:1。
- 状态不得只依赖颜色表达。
- 颜色、边框、透明度过渡 150ms；抽屉位移 200ms；轻微阴影或位移 200ms。
- 复制成功状态保持约 2 秒。
- 动效必须尊重 `prefers-reduced-motion`。

## 7. 阅读端组件与交互

| 组件 | 职责 | 关键状态 |
|---|---|---|
| `AppHeader` | Logo、站点名、版本、搜索、外链、移动菜单 | 固定、移动端、菜单打开 |
| `VersionSelector` | 选择文档版本 | 默认、展开、加载、无其他版本 |
| `GlobalSearch` | 输入查询并展示结果 | 空、输入、加载、无结果、错误 |
| `DocsSidebar` | 分类和文档树 | 收起、展开、活动项、过滤结果 |
| `MobileSidebarDrawer` | 移动端导航 | 打开、关闭、遮罩、Escape |
| `Breadcrumbs` | 当前文档层级 | 长标题截断、窄屏 |
| `MarkdownArticle` | 展示安全 HTML | 加载、空文档、解析错误 |
| `ArticleToc` | 文章内标题导航 | 活动标题、无标题、滚动同步 |
| `CodeBlock` | 代码、语言标签、复制 | 默认、复制成功、复制失败 |
| `Callout` | note/info/warning/danger | 四种语义变体 |
| `PrevNextNav` | 上一篇、下一篇 | 单侧为空、两侧为空 |
| `EmptyState` | 空分类、空搜索、空文档 | 带主操作或纯说明 |
| `SiteFooter` | 版权、隐私、条款、GitHub | 桌面横排、移动竖排 |

交互要求：

- 左侧活动文档和右侧活动标题均使用 Indigo 语义色与 2px 左边线。
- 点击目录后滚动到标题并更新 URL hash；滚动正文时用 IntersectionObserver 更新活动标题。
- 文档 URL 必须可刷新、收藏、分享，并支持浏览器前进后退。
- 外部链接使用安全 `rel` 属性；图片自适应宽度并有失败占位。
- 表格和代码块在小屏幕横向滚动，不得撑破页面。

## 8. 管理端界面规范

管理端沿用阅读端的 Indigo/Gray 体系、顶栏高度、圆角、边框和焦点状态，不另建视觉语言。

```text
管理顶栏：站点名称 / 返回阅读端 / 保存状态 / 管理员菜单
管理工作区：左侧文件树 280px / 中间 Markdown 编辑器 / 右侧预览或属性面板
```

组件包括：`AdminHeader`、`AdminSidebar`、`ContentTree`、`EditorToolbar`、`MarkdownEditor`、`MarkdownPreview`、`DocumentMetaPanel`、`UploadDialog`、`MoveDocumentDialog`、`DeleteConfirmDialog`、`UnsavedChangesDialog`、`ToastRegion`、`LoginForm`、`FirstRunSetup`。

编辑器行为：

- `Ctrl+S` 保存；macOS 对应 `Cmd+S`。
- 未保存时显示状态，离开页面必须提示。
- 保存使用乐观并发控制；服务器版本改变时禁止静默覆盖。
- 第一版以明确的手动保存为主，自动保存后置。
- 预览请求防抖，使用与正式阅读端相同的后端渲染器。
- 图片上传后可以插入 Markdown 相对路径。
- 删除、覆盖、批量移动前必须二次确认。
- 编辑区、预览区和属性区宽度可调整，并记住本机偏好。

## 9. Markdown 内容契约

### 9.1 文件组织

```text
content/
├─ getting-started/
│  ├─ _category.yml
│  ├─ introduction.md
│  └─ installation.md
└─ guides/
   ├─ _category.yml
   └─ docker.md
```

目录决定导航层级。`_category.yml` 控制分类展示属性：

```yaml
title: 快速开始
order: 10
collapsed: false
```

Markdown Front Matter：

```yaml
---
title: Docker 安装
description: 使用 Docker 部署文档服务
order: 20
draft: false
tags:
  - Docker
  - 飞牛
---
```

| 字段 | 必填 | 默认行为 |
|---|---|---|
| `title` | 否 | 使用第一个 H1，再回退到文件名 |
| `description` | 否 | 阅读页副标题为空 |
| `order` | 否 | 同层按文件名自然排序 |
| `draft` | 否 | 默认为 `false` |
| `tags` | 否 | 默认为空数组 |
| `slug` | 否 | 默认由相对文件路径生成 |

系统扫描外部 Markdown 时不得擅自修改原文件。缺失字段仅在内存中推导；管理员主动保存时才写入用户确认过的 Front Matter。

### 9.2 支持范围

v0.1 必须支持：

- CommonMark 基础语法。
- GFM 表格、任务列表、删除线和自动链接。
- 标题锚点、有序与无序列表、引用和水平线。
- 图片、链接和围栏代码块。
- 中文文件名、标题和锚点。

v0.2 可以支持脚注、提示容器、Mermaid 和 KaTeX。原始 HTML 默认关闭。Mermaid 和 KaTeX 必须作为受控扩展，不能通过开放任意脚本实现。

## 10. 后端模块设计

### 10.1 Content Service

职责：

- 扫描 `.md` 和 `.markdown`。
- 读取分类元数据和 Front Matter。
- 生成稳定的树结构。
- 读取、新建、保存、移动和删除文档。
- 计算内容哈希，用于缓存和并发控制。
- 监听外部修改并触发局部重建索引。

写入规则：

- 接收路径后先标准化。
- 拒绝绝对路径、盘符、`..`、空字节和越界符号链接。
- 先写同目录临时文件，刷盘后再替换目标文件。
- Windows 和 Linux 分别测试覆盖写入行为。
- 修改前可以生成短期备份；正式修订历史后置。

### 10.2 Markdown Service

- 解析 Front Matter。
- 将 Markdown 转为安全 HTML。
- 提取标题目录和搜索纯文本。
- 生成代码高亮。
- 重写内部链接和媒体地址。
- 根据内容哈希缓存渲染结果。

前端不得自行建立另一套不一致的正式渲染规则。

### 10.3 Search Service

v0.1 使用服务端内存索引，不增加外部进程。建议权重：

| 字段 | 权重 |
|---|---:|
| 标题 | 8 |
| 标签 | 6 |
| 章节标题 | 4 |
| 描述 | 3 |
| 正文 | 1 |

搜索必须支持中文连续字符串、英文大小写归一化、结果片段和关键词高亮。性能上限以压力测试为准；达到瓶颈后再评估 SQLite FTS5 或 Bleve。

### 10.4 Auth Service

- 首次启动进入管理员初始化流程。
- 密码使用 Argon2id 哈希。
- 登录后使用随机服务器会话，不把敏感信息放入 JWT。
- Cookie 设置 `HttpOnly`、`SameSite=Lax`；HTTPS 下设置 `Secure`。
- 写接口要求 CSRF 令牌或等价同源防护。
- 登录失败限速，并记录不含密码的安全日志。
- 登出和密码变更后使旧会话失效。

### 10.5 Authorization Service

认证只回答“访问者是谁”，授权必须独立回答“他能对某个资源做什么”。所有接口通过统一入口调用：

```go
Can(subject, action, resource) Decision
Filter(subject, action, resources) []Resource
```

第一版的主体只有 `guest` 和 `admin`，但接口与存储必须允许后续增加 `user`、`role`、`group` 和分享令牌。管理员是不可删除的全权限主体；访客默认拒绝，只能读取明确公开的资源。

权限检查必须位于服务层，而不是只写在路由中。这样 HTTP API、搜索、静态导出、备份预览和未来 FPK 网关都会复用同一判定。

## 11. 数据库边界

SQLite 初始表：

- `schema_migrations`
- `users`
- `sessions`
- `settings`
- `content_nodes`
- `access_rules`
- `media_assets`
- `document_assets`
- `path_redirects`
- `audit_logs`

后续可增加 `document_revisions`、`api_tokens`、`backup_jobs`。

数据库迁移必须单向、带版本号，并在备份后执行。Markdown 正文不进入唯一性的数据库字段；数据库丢失后应能重新扫描文档，但所有重新发现的资源必须恢复为私有状态，由管理员重新确认公开范围。这是权限数据损坏时的 fail-closed 行为。

## 12. REST API 初稿

### 12.1 公共接口

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/v1/site` | 站点名称、Logo、版本和公开设置 |
| GET | `/api/v1/tree` | 仅返回当前访问者可列出的文档树 |
| GET | `/api/v1/docs/*path` | 仅返回可读文档的元数据、HTML、目录、前后页 |
| GET | `/api/v1/search?q=` | 只搜索和返回当前访问者可读的文档 |
| GET | `/media/*path` | 在附件权限检查后返回文件 |
| GET | `/healthz` | 存活检查 |
| GET | `/readyz` | 数据目录和数据库就绪检查 |

### 12.2 管理接口

| 方法 | 路径 | 用途 |
|---|---|---|
| POST | `/api/v1/auth/setup` | 首次创建管理员 |
| POST | `/api/v1/auth/login` | 登录 |
| POST | `/api/v1/auth/logout` | 登出 |
| GET | `/api/v1/admin/tree` | 包含草稿的完整树 |
| GET | `/api/v1/admin/docs/*path` | 获取 Markdown 原文和版本哈希 |
| POST | `/api/v1/admin/docs` | 新建文档 |
| PUT | `/api/v1/admin/docs/*path` | 保存文档，要求版本匹配 |
| PATCH | `/api/v1/admin/docs/*path` | 重命名、移动或更新属性 |
| DELETE | `/api/v1/admin/docs/*path` | 删除文档 |
| POST | `/api/v1/admin/preview` | 使用正式渲染器生成预览 |
| POST | `/api/v1/admin/import/markdown` | 上传 Markdown |
| POST | `/api/v1/admin/media` | 上传图片或附件 |
| GET | `/api/v1/admin/access/*resource` | 获取直接规则、继承来源和最终权限 |
| PUT | `/api/v1/admin/access/*resource` | 修改文档或分类可见性 |
| POST | `/api/v1/admin/access/preview` | 预览访客实际能看到的树和受影响资源 |
| GET/PUT | `/api/v1/admin/settings` | 读取或修改站点设置 |

错误统一返回：

```json
{
  "error": {
    "code": "DOCUMENT_CONFLICT",
    "message": "文档已被其他操作修改",
    "requestId": "...",
    "details": {}
  }
}
```

生产响应不得返回堆栈、磁盘绝对路径、SQL 或敏感配置。

## 13. 缓存和一致性

- 文档响应使用内容哈希生成 ETag。
- 前端对公开树和文档使用条件请求。
- 渲染缓存以“路径 + 内容哈希 + 渲染器版本”为键。
- 文件监听事件必须去抖。
- 管理保存后立即使对应文档、树和搜索缓存失效。
- 外部文件变化允许数秒内的最终一致。
- 已删除或转为草稿的文档不得因缓存继续公开。

## 14. 权限与可见性模型

### 14.1 目标行为

- 未登录访问者是 `guest`，只能访问管理员明确公开的范围。
- 管理员登录后是 `admin`，能查看所有分类、文档、附件、草稿、归档和权限状态。
- 新建、外部复制或重新扫描到的内容默认私有。
- 公开分类时可以批量公开其后代；后代可以单独设为私有。
- 可以单独公开私有分类中的某篇文档；此时访客树只显示到达该文档所需的祖先容器，不显示其私有兄弟节点。
- 草稿和归档是内容生命周期状态，不等同于权限。访客能读取的必要条件是“已发布且最终可见性为公开”。

### 14.2 主体、动作和资源

| 维度 | 第一版 | 后续扩展 |
|---|---|---|
| 主体 Subject | `guest`、`admin` | 用户、角色、用户组、分享令牌 |
| 资源 Resource | 站点、分类、文档、附件 | 版本、修订、备份、API 令牌 |
| 动作 Action | `list`、`read`、`download`、`create`、`edit`、`move`、`delete`、`publish`、`manage_access` | 评论、审核、导出、分享 |

所有授权决策使用资源的稳定 UUID，不直接把可变文件路径作为唯一权限主键。`content_nodes` 保存 UUID、类型、父节点、当前相对路径、内容哈希和发现状态；Markdown 仍是正文事实源，SQLite 是访问策略事实源。

管理员通过管理端移动文件时，在同一业务事务中更新路径映射和父节点。文件在系统外部被移动后，无法可靠确认其身份，因此作为新资源导入并默认私有；旧记录进入 missing/tombstone 状态等待管理员处理。

### 14.3 公开规则与继承

每个分类或文档对访客具有三个可见性选项：

| 模式 | 含义 |
|---|---|
| `inherit` | 继承最近祖先的显式规则 |
| `public` | 当前资源显式公开；分类规则默认作用于后代 |
| `private` | 当前资源显式私有；其后代仍可被逐项显式公开 |

站点根节点默认为 `private`。判定时从根到目标资源收集规则，距离目标最近的显式规则生效。这样同时支持“公开整个分类但隐藏其中一篇”和“仅公开私有分类中的特定文件”。

若私有分类中存在显式公开文档，访客目录可以显示祖先分类的标题和层级作为导航容器，但不得暴露分类正文、描述、私有子项数量或更新时间。管理端在保存此规则前必须提示“祖先分类名称将对访客可见”。

文档最终可公开的条件：

```text
subject == guest
AND lifecycle_status == published
AND effective_visibility == public
AND resource_exists == true
```

管理员不受访客规则限制，但仍受资源存在状态和写入安全检查限制。

### 14.4 目录树过滤

后端先判定权限，再构造返回树：

1. 移除访客不可读的文档。
2. 移除既不可公开读取、又没有公开后代的分类。
3. 对仅作为祖先容器保留的私有分类，只返回 `id`、`title`、`type`、`children` 和必要路由信息。
4. 重新计算访客视角的顺序、上一篇和下一篇。
5. 不向访客返回 `visibility`、内部路径、草稿数量或权限规则。

禁止由前端接收完整树后再隐藏私有节点。

### 14.5 搜索、附件和衍生内容

权限不仅保护文档正文，还必须保护所有衍生入口：

- **搜索**：先获得可读资源集合，再返回命中结果和摘要；不得先搜索全部内容再在前端过滤。
- **附件**：附件必须登记所属文档或独立 ACL；不得把 `/data/media` 作为无鉴权静态目录暴露。
- **上一篇/下一篇**：基于过滤后的树计算，不能出现私有标题。
- **面包屑**：仅显示公开祖先或最小导航容器信息。
- **重定向**：旧路径跳转前重新检查目标权限。
- **修订历史与备份**：第一版仅管理员可见。
- **站点地图、RSS 和静态导出**：只包含访客可读内容。
- **Open Graph 与页面摘要**：未授权资源不得生成带标题或摘要的响应。

附件表使用 `media_assets` 和 `document_assets` 建立归属关系。默认附件继承上传目标文档的读取权限；同一文件要跨公开、私有文档复用时，应创建明确的附件规则或独立副本，不能靠难猜 URL 保护。

### 14.6 HTTP 与缓存行为

- 未登录访问私有或不存在资源统一返回 `404`，避免通过 `403` 探测资源是否存在。
- 已登录管理员请求不存在资源返回 `404`；权限不足的未来普通用户可以返回 `403`。
- 公共与管理接口使用不同 URL 空间和缓存策略。
- 公共文档可以使用 `Cache-Control: public` 与权限版本参与的 ETag。
- 管理内容、草稿和权限预览使用 `Cache-Control: private, no-store`。
- 权限改变后必须同时失效树、搜索、正文、前后页、附件和站点地图缓存。
- 若使用反向代理，缓存键不得只使用路径而忽略认证状态。
- 第一版不使用 Service Worker 缓存受限正文，避免退出后仍可离线读取管理员内容。

### 14.7 管理端权限界面

文档和分类的操作菜单增加“访问权限”。权限面板必须展示：

- 当前直接规则：继承、公开或私有。
- 最终生效结果。
- 继承来源及可点击的祖先位置。
- 分类修改将影响的文档数量。
- 被后代显式规则覆盖的例外数量。
- 锁图标表示私有，地球图标表示公开，链状或层级图标表示继承。

批量公开分类前显示影响预览；从公开改为私有时提示已有公开链接将失效。提供“以访客身份预览”功能，调用服务端访客权限接口，而不是在管理员数据上做前端模拟。

阅读端不为访客显示私有节点占位。管理员阅读视图应显示全部节点，并用徽标区分 `公开`、`私有`、`继承`、`草稿` 和 `归档`。

### 14.8 数据表建议

```text
content_nodes
  id UUID PRIMARY KEY
  parent_id UUID NULL
  kind category|document
  relative_path UNIQUE
  lifecycle_status draft|published|archived
  content_hash
  missing_at NULL

access_rules
  id UUID PRIMARY KEY
  resource_id UUID
  subject_type guest|role|group|user|share_token
  subject_id NULLABLE
  action list|read|download|...
  effect allow|deny
  inheritance inherit|self|subtree
  created_by
  created_at
  updated_at
```

第一版 UI 的三态可见性映射为 `guest + read/list` 规则；底层通用结构保留未来用户和用户组能力。权限规则必须带唯一性约束和外键，删除资源时进行可审计的级联或软删除。

### 14.9 是否使用 Casbin

[Apache Casbin](https://github.com/apache/casbin) 支持 ACL、RBAC、ABAC、REST 路径、deny-override 和优先级规则，适合未来加入多用户、用户组和不同编辑权限。第一版不建议立即依赖 Casbin：当前只有 guest/admin，而目录继承、批量树过滤、搜索和附件归属仍需自行实现。

实施策略是先定义独立 `AuthorizationService` 接口与完备测试，规则存 SQLite；当多用户 ACL 进入里程碑时，再用压力测试和迁移原型决定是否将判定器替换为 Casbin。业务模块不得直接依赖具体策略库。

### 14.10 可借鉴的开源项目

| 项目 | 可借鉴设计 | 本项目取舍 |
|---|---|---|
| [Wiki.js 权限模型](https://docs.requarks.io/groups) | Guest、用户组、全局能力和基于路径的 Page Rules | 借鉴 Guest 与分类路径规则，不照搬复杂全局权限 |
| [BookStack 角色与内容权限](https://www.bookstackapp.com/docs/user/roles-and-permissions/) | 分类/章节/页面覆盖、父级级联、管理员永久保留全部权限 | 借鉴继承、直接覆盖和管理员兜底 |
| [Docmost 页面权限](https://docmost.com/docs/user-guide/pages/page-permissions) | 限制沿层级继承；侧栏与搜索同步隐藏无权页面 | 借鉴所有派生入口统一过滤；不依赖其商业功能代码 |
| [Outline](https://github.com/outline/outline) | Collection、Document、公开分享和子树权限 | 借鉴权限来源指示；避免复制式继承产生规则漂移 |
| [Apache Casbin](https://github.com/apache/casbin) | 可替换的 ACL/RBAC/ABAC 判定器 | 作为多用户阶段候选，不作为第一版前置依赖 |

BookStack 的安全记录还说明，附件元数据、草稿附件、修订和嵌入预览都可能绕过正文权限，因此本项目把“所有派生内容经过同一授权服务”列为发布阻断项。

## 15. 安全基线

发布前必须覆盖：

- Markdown XSS 和危险 URL。
- 路径穿越、Windows 盘符、UNC 路径和符号链接逃逸。
- 文件覆盖、同名上传和双扩展名。
- 上传、请求体和文档体积限制。
- Cookie、CSRF、CORS 和点击劫持策略。
- 暴力登录限速、MIME 嗅探控制和日志脱敏。
- IDOR（通过猜测资源 ID 或路径越权读取）和批量接口越权。
- 搜索、附件、重定向、修订、站点地图和缓存的权限旁路。
- 容器非 root 运行，程序目录只读，仅 `/data` 可写。

默认 CORS 仅同源，不开放任意目录映射，不允许 Markdown 原始 HTML；图片使用白名单，其他附件按下载处理。

## 16. 可访问性和国际化

- 页面根语言默认 `zh-CN`，允许从设置调整。
- 所有按钮必须有可感知名称。
- 导航、面包屑、目录和主内容使用正确语义区域。
- 抽屉和对话框打开时管理焦点，关闭后恢复焦点。
- 键盘可以访问搜索、目录、版本选择、复制和管理操作。
- Loading 使用 `aria-busy`；错误提示使用 `role="alert"`。
- 支持 200% 页面缩放，不出现核心功能遮挡。
- 界面文本集中管理，为后续中英文切换保留空间；第一版以中文为主。

## 17. Windows 本地开发方案

依赖 Git、Go 当前稳定版、Node.js 当前 LTS，以及仓库锁定的一种 JS 包管理器。进入容器阶段后使用 Docker Desktop。

开发模式为 Vite 和 Go 两个进程，Vite 将 `/api`、`/media` 代理到 Go。`scripts/dev.ps1` 负责依赖检查、准备本地数据并启动服务，但底层命令也必须可单独运行。

生产预览必须使用嵌入前端后的单个 Go 程序，避免只在 Vite 代理模式中测试。

## 18. 测试策略

### 18.1 Go 单元测试

- Front Matter 解析与默认值。
- 中文路径和 URL 编码。
- 路径越界拒绝。
- Markdown 渲染与 XSS 样例。
- 目录提取和重复标题锚点。
- 内部链接重写。
- 搜索评分和中文匹配。
- 文件哈希和并发冲突。
- SQLite 迁移。

### 18.2 前端与组件测试

- 文档树展开、折叠和活动项。
- 搜索的空、加载、无结果和错误状态。
- 抽屉键盘行为和焦点恢复。
- 代码复制成功与失败。
- 编辑器未保存提示和 API 错误映射。
- 设计令牌使用检查，禁止组件硬编码颜色。

### 18.3 API 集成测试

- 首次初始化、登录、登出和会话失效。
- 文档 CRUD 和合法、非法文件上传。
- 并发修改返回冲突。
- 草稿不出现在公共接口。
- 外部文件变化后索引刷新。
- 数据库迁移与失败恢复。
- 访客读取公开文档成功，读取私有、草稿或归档文档统一返回 404。
- 公开分类、私有后代、私有分类和公开后代的继承矩阵正确。
- 管理员能查看所有节点，访客树不含私有兄弟节点和敏感元数据。
- 搜索、附件、前后页、面包屑、重定向和站点地图不泄漏私有内容。
- 权限修改后所有相关缓存立即失效。
- 数据库权限记录丢失或重建时，内容默认私有而不是默认公开。

### 18.4 浏览器端到端测试

1. 初始化管理员并登录。
2. 新建分类与 Markdown。
3. 编辑并预览。
4. 上传图片并插入文档。
5. 发布后在阅读端出现。
6. 左侧导航、目录、搜索和前后页正常。
7. 刷新和浏览器后退保持正确页面。
8. 移动视口中通过抽屉完成导航。

### 18.5 容器测试

- 镜像以非 root 用户运行。
- `/healthz` 和 `/readyz` 正常。
- 挂载目录权限正确。
- 重启、删除并重建容器后数据保留。
- 只读程序文件系统下仍能运行。
- AMD64 和 ARM64 镜像均可拉取。

## 19. 代码质量与提交规范

每个 Pull Request 必须通过 Go 格式化、静态检查和测试，TypeScript 类型检查，前端 lint、单元测试和生产构建，API 契约检查，以及 Docker 构建检查。

建议提交格式：

```text
feat(reader): add document tree navigation
fix(markdown): reject unsafe image protocols
docs(plan): update fnOS release checklist
```

一个提交表达一个主要意图。格式化和大规模机械改动不得与业务改动混在同一提交中。

## 20. Docker 设计

### 20.1 多阶段镜像

```text
frontend-builder  构建 Vue
backend-builder   编译 Go 并嵌入前端
runtime           仅包含程序、CA 证书和非 root 用户
```

运行约定：

- 默认监听 `0.0.0.0:8080`。
- 数据根目录默认为 `/data`。
- 支持通过环境变量修改端口、日志级别、基础路径和数据目录。
- 容器内 UID/GID 应可配置或至少记录清楚。
- 提供绑定目录和命名卷两种说明。
- 健康检查调用 `/healthz`。
- 测试版本不得覆盖 `latest`。

最小 Compose 目标：

```yaml
services:
  docs:
    image: ghcr.io/OWNER/PROJECT:VERSION
    ports:
      - "8080:8080"
    volumes:
      - ./data:/data
    restart: unless-stopped
```

正式 Compose 还必须包含健康检查、安全选项和环境变量说明。

## 21. GitHub Actions 与发布

### 21.1 Pull Request

- 安装锁定依赖并运行前后端检查和测试。
- 构建生产前端、Go 程序和 Docker 镜像，但不推送。
- 上传必要测试报告，不上传密钥或运行时数据。

### 21.2 主分支

- 全部测试通过后构建开发镜像。
- 标签使用 `main-<短SHA>`，不覆盖稳定标签。

### 21.3 Release

推送 `vX.Y.Z` 标签时：

- 生成 `X.Y.Z`、`X.Y` 和稳定条件下的 `latest` 标签。
- 使用 Buildx 构建 `linux/amd64`、`linux/arm64`。
- 推送到 GHCR。
- 生成 SBOM、来源证明和镜像摘要。
- 创建 GitHub Release 和变更说明。
- 后续阶段附加对应 FPK 文件。

仓库公开前必须确定项目许可证、第三方组件许可证清单和样式原型的归属说明。原型 README 表明可个人和商业使用、署名非强制；仍建议在 `NOTICE` 或致谢页保留 StyleKit Templates 来源。

## 22. 飞牛 fnOS 路线

### 22.1 Docker 验证

先通过飞牛 Docker 管理器或 Compose 验证 GHCR 公共镜像拉取、端口访问、NAS 目录挂载、容器启停和自动重启、升级数据兼容，以及目标设备架构。

### 22.2 Docker 型 FPK

```text
packaging/fnos/
├─ app/
│  ├─ docker/docker-compose.yaml
│  └─ ui/
├─ cmd/
├─ config/
├─ wizard/
├─ manifest
├─ ICON.PNG
├─ ICON_256.PNG
└─ LICENSE
```

FPK 验收：

- 能安装并拉取正确架构镜像。
- 桌面入口能打开服务。
- 安装向导能设置端口和数据目录。
- 状态检查能反映容器实际状态。
- 升级不丢失 `/data`。
- 卸载默认不删除用户文档；如提供删除选项，必须明确说明并二次确认。

FPK 结构、`fnpack` 命令和网关能力以制作时的飞牛官方文档及目标 fnOS 版本为准，不把当前社区资料视为永久不变的接口。

### 22.3 原生 FPK

仅在 Docker 型 FPK 稳定且确有更低资源诉求时评估。Go 单二进制使原生 FPK 可行，但要额外处理 AMD64/ARM64 产物、生命周期脚本、数据权限、端口或 Unix Socket 网关、升级迁移和进程守护。

## 23. 里程碑与验收标准

### M0：工程基线

交付前后端骨架、Windows 开发脚本、设计令牌、API 错误模型、Authorization Service 接口、示例内容和基础 CI。

验收：新电脑按 README 可以启动；空数据和示例数据都能正常加载。

### M1：阅读端 v0.1

交付三栏布局、移动抽屉、经过访客权限过滤的文件树、面包屑、文章目录、Markdown/GFM、代码复制、表格、图片、真实路由、前后页和搜索。

验收：桌面和移动断点与样式原型达到结构及视觉一致；真实 Markdown 替代全部硬编码示例内容。

### M2：管理端 v0.2

交付管理员初始化、登录、完整文件树、文档 CRUD、上传、实时预览、草稿、排序、公开/私有/继承权限面板、访客预览和站点设置。

验收：完成“登录 → 新建 → 编辑 → 上传图片 → 发布并设为公开 → 访客查看”闭环；私有内容仅管理员可见，冲突和未保存离开不会静默丢数据。

### M3：可靠性与安全

交付原子写入、文件监听、权限感知缓存失效、安全防护、权限旁路集成测试、E2E 和备份恢复初版。

验收：安全样例不会执行脚本或越界读取；异常中断后原始文档可恢复。

### M4：Docker 与 GitHub 发布

交付 Dockerfile、Compose、健康检查、PR/Release 工作流、双架构 GHCR 镜像和运维文档。

验收：Windows Docker Desktop 和飞牛 Docker 均能运行；重建容器不丢数据。

### M5：飞牛 FPK

交付 Docker 型 FPK、安装向导、桌面入口、图标、打包流程和飞牛测试记录。

验收：目标 fnOS 设备完成安装、启动、更新和保留数据测试。

### M6：增强版本

候选功能包括修订历史、多用户角色、暗色主题、品牌定制、Mermaid、KaTeX、ZIP 导入导出、Git 备份、纯静态导出和原生 FPK。

## 24. 风险登记

| 风险 | 影响 | 应对 |
|---|---|---|
| Markdown HTML 导致 XSS | 高 | 默认关闭 HTML，建立恶意样例测试 |
| 文件 API 路径穿越 | 高 | 统一路径服务、真实路径边界校验、双平台测试 |
| 数据库与文件不一致 | 高 | Markdown 为事实源，数据库只存管理数据 |
| 前端隐藏但 API 仍返回私有数据 | 高 | 所有查询先经 Authorization Service，端到端越权测试 |
| 搜索、附件或缓存泄漏私有内容 | 高 | 派生入口统一授权，权限变更做全链路缓存失效 |
| 外部新增文件意外公开 | 高 | 新发现和身份不明资源默认私有，管理员确认后公开 |
| 祖先分类名称意外暴露 | 中 | 单篇公开时显示影响预览并提示最小导航元数据 |
| Windows/Linux 原子替换差异 | 高 | 抽象文件写入器并做平台集成测试 |
| 中文搜索质量不足 | 中 | 连续字符串和加权字段，可替换搜索接口 |
| 原型只有亮色和阅读页 | 中 | 先锁定亮色基线，再设计管理端和暗色令牌 |
| 组件继续硬编码样式 | 中 | 三层令牌、代码审查和自动扫描 |
| ARM64 构建成功但运行失败 | 高 | Release 前真实 ARM/飞牛设备冒烟测试 |
| fnOS/FPK 接口变化 | 中 | FPK 阶段重新核对官方文档并锁定系统版本 |
| GHCR 权限导致拉取失败 | 中 | 正式镜像设为公开并在无登录环境验证 |

## 25. 完成定义（Definition of Done）

一个功能只有同时满足以下条件才算完成：

- 行为符合本手册和对应验收标准。
- 正常、加载、空、错误、禁用等状态完整。
- 键盘和移动端可使用。
- 不引入组件内硬编码颜色。
- 新增 API 有错误处理和权限检查。
- 新增文件操作有路径边界和失败恢复。
- 单元或集成测试覆盖关键逻辑。
- 文档和示例同步更新。
- 本地生产构建通过。
- 不提交密钥、数据库、备份或用户内容。
- Code Review 没有未解释的高风险问题。

## 26. 默认产品决策

在没有额外产品输入前，按以下默认值推进：

- 阅读端允许匿名访问，但匿名访客只能看到管理员明确公开的内容；根目录与新资源默认私有。
- 管理端只有一个管理员，管理员拥有不可配置的全量查看和管理权限。
- 第一版只有一个站点和一个内容空间。
- 默认语言为简体中文。
- 默认单版本；界面保留版本选择能力，多版本数据模型后置。
- 文档路径是公开 URL 的基础。
- Markdown 和图片是第一等内容；其他文件只提供下载。
- 亮色主题是首个视觉验收基准。
- 数据通过单个 `/data` 目录备份和迁移。

若改变公开访问、用户数量、版本模型或内容权限，必须先更新本节和相关数据模型，再修改实现。

## 27. 推荐的首轮开发顺序

1. 建立仓库结构、配置加载、日志和健康检查。
2. 将样式原型转换为三层设计令牌。
3. 建立 Vue 阅读端布局和静态组件目录。
4. 实现内容扫描、Front Matter 和文件树 API。
5. 实现统一 Markdown 渲染与文章目录。
6. 用真实 API 接替前端所有样例数据。
7. 完成路由、搜索、移动抽屉和滚动目录。
8. 建立 SQLite 迁移和管理员初始化。
9. 实现 Authorization Service、访客树过滤和所有衍生入口权限检查。
10. 实现管理端编辑、预览、权限面板和安全写入。
11. 完成权限矩阵与测试基线后进入 Docker、GHCR 和飞牛部署。

首轮实现不得同时引入多用户、实时协作、复杂版本系统或原生 FPK，以免延迟最小可用闭环。

---

本手册应随架构决策和里程碑持续更新。涉及数据事实源、身份认证、公开 URL、Markdown 兼容性或部署方式的重大变化，必须先记录决策，再修改实现。
