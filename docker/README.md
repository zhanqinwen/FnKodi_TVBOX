# docker — fnkodi-tvbox 单镜像

- **Base：** `wjz304/kodi:latest`（Kodi 21 + `tini`）
- **Digest（开发机记录 2026-08-08）：** `sha256:6cbba5b1bd8e5cb1b7d5ba1d7b330042e972c04fc5fa16dcf77a08e7e752c63b`  
  生产构建建议改为 `FROM wjz304/kodi@sha256:...`
- **入口：** `tini` → `fnkodi-entrypoint.sh`（禁止 exec 掉自身）→ gateway 看门狗 + 上游 Kodi entrypoint
- **预装：** `plugin.video.fntvbox` + `skin.fntvbox`（首次 userdata 默认皮肤；标记文件 `.fnkodi-default-skin` 避免覆盖用户回退 Estuary）
- **禁止：** 映射宿主机 `18765`；安装 Android / redroid / JAR 运行时

一键构建（**Linux 构建机**）：

```bash
./scripts/build-all.sh
```

分步构建（需已有 `release/gateway` 与 `release/addons`）：

```bash
bash scripts/build-gateway.sh
bash scripts/package-addons.sh
bash scripts/build-image.sh
# → fnkodi-tvbox:<VERSION>
# → release/images/fnkodi-tvbox_<VERSION>_amd64.tar.gz
```

设备部署见 [`docs/deploy.md`](../docs/deploy.md)。

无 DRM 时做信号/看门狗验收可用：

```bash
docker run -d --name fn-sigtest -e APP_COMMAND='sleep infinity' fnkodi-tvbox:0.1.0
```
