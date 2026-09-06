# 容器测试准备进度

日期：2026-09-06。

用户要求提前准备 Docker、GitHub Actions 镜像构建与飞牛拉取测试；本次范围为 M0 基线的容器化，不表示 M1–M4 全部完成。

## 已实现

- Go 生产构建嵌入 Vue 静态资源，普通构建保留 Vite 开发模式。
- 修复静态页面和 API 兜底路由的注册冲突，补充路由与 readiness 测试。
- 多阶段 Dockerfile，非 root UID/GID 10001、8080 端口、/data 数据目录、内置健康检查。
- GHCR 工作流：先检查与 AMD64 容器冒烟，再构建并发布 AMD64/ARM64 镜像；带 SBOM 和 provenance。
- edge、完整 SHA 与语义版本标签；测试阶段不生成 latest。
- 飞牛 Compose、环境变量示例、权限/持久化/升级与拉取指南。

## 本机验证

- Vue/TypeScript 类型检查：通过。
- Vite 生产构建：通过。
- Go 格式化、普通与 production 标签测试、production go vet：通过。
- Windows 生产二进制及 Linux AMD64/ARM64 交叉编译：通过。
- Windows 生产程序 HTTP 冒烟：首页、构建后 JS、健康检查、站点 API、SPA 文档路径通过；未知 API 和缺失资源为 404。
- 二进制 healthcheck 子命令：通过。
- 工作流与 Compose YAML 语法解析：通过；不等于 GitHub Actions 或 Docker Compose 实际运行通过。

此前进度记录中的 Windows 构建阻塞属于历史记录。本次使用临时下载的 Go 1.26.5 和现有 Node 运行时完成验证，未修改系统应用控制策略。

## 外部待验证

- 本机未安装 Docker，尚未执行 Docker 镜像构建或容器运行。
- 尚未提供 GitHub 仓库地址，未上传源码或发布 GHCR 镜像。
- Actions 执行后应确认容器健康、非 root、只读根目录及重建数据保留测试通过。
- 飞牛设备实机拉取、端口、挂载、更新和 ARM64 运行验收待执行。

下一步按 [部署指南](../DOCKER_GITHUB_FNOS.md) 上传仓库、观察 Actions，再在飞牛导入 Compose。
