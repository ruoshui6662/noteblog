# Docker → GitHub Actions → 飞牛测试

当前镜像包含阅读端、搜索、管理员认证、文档管理和文件系统分类设置。Markdown 与 `_category.yml` 保存在 `/data/content`，Compose 默认将项目目录下的 `data/` 绑定到该路径，因此容器重建后文件仍可直接查看和备份。

## 1. 上传到 GitHub

新建空 GitHub 仓库，建议使用简单英文仓库名，例如 `markdown-docs`。本目录尚未初始化 Git。将下面地址替换成自己的仓库地址，在项目根目录运行：

```powershell
git init -b main
git add .
git diff --cached --stat
git commit -m "build: prepare Docker image and GHCR workflow"
git remote add origin https://github.com/YOUR_NAME/YOUR_REPOSITORY.git
git push -u origin main
```

如果上传网页文件，必须包含隐藏目录 `.github/workflows/`、`.dockerignore` 和 `web/package-lock.json`；不要上传 `web/node_modules`、`web/dist`、`var` 或本地 `.env`。推荐 Git 推送，避免漏掉隐藏文件。

仓库 Actions 页面查看 `Build and publish image`。工作流使用内置 `GITHUB_TOKEN`，不需要手动添加个人访问令牌；仓库或组织策略必须允许 Actions 写入 Packages。

工作流行为：

| 触发 | 行为 |
|---|---|
| PR 到 main/master | 构建 AMD64，运行 Go 检查、前端构建、容器冒烟和数据保留测试，不推送 |
| main/master 推送 | 检查成功后发布双架构 `edge` 和 `sha-完整提交SHA` |
| `v0.0.1-test.1` 等预发布标签 | 发布对应预发布版本和 SHA 标签 |
| `v0.1.0` 等正式版本标签 | 发布 `0.1.0`、`0.1` 和 SHA 标签 |
| 手动 Run workflow | 检查后发布所选分支/标签；仅 main/master 更新 edge |

当前测试阶段不生成 `latest`。发布成功后，Actions 运行摘要给出实际镜像标签和 digest，可直接复制。镜像名自动转换为小写：`ghcr.io/用户名/仓库名:edge`。生产构建在 Linux 完成，不依赖 Windows 本机 Go/Docker。

## 2. 让飞牛可以拉取

首次发布后进入 GitHub 用户或组织的 Packages，打开该镜像的 Package settings。若要匿名拉取，将包的可见性设置为 Public；源码仓库公开不代表镜像包自动公开。

若保持包私有，在飞牛的镜像仓库凭据或终端配置 GHCR 登录，使用有 `read:packages` 权限的 classic PAT，并确保账户有包访问权限：

```sh
docker login ghcr.io -u YOUR_NAME
```

在密码提示中输入令牌，不要把令牌写入 Compose 或提交到仓库。

## 3. 飞牛 Compose 安装

将 `deploy/compose.yaml` 放入飞牛上的项目目录。Compose 默认使用项目目录下的 `data/` 绑定目录，并将其映射到容器 `/data`；也可以通过 `DOCS_DATA_DIR` 指定 NAS 上的绝对路径。镜像地址和数据目录都支持环境变量覆盖：

```yaml
environment:
  DOCS_IMAGE: ghcr.io/ruoshui6662/noteblog:edge
  DOCS_DATA_DIR: /vol1/1000/docker/noteblog-data
```

在飞牛 Docker 项目管理中导入该 Compose 即可。

Compose 明确设置了 `entrypoint` 并清空 `command`，确保容器主进程运行 HTTP 服务。`healthcheck` 只由 Docker 在后台探测，不能把 `healthcheck` 填到项目的“启动命令”字段中。

也可在该目录执行：

```sh
docker compose config
docker compose pull
docker compose up -d
docker compose ps
docker compose logs --tail=100
```

访问 `http://飞牛IP:8080`，应显示文档首页和“container”环境状态。

绑定目录中的结构就是应用的真实数据结构：`data/content/` 保存 Markdown 文件和 `_category.yml`，`data/media/` 保存附件，`data/noteblog.db` 保存管理员认证数据。镜像以 UID/GID `10001:10001` 运行，根文件系统只读，只有 `/data` 可写。首次使用前，在飞牛上创建 `DOCS_DATA_DIR` 指定的目录，并给予 UID/GID `10001:10001` 写权限；只修改这个专用目录的归属，不要修改整个 NAS 共享目录。

## 4. 验收与升级

1. `docker compose ps` 显示 healthy。
2. 首页加载成功，无资源 404；服务状态为 container。
3. `/healthz` 返回 ok，`/readyz` 返回 ready，`/api/v1/site` 返回 M2。
4. 在绑定目录的 `content/` 下写入临时 Markdown 文件，执行 `docker compose down` 和 `docker compose up -d` 后文件仍存在。
5. 查看日志无 permission denied；AMD64 和 ARM64 设备各自需要真实运行验证，工作流运行测试当前只覆盖 AMD64。

更新测试镜像：

```sh
docker compose pull
docker compose up -d
```

更新前备份完整 `DOCS_DATA_DIR` 目录。生产环境如需固定版本，可将 Compose 中的 `DOCS_IMAGE` 改为 Actions 提供的 SHA 标签或 digest；日常更新使用 `edge` 即可。

如果项目管理器显示 `Exited (0)`，先检查实际启动配置和日志：

```sh
docker inspect noteblog --format '{{json .Config.Entrypoint}} {{json .Config.Cmd}}'
docker logs noteblog
```

正常结果应为 `Entrypoint=["/usr/local/bin/markdown-docs"]` 且 `Cmd=[]`；若 `Cmd` 中出现 `healthcheck`，删除项目中自定义的启动命令后重新创建容器。

## 5. 本机有 Docker 时

```powershell
docker build -t markdown-docs:local .
```

然后在 deploy 目录临时将 Compose 中的 `image` 改为 `markdown-docs:local`，执行 `docker compose up -d`。Linux/WSL/Git Bash 可运行 `bash scripts/smoke-docker.sh markdown-docs:local`，脚本只创建并清理自身的临时测试容器与卷。

生产 Go 构建需要先在 web 目录运行 `npm ci` 和 `npm run build`，再在根目录运行 `go build -buildvcs=false -tags production -o bin/markdown-docs ./cmd/server`。普通 `go run ./cmd/server` 保留 Vite 开发模式。

参考：[Docker 多架构工作流](https://docs.docker.com/build/ci/github-actions/multi-platform/)、[GitHub 镜像发布](https://docs.github.com/en/actions/tutorials/publish-packages/publish-docker-images)、[GHCR 访问权限](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)。
