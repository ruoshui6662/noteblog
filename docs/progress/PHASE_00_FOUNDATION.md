# 阶段进度：M0 工程基线

> 状态：实现完成；本机运行验证受应用控制策略阻塞  
> 开始日期：2026-09-06  
> 对应手册：[开发计划手册](../DEVELOPMENT_PLAN.md)

> 后续更新：本次容器准备已完成本机前后端构建与生产程序冒烟，详见 [容器测试准备进度](DOCKER_PREPARATION.md)。下方阻塞表保留为首次验证的历史记录。

## 本阶段目标

从最小可验证系统出发，建立能够在 Windows 本地运行的前后端底座，不提前实现文档 CRUD、管理员登录或 Docker。

## 已实现

- Go 服务入口：`cmd/server/main.go`。
- `GET /healthz`、`GET /readyz`、`GET /api/v1/site`。
- 本地运行数据目录：`var/data/content`、`var/data/media`、`var/data/backups`。
- Vue 3 + Vite 前端壳及 Go API 代理。
- 设计令牌三层目录：原始值、语义值、组件值。
- 与样式基准一致的 56px 顶栏、280px 桌面侧栏、768px 正文阅读宽度和超宽屏目录栏骨架。
- 可访问性基线：语义区域、可见焦点、`aria-live` 服务状态、缩减动画偏好。

## 本阶段不包含

- Markdown 扫描、渲染和真实文档树。
- 权限数据库、管理员认证和访问控制。
- Docker、GitHub Actions、飞牛部署。

## 验证记录

| 检查 | 结果 | 说明 |
|---|---|---|
| `npm install` | 通过 | 已生成 `web/package-lock.json` 与本地依赖。 |
| `npm exec vue-tsc -- --noEmit` | 通过 | Vue 组件和 TypeScript 类型检查通过。 |
| `npm run build` | 受阻 | Windows 应用控制策略阻止 `@rollup/rollup-win32-x64-msvc` 原生模块加载。 |
| `gofmt -w cmd/server/main.go` | 受阻 | Windows 应用控制策略阻止 `C:\Program Files\Go\bin\gofmt.exe`。 |
| `go test ./...` / `go build ./cmd/server` | 受阻 | Windows 应用控制策略阻止 Go 工具链 `compile.exe`。 |
| 浏览器与 API 代理冒烟测试 | 未执行 | 依赖 Go 与 Vite 开发服务器，均被同一策略阻止。 |

已确认的环境事实：`go version`、`node --version` 和 `npm --version` 可正常执行；阻塞只发生在后续编译器或原生模块进程启动时。

## 解除阻塞后的复测顺序

1. 允许 Go 的 `gofmt.exe` 与 `pkg/tool/windows_amd64/compile.exe` 执行。
2. 允许工作区 `web/node_modules` 下 Rollup 与 Esbuild 的 Windows 原生模块执行。
3. 在项目根目录运行 `go test ./...`、`go build ./cmd/server`。
4. 在 `web` 目录运行 `npm run build`。
5. 分别启动 `go run ./cmd/server` 与 `npm run dev`。
6. 在浏览器访问 `http://127.0.0.1:5173`，检查服务状态显示为“服务已就绪：local 环境”。

未完成以上复测前，M0 不应标记为运行验收通过，也不进入 M1 的功能开发。

## 下一阶段

M1 阅读端：内容扫描、Front Matter、访客权限过滤、Goldmark 渲染、真实三栏文档树、目录与基础搜索。
