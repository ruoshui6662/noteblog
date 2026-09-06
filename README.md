# Markdown 文档库

自托管、Markdown 文件原生的文档展示与管理系统。

当前完成 M0「工程基线」：Go 健康检查、数据目录初始化、Vue/Vite 前端壳和基于样式规范的设计令牌。

## Docker / GitHub / 飞牛测试

已提供多阶段 `Dockerfile`、GHCR 双架构构建工作流和飞牛 Compose。Go 在容器中直接提供嵌入的前端页面，数据保存到 `/data`。当前镜像只包含 M0 基线功能。

完整步骤见 [Docker、GitHub Actions 与飞牛测试指南](docs/DOCKER_GITHUB_FNOS.md)。将项目推送到 GitHub 的 `main` 或 `master` 后，工作流通过检查即发布 `ghcr.io/用户名/仓库名:edge`；飞牛使用 `deploy/compose.yaml` 拉取测试。

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
