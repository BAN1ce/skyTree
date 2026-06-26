# SkyTree Config Matrix

## 配置角色

| 文件 | 用途 | 特点 |
| --- | --- | --- |
| `etc/config.yaml` | 默认单机开发配置 | 单机、Badger、本地快速启动 |
| `etc/config.example.yaml` | 全量字段示例 | 覆盖所有主要配置项 |
| `etc/examples/config.beta.example.yaml` | beta 推荐模板 | 安全默认值、Scylla、TLS、禁用明文 gRPC |
| `etc/examples/config.cluster.example.yaml` | 集群示例 | 开发/测试集群参考，不是 beta 安全基线 |
| `etc/examples/config.stress.example.yaml` | 压测参考 | 容量与压测场景导向 |
| `etc/fixtures/*.yaml` | 历史夹具 | 用于测试/兼容验证，不作为默认入口 |

## Profile 规则

| Profile | 规则 |
| --- | --- |
| `default` | 基础合法性校验 |
| `beta` | 额外禁止 `cluster.grpc.allow_insecure=true` |

## K8s 目录映射

| 目录 | 用途 |
| --- | --- |
| `deploy/k8s/local` | 本地 smoke / 开发验收 |
| `deploy/k8s/beta` | beta 标准模板 |
| `deploy/k8s/stress` | 压测模板 |
