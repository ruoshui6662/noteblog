# Markdown 文档库

自托管、Markdown 文件原生的文档展示与管理系统。

当前已完成 M0「工程基线」、M1「阅读端」和 M2「管理端」的首个闭环：服务扫描 `/data/content` 中的 Markdown，生成文档树，统一渲染安全 HTML，提供搜索、管理员认证、文档新建/编辑/删除，以及文件系统分类 `_category.yml` 的创建与维护。

## Docker / GitHub / 飞牛测试

已提供多阶段 `Dockerfile`、GHCR 双架构构建工作流和飞牛 Compose。Go 在容器中直接提供嵌入的前端页面，数据保存到 `/data`；Compose 默认将项目目录下的 `data/` 绑定到 `/data`，因此可以直接在 Docker/FnOS 文件管理器中看到 Markdown 和分类文件。当前镜像包含阅读、搜索、管理员初始化、登录、文档新建/编辑/删除和分类设置，媒体上传仍在开发。

完整步骤见 [Docker、GitHub Actions 与飞牛测试指南](docs/DOCKER_GITHUB_FNOS.md)、[Docker/飞牛开发经验记录](docs/DEVELOPMENT_EXPERIENCE.md)、[M1 阅读端进度](docs/progress/PHASE_01_READER.md) 和 [M2 身份进度](docs/progress/PHASE_02_ADMIN.md)。将项目推送到 GitHub 的 `main` 或 `master` 后，工作流通过检查即发布 `ghcr.io/用户名/仓库名:edge`；飞牛使用 `deploy/compose.yaml` 拉取测试。

## Windows 本地启动

在两个 PowerShell 窗口中分别执行：

```powershell
go run ./cmd/server
```

```powershell
Set-Location web
npm install
npm run dev
```

打开 `http://127.0.0.1:5173`。Vite 会把 `/api` 请求代理至 Go 服务的 `http://127.0.0.1:8080`。

### Windows 应用控制策略

当前开发机如果启用了应用控制策略，必须允许 Go 工具链的 `compile.exe`、`gofmt.exe`，以及 Vite 所依赖的 Rollup/Esbuild Windows 原生模块执行。否则 `go run` 和 `npm run dev`/`npm run build` 会被系统策略拦截；这是主机环境配置问题，不是项目依赖或源码错误。解除限制后按本节命令重新启动即可。

## 健康检查

```powershell
Invoke-RestMethod http://127.0.0.1:8080/healthz
Invoke-RestMethod http://127.0.0.1:8080/readyz
```

详见 [开发计划手册](docs/DEVELOPMENT_PLAN.md) 与阶段进度文档。
