# 开发经验记录

本文件记录已经在真实部署环境验证过的故障、根因和优先解决方案。遇到同类问题时，先按对应条目排查，再考虑修改代码或镜像。

## Docker / 飞牛：容器反复重启，`/data` 无法写入

### 现象

- 容器创建和启动后立即进入 `restarting`，状态可能显示 `Exited (1)`。
- 日志出现：

  ```text
  failed to prepare data directory
  mkdir /data/content: permission denied
  ```

- 飞牛文件管理器中看不到新增的 Markdown 文件或目录。

### 第一性原理判断

`/data` 是宿主机绑定目录。容器内进程对 `/data` 的读写，最终由宿主机文件系统对该进程的 UID/GID 和 ACL 决定。镜像层只能设置镜像内部目录的默认权限，不能可靠地替宿主机绑定挂载目录执行 `chown`。因此，这类问题优先判断为“绑定目录权限或挂载配置问题”，不是先改应用保存逻辑。

当前镜像以非 root 用户运行：

```text
UID/GID: 10001:10001
```

### 优先解决方案

1. Compose 必须使用宿主机绝对路径绑定到 `/data`，不要使用命名卷：

   ```yaml
   volumes:
     - /vol1/1000/docker/noteblog:/data
   ```

2. 在飞牛宿主机上准备目录并授予镜像 UID/GID：

   ```sh
   sudo mkdir -p /vol1/1000/docker/noteblog/content
   sudo mkdir -p /vol1/1000/docker/noteblog/media
   sudo mkdir -p /vol1/1000/docker/noteblog/backups
   sudo chown -R 10001:10001 /vol1/1000/docker/noteblog
   sudo chmod -R u+rwX /vol1/1000/docker/noteblog
   ```

3. 先用一次性容器验证绑定目录，而不是直接反复重启业务容器：

   ```sh
   sudo docker run --rm \
     --user 10001:10001 \
     --entrypoint /bin/sh \
     -v /vol1/1000/docker/noteblog:/data \
     ghcr.io/ruoshui6662/noteblog:edge \
     -c 'mkdir -p /data/content && touch /data/.write-test && rm /data/.write-test && echo write-ok'
   ```

   输出 `write-ok` 后，才继续重建业务容器：

   ```sh
   sudo docker rm -f noteblog 2>/dev/null || true
   sudo docker compose pull
   sudo docker compose up -d --force-recreate
   sudo docker logs --tail=100 noteblog
   ```

4. 如果 `chown` 已成功但测试仍失败，再检查飞牛 ACL 或 Docker 用户命名空间；不要立即修改应用代码。必要时使用飞牛 ACL 为 UID `10001` 添加目录及子项的读写执行权限。

### Compose 配置检查

业务容器必须让 HTTP 服务作为主进程运行：

```yaml
entrypoint:
  - /usr/local/bin/markdown-docs
command: []
```

不要把 `healthcheck` 当作容器启动命令，也不要继续使用旧的 `/usr/local/bin/docker-entrypoint`。修改 Compose 后必须删除并重新创建旧容器，否则旧的入口命令、命名卷或挂载配置可能继续生效。

可用以下命令确认实际配置：

```sh
sudo docker inspect noteblog \
  --format 'user={{.Config.User}} entrypoint={{json .Config.Entrypoint}} cmd={{json .Config.Cmd}} mounts={{json .Mounts}}'
```

期望值：用户为 `10001:10001`，入口为 `/usr/local/bin/markdown-docs`，数据挂载源为 `/vol1/1000/docker/noteblog`，命令为空。

### 已验证结论

本次飞牛环境中，目录最终显示为 `10001:10001` 且模式为 `drwx------`；使用 UID `10001` 的一次性 Docker 容器可以成功创建 `/data/content` 和临时文件，随后业务容器正常启动。该验证顺序应作为后续同类问题的首选方案。

### 避免的处理方式

- 不要用 `docs-data:/data` 代替宿主机绑定目录，否则文件会进入 Docker 命名卷，飞牛文件管理器中看不到预期路径。
- 不要把 `healthcheck` 填入飞牛项目的“启动命令”。
- 不要只在镜像构建阶段 `chown /data` 并假定它能改变宿主机绑定目录。
- 不要一开始把整个应用改成 root；先验证宿主机目录是否能被 UID `10001` 写入。
