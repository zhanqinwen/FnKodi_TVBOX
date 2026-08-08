# fn-tvbox-gateway

Go 网关：TVBox 订阅、CMS/T4、直播、解析与播放代理。热路径在此进程，仅监听 `127.0.0.1:18765`。

## 本地开发

```bash
cd plugins/fn-tvbox-gateway
go test ./...
go run ./cmd/gateway
# 另开终端
curl -sS http://127.0.0.1:18765/health
curl -sS http://127.0.0.1:18765/api/subscription
curl -sS http://127.0.0.1:18765/api/sources
```

常用环境变量见 `docs/api-contract.md` §0.1（如 `FNTVBOX_SUBSCRIPTION_URL`）。

## 构建产物

在 Linux 构建机执行仓库根目录：

```bash
./scripts/build-gateway.sh
```

产物：`release/gateway/fn-tvbox-gateway`（`CGO_ENABLED=0`，linux/amd64）。

契约见 [`docs/api-contract.md`](../../docs/api-contract.md)。
