# K8s Deploy Layout

`deploy/k8s` 是 SkyTree 当前唯一的 canonical K8s 部署目录。

- `local/`: 本地 smoke / 开发验收环境，允许为了速度做简化，但仍跟随 beta 产物结构。
- `beta/`: 面向 beta 上线准备的云无关模板，占位 Secret、TLS、探针和 StatefulSet 形态。
- `stress/`: 大规模压测与容量验证模板。

旧的 `k8s/` 目录保留用于兼容历史脚本和参考，不再作为新增部署改动的首选来源。
