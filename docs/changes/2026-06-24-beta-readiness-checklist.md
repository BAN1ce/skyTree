# SkyTree Beta Readiness Checklist

## 目标

把 SkyTree 收敛到“代码和仓库产物已经为 beta 上线做好准备”的状态。

## 当前基线

- `cmd/main.go` 只保留启动编排与退出路径。
- 配置校验支持 beta profile。
- `deploy/k8s` 已划分 `local`、`beta`、`stress`。
- 健康探针已拆分为 `/health/liveness`、`/health/readiness`、`/health/startup`。

## 发布前必做

- 运行 `make test-configs`
- 运行 `make test-beta-assets`
- 运行 `make test-short`
- 运行 `make test-race-core`
- 运行 `make test-scylla-integration`
- 运行 `make test-distributed`

## Beta 代码准入标准

- 不允许 beta profile 使用 `cluster.grpc.allow_insecure=true`
- 不允许集群配置缺少 `cluster.data_dir`
- 启用 mTLS 的 TLS block 必须带 `ca_file`
- Console 默认关闭，启用时必须显式提供凭据
- beta K8s 模板必须保留 TLS/ACL Secret 挂载位
