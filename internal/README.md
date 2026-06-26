# inner 目录导览

## 定位
- `inner` 是 broker 内部实现层，承载运行时行为与适配器。
- `inner` 可以依赖 `pkg`，但 `pkg` 不应依赖 `inner`。

## 目录职责
- `broker/`：MQTT broker 领域实现（连接、订阅、投递、共享订阅、will delay、ACL）。
- `cluster/`：集群运行时能力（当前健康检查位于 `cluster/health`）。
- `event/`：预留事件相关扩展目录（不是通用事件总线）。
- `grpc/`：节点间 gRPC 服务实现（投递通知、远端关闭连接）。
- `localstate/`：单机持久化运行时设施（WAL + snapshot 引擎）。
- `facade/`：跨模块流程型门面（应保持轻量）。

## 分层约束
- `inner` 内部按三类职责组织：
  - `domain`：业务规则与状态转换；
  - `application`：用例编排；
  - `adapters`：存储、RPC、事件总线等技术实现。
- `orchestrator` 目录只做流程编排，不放状态机与持久化细节。
- 业务代码优先沉淀到 `logic/*service`，`inner` 侧保留可复用能力与适配器。

## 事件边界
- `pkg/eventbus`：通用事件总线实现（基础设施）。
- `internal/broker/delivery/event`：broker 投递通知事件模型（单一模型源）。
- `internal/broker/delivery/notify`：broker 投递通知桥接实现（本地 EventCenter + 跨节点 gRPC）。

## 阅读建议
1. 先看 `broker/core` 与 `broker/core/client` 建立主链路。
2. 再看 `delivery`、`sub_center`、`session_center` 理解状态读写与投递。
3. 最后看 `shared_subscription`、`will_delay`、`cluster` 理解分布式增强能力。
