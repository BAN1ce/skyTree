---
topic: Distributed MQTT5 production readiness plan
audience: developer,architect,sre
doc_type: explanation,how-to
dependencies:
  - Go 1.24
  - Dragonboat
  - gRPC
  - ScyllaDB
  - Badger
keywords:
  - mqtt5
  - distributed-broker
  - production-readiness
  - reliability
  - roadmap
---

# SkyTree 分布式 MQTT5 Broker 生产化评估与开发计划

日期: 2026-05-22  
范围: 功能完善度、可用性、可靠性、可维护性、缺陷清单和后续开发计划  
状态: Phase 0 beta blockers implemented

<!-- maintained-by: human+ai -->
<!-- ai-generated-start -->

## 1. 结论总览

SkyTree 当前已经具备“可运行的分布式 MQTT5 broker 核心骨架”：协议处理、会话中心、订阅中心、遗嘱延迟、保留消息、共享订阅、投递队列、HTTP API、节点间 gRPC、Dragonboat-backed 状态中心和本地 WAL 模式都已经有实现与测试基础。

但项目还没有上线，生产化判断应保守：当前更接近“功能内核和工程骨架已成型”，还未达到“可承诺生产 SLA”的状态。上线前最关键的工作不是继续堆功能，而是补齐分布式故障验证、可观测性、运维闭环、存储一致性和容量边界。

优先级最高的结论：

1. **功能完善度**: MQTT5 核心能力覆盖较广，但管理面、集群成员变更、跨节点运维工具、Scylla backlog 可视化仍不足。
2. **可用性**: 配置校验和 runbook 已有基础，但本地多节点启动、压测、故障演练、API 使用说明还不够一键化。
3. **可靠性**: 健康检查写探活和 WAL 坏日志持久化两个旧高风险点已有代码缓解；下一步必须用三节点故障注入证明行为正确。
4. **可维护性**: `app/` 组合层、`internal/` 业务层、`pkg/` 契约层边界基本清楚，但仍存在长函数、事件载荷使用 `map[string]interface{}`、分布式语义指标不足等维护风险。

## 2. Discovery 摘要

本次评估基于以下已核对入口和模块：

| 领域 | 代表路径 | 观察 |
| --- | --- | --- |
| 进程入口 | `cmd/main.go` | 加载配置、创建 `app.App`、启动组件、处理退出信号。 |
| 应用组合 | `app/app.go` | 构建 infra、state centers、broker、API、gRPC、health checker。 |
| MQTT 数据面 | `internal/broker/core` | 处理 CONNECT、PUBLISH、SUBSCRIBE、AUTH、ACK、保留消息、遗嘱等协议行为。 |
| 投递链路 | `internal/broker/delivery` | 负责路由、任务追加、cursor-based delivery。 |
| 共享订阅 | `internal/broker/sharedsubscription` | 管理共享组 consumer、leader renewal、任务回滚和超时扫描。 |
| 状态中心 | `internal/broker/sessioncenter`、`internal/broker/subcenter`、`internal/broker/willdelay` | 支持 memory、local WAL、raft-backed 形态。 |
| 集群 | `pkg/cluster/raft`、`internal/cluster` | Dragonboat 封装、cluster registry、健康检查。 |
| 运维面 | `api` | `/health`、`/metrics`、`/debug/pprof/*filepath`、ACL API、cluster API。 |
| 配置 | `config`、`configs` | 配置加载、环境变量展开、TLS/driver/cluster 校验。 |
| 测试与脚本 | `Makefile`、`scripts`、`docs/07-testing.md` | 已有短测、race core、配置校验和若干压测脚本。 |

## 3. 功能完善度评估

### 3.1 已具备的核心能力

MQTT5 协议能力已经覆盖较多核心场景：

- CONNECT 协议校验、服务端分配 ClientID、Session Expiry、Clean Start、Server Keep Alive、Receive Maximum、Maximum Packet Size。
- PUBLISH 的 Topic Alias、Payload Format Indicator、QoS0/1/2、No Matching Subscribers、Retain Available、Maximum QoS。
- SUBSCRIBE 的共享订阅、Subscription Identifier、No Local、Retain As Published、共享订阅能力开关。
- QoS2 接收状态、重复 PUBLISH 处理、PUBREL negative reason 处理、QoS1 重发逻辑。
- Retained message 存储与过期清理。
- Will Delay task 和 owner token 防旧 owner 污染。
- Enhanced AUTH 插件钩子和 mTLS 证书上下文传递。

分布式能力已有以下基础：

- session/subscription/will-delay/key-store 可走 Raft 状态机。
- client delivery 支持 queue/payload 分离，cluster 模式要求使用 Scylla pair。
- 节点间 gRPC 支持 client delivery notify 和 close client。
- session owner token 和 sub-center owner token 可降低重连、抢占、远程关闭乱序风险。
- shared subscription consumer 具备 leader election、pending/processing 状态转换、超时回滚和客户端 offline 回滚。

### 3.2 主要功能缺口

| 优先级 | 缺口 | 影响 | 建议 |
| --- | --- | --- | --- |
| P0 | 缺少三节点端到端分布式验收套件 | 无法证明 leader 切换、节点宕机、网络抖动下消息语义可靠 | 已建立 K8s 默认 `make test-distributed` beta 验收入口。 |
| P1 | 集群成员变更能力不完整 | 上线后扩缩容和替换节点风险高 | 明确是否支持动态 membership；若不支持，文档声明需要滚动重建。 |
| P1 | Scylla backlog summary 尚未实现 | cluster overview 在生产存储模式下无法观察积压 | 为 Scylla queue store 实现 `DeliveryBacklogSummary`。 |
| P1 | HTTP 管理面偏少 | 运维无法从 API 完成 drain、pause、节点状态诊断 | 增加 broker runtime、client/session、delivery backlog、shared group 状态 API。 |
| P2 | 认证与 ACL 是插件式基础能力，缺少生产策略模板 | 接入方需要自行摸索安全配置 | 提供 mTLS、用户名密码、ACL rule、enhanced auth 示例。 |
| P2 | MQTT5 compliance 没有系统化矩阵 | 已测行为较多，但难声明兼容等级 | 建立 MQTT5 feature matrix 和 spec case mapping。 |

## 4. 可用性评估

### 4.1 正向信号

- `config.Validate` 已经对 broker listener、TLS、cluster member、gRPC TLS、delivery queue/payload driver 做启动前校验。
- `ResolveClientDeliverySpec` 明确禁止 cluster 模式使用 single-node Badger delivery queue，避免误用单机存储上线。
- `docs/06-runbook.md` 已提供本地启动、测试命令、健康接口和 TLS checklist。
- `api/api.http` 提供基础 API 调用样例。
- `scripts/start-k8s-console.sh`、`scripts/mqtt_functional_test.sh`、`scripts/distributed_stress_test.sh` 提供 K8s 默认运维/验证脚本基础。

### 4.2 可用性不足

| 优先级 | 问题 | 影响 | 计划 |
| --- | --- | --- | --- |
| P1 | 多节点本地启动和故障演练不够标准化 | 新开发者和 SRE 难复现分布式问题 | 已固化 `make k8s-up`、`make k8s-status`、`make k8s-down`、`make test-distributed`。 |
| P1 | cluster overview 在 Scylla 模式缺少 backlog 数据 | 生产模式下看不到核心投递积压 | 实现 Scylla backlog summary 并暴露到 `/api/v1/cluster/overview`。 |
| P1 | 缺少上线前配置模板 | 容易遗漏 TLS、storage、health、metric 等关键项 | 增加 `etc/config.production.example.yaml`。 |
| P2 | API 文档只覆盖基础示例 | 管理面使用成本高 | 补全 `docs/04-data-and-api.md` 的请求/响应示例。 |
| P2 | 启动输出含中文和 emoji | 对机器解析、容器日志和国际化不友好 | 生产模式支持 structured startup logs，banner 仅在 dev 模式启用。 |

## 5. 可靠性评估

### 5.1 已缓解的旧高风险问题

旧审计中指出的两个 P0 风险已有关键修复迹象：

1. 健康检查不再使用写请求探活。`internal/cluster/health_checker.go` 的 `performHealthCheck` 当前调用 `client.Read(ctx, healthcheck.HealthCheckMessage)`，`pkg/cluster/healthcheck/base.go` 的 `Lookup` 显式处理健康检查 query。这避免了健康检查污染 Raft 日志和业务状态机 `Update`。
2. 本地 WAL 引擎不再是单条“先 WAL 后 Update”的不可区分记录。`internal/localstate/walsm/engine.go` 当前使用 pending/commit 双记录，回放时只应用有 commit 标记的 pending entry，未提交 pending 会被跳过。

这两项应从“待修复 P0 bug”改为“已缓解但必须回归验证”。仍需补齐的验证：

- 健康检查只读 query 在所有业务状态机 wrapper 下都不触发业务副作用。
- WAL pending 写入成功但 `Update` 失败时，重启后不会回放未提交 entry。
- commit 写入失败时，状态机不再被应用；`Write` 语义为 validate -> pending WAL -> commit WAL -> memory update。commit save failure 返回 error 且不污染内存状态。

### 5.2 仍需重点关注的可靠性风险

| 优先级 | 风险 | 证据/位置 | 影响 | 计划 |
| --- | --- | --- | --- | --- |
| P0 | 缺少真实三节点故障注入 | 已新增 K8s beta scenarios 和 Gardener cluster runner | 需在 beta 环境持续运行 | `make test-distributed`。 |
| P0 | cluster 模式 delivery 依赖 Scylla，但一致性/幂等验收不完整 | 已新增真实 Scylla-backed delivery E2E | 需在 beta 环境持续运行 | `make test-scylla-integration`。 |
| P1 | shared subscription leader/consumer 语义需跨节点验证 | `internal/broker/sharedsubscription/manager`、`consumer` 有本地测试，但缺少多节点 leader 切换测试 | 共享组可能重复消费或长时间 pending | 增加 leader renewal、consumer failover、processing timeout case。 |
| P1 | owner token fencing 需要端到端验证 | session/sub-center 已有单元测试 | 跨节点 CloseClient 乱序可能关闭新连接 | 增加同 ClientID 快速重连 + 远程 close 延迟注入测试。 |
| P1 | 健康检查采样和 shard 映射需回归 | `HostedClusterIDs` 已过滤本节点承载 cluster | 后续启用 delivery shards 时可能漏检或误报 | 固化 app 层和 raft registry 回归测试。 |
| P2 | goroutine 生命周期整体审计不足 | 组件中存在多个 background scanner/consumer/runner | 停机、重启、测试环境可能泄漏 | 增加 leak/race focused tests，必要时引入 goleak。 |

## 6. 可维护性评估

### 6.1 正向信号

- 顶层结构清晰：`cmd` 入口、`app` 组合、`config` 配置、`inner` 业务实现、`pkg` 契约/基础设施、`api` 管理面。
- `app.NewApp` 的构建阶段已经拆成 infra、state centers、broker resources、component registration。
- session/subscription/will-delay 通过接口隔离 memory、WAL、Raft 实现。
- 文档体系 `docs/00-07` 已覆盖 overview、repo map、architecture、workflow、data/API、conventions、runbook、testing。
- 测试覆盖了许多 MQTT5 边界，例如 receive maximum、topic alias、retain、will delay、QoS2、shared subscription、owner token fencing。

### 6.2 维护风险

| 优先级 | 风险 | 位置/表现 | 计划 |
| --- | --- | --- | --- |
| P1 | 事件载荷使用 `map[string]interface{}` | `app.registerHealthCheckEventListeners`、`internal/cluster.emitStatusEvent` | 改为结构体事件，例如 `HealthCheckStatusEventPayload`，提升可读性和类型安全。 |
| P1 | `cmd/main.go` 超过 100 行且混合启动、展示、统计 | 启动逻辑和 UI 输出耦合 | 抽出 startup reporter 或只保留生产结构化日志。 |
| P1 | `app/app.go` 组合层仍偏大 | 单文件承担生命周期、infra、health、listener 绑定 | 继续按 service/builder 拆分，保持每个函数职责小。 |
| P2 | 部分注释含“cursor-generated” | `pkg/cluster/raft/raft.go` | 改成正常工程解释，避免工具痕迹污染长期代码。 |
| P2 | 文档与代码状态可能漂移 | 5 月 19 日旧审计部分结论已被代码更新 | 建立变更后更新 `docs/changes` 和主文档的 checklist。 |
| P2 | 分布式语义指标不足 | 现有 metric 偏组件级 | 新增 owner token conflict、shared rollback、duplicate delivery、remote close latency 等指标。 |

## 7. Bug 与缺陷清单

### P0 - 上线阻断

1. **缺少三节点分布式故障注入验收**
   - 类型: 可靠性缺陷
   - 影响: 无法证明 leader 切换、节点宕机、网络超时下 session、delivery、shared subscription 的语义正确。
   - 状态: 已新增 `make test-distributed`，默认基于 K8s 三副本 StatefulSet，覆盖 leader pod kill、broker pod kill、节点恢复、同 ClientID 快速重连和 shared rollback beta 场景。

2. **Scylla-backed delivery 生产路径缺少 E2E 证明**
   - 类型: 功能/可靠性缺陷
   - 影响: cluster 模式强依赖 Scylla pair，但需要验证 payload/task/cursor/shared task 在真实 CQL 后端的一致性。
   - 状态: 已新增 `make test-scylla-integration`，通过 K8s `svc/scylla` port-forward 运行真实 Scylla delivery E2E。

3. **commit 写入失败后的 WAL 语义需要明确**
   - 类型: 可靠性缺陷
   - 影响: `Update` 成功但 commit entry 写入失败时，内存状态已经变化，但重启后未提交 pending 会被跳过。
   - 状态: 已重构为 commit 持久化成功后才 apply 内存状态；已增加 commit save failure 故障注入测试。

### P1 - 上线前必须处理

1. **Scylla backlog summary 未实现**
   - 位置: `app/cluster_overview_provider.go`
   - 影响: cluster overview 在生产 queue 类型下返回 unsupported。
   - 计划: 在 Scylla queue store 实现 `DeliveryBacklogSummary`。

2. **共享订阅跨节点 leader failover 未验收**
   - 位置: `internal/broker/sharedsubscription/manager`、`internal/broker/sharedsubscription/consumer`
   - 影响: 共享组可能重复分配、processing 任务回滚不及时或漏唤醒。
   - 计划: 增加多节点 shared group 测试，模拟 leader 节点停止和 consumer 重启。

3. **健康检查事件载荷不是类型安全结构**
   - 位置: `internal/cluster/health_checker.go`、`app/app.go`
   - 影响: 字段名写错无法编译期发现，也不符合结构体优先的代码习惯。
   - 计划: 定义结构体事件载荷，替换 `map[string]interface{}`。

4. **生产配置模板不足**
   - 位置: `configs`
   - 影响: 上线容易漏配 TLS、Scylla、health、metrics、ACL、日志。
   - 计划: 增加 production example 和配置说明。

### P2 - 生产增强

1. **MQTT5 compliance matrix 缺失**
   - 影响: 难对外说明支持等级。
   - 计划: 维护 MQTT5 feature/spec/test mapping。

2. **分布式语义指标不足**
   - 影响: 故障时只能看到组件健康，难定位消息语义问题。
   - 计划: 增加业务指标和 dashboard。

3. **启动输出不适合生产日志**
   - 影响: 机器解析和集中日志采集不友好。
   - 计划: dev banner 和 production structured log 分离。

4. **长期文档维护机制不足**
   - 影响: `docs/` 容易跟代码漂移。
   - 计划: 在 PR checklist 中要求更新受影响文档。

## 8. 未来开发计划

### Phase 0: 上线阻断项清零

目标: 证明分布式核心语义在故障场景下可靠。

当前 beta 阻断项验收入口:

- `make test-distributed`: 启动/复用 K8s 三副本 SkyTree + K8s 三节点 Scylla，运行 Gardener `beta` cluster scenarios。
- `make test-scylla-integration`: 通过 K8s `svc/scylla` port-forward 验证 payload、task、cursor、dedupe 和 shared rollback。
- `go test ./internal/localstate/walsm`: 覆盖 pending/commit replay 和 commit save failure 不 apply 内存状态。
- `make beta-check`: 汇总 short、race core、Scylla integration 和 distributed beta。
- `docs/changes/k8s-capacity-baseline.md`: 记录 Makefile K8s profile 的 1W 默认小集群和 10W/100W/1000W 连接资源估算。

Makefile K8s profile 的默认容量 baseline:

- 1W 在线设备: SkyTree 3 x 1 vCPU/2Gi；Scylla 3 x 1 vCPU/2Gi；Scylla PVC 20Gi each。
- 10W 在线设备: SkyTree 3 x 4 vCPU/8Gi；Scylla 3 x 4 vCPU/16Gi；Scylla PVC 200Gi each。
- 100W 在线设备: SkyTree 8-12 x 8 vCPU/16Gi；Scylla 6-9 x 16 vCPU/64Gi；Scylla PVC 1TiB each 起步。
- 1000W 在线设备: SkyTree 80-120 x 8-16 vCPU/24-32Gi；Scylla 24-60 x 16-32 vCPU/128Gi；需要多 AZ、分片、SLO、压测和 soak test 共同确认。

任务:

1. 建立三节点集成测试环境。
   - 状态: 已完成，K8s 为唯一默认验收路径。
   - 输出: `make test-distributed`、`make k8s-up`、`make k8s-status`、`make k8s-down`。
   - 覆盖: 三节点启动、leader 识别、健康检查、pod 停止、pod 恢复。

2. 验证 session owner fencing。
   - 场景: 同 ClientID 在节点 A/B 快速重连，旧节点延迟发送 CloseClient。
   - 验收: `k8s-same-clientid-fast-reconnect` 证明新 owner 可接收后续消息。

3. 验证 delivery 端到端幂等。
   - 场景: QoS1/QoS2 publish、payload 保存成功但 task append 重试、client runner 重启。
   - 验收: Scylla E2E 覆盖 payload 保存/读取、task append/read、duplicate dedupe、cursor advance/read。

4. 验证 shared subscription failover。
   - 场景: shared group leader 停止、processing task 超时、consumer 迁移。
   - 验收: `k8s-shared-subscription-processing-rollback` 和 Scylla E2E 覆盖 shared rollback。

5. 补 WAL 故障注入。
   - 场景: validator reject、commit save fail、committed/uncommitted pending replay。
   - 验收: commit save failure 不改变内存状态；无 commit marker 的 pending entry 不 replay。

### Phase 1: 生产可用性补齐

目标: 让开发、测试、SRE 可以稳定启动、观察、排障。

任务:

1. 实现 Scylla backlog summary。
   - 输出: Scylla queue store 支持 `DeliveryBacklogSummary`。
   - API: `/api/v1/cluster/overview` 返回 pending tasks 和 active clients。

2. 增加生产配置模板。
   - 输出: `etc/config.production.example.yaml`。
   - 内容: TLS/mTLS、Scylla、cluster members、health check、metrics、ACL、logging。

3. 完善运维 API。
   - 新增建议: node runtime、client/session lookup、delivery backlog by client、shared group status。
   - 安全要求: 管理 API 必须鉴权，敏感字段不直接返回。

4. 完善 runbook。
   - 内容: 多节点启动、滚动重启、节点替换、Scylla 检查、常见告警处理。

5. 建立上线 checklist。
   - 内容: 配置、测试、压测、故障演练、回滚方案、数据保留策略。

### Phase 2: 可靠性增强

目标: 从“测试证明可用”提升到“长期运行可观测、可恢复”。

任务:

1. 增加 MQTT 语义 SLO 指标。
   - shared task rollback count
   - processing timeout count
   - owner token conflict count
   - remote close request latency and failure count
   - duplicate delivery count
   - delivery cursor lag

2. 增加 goroutine leak 和 shutdown 测试。
   - 范围: broker runner、shared consumer、will-delay scanner、health checker、API/gRPC server。
   - 建议: 对高风险包引入 leak 检查。

3. 压测和 soak test。
   - 场景: 长连接、订阅风暴、retain 大量匹配、shared group 高并发、节点滚动重启。
   - 输出: 容量基线和资源曲线。

4. 明确集群成员变更策略。
   - 如果支持动态 membership: 提供 API/CLI/文档和测试。
   - 如果不支持: 明确节点扩缩容需要维护窗口和重建流程。

### Phase 3: 可维护性和持续演进

目标: 降低长期演进成本，减少分布式行为回归。

任务:

1. 类型化事件载荷。
   - 将健康检查和其他内部事件的 `map[string]interface{}` 替换为结构体。

2. 拆分启动展示和生产日志。
   - `cmd/main.go` 保留流程控制，展示逻辑移到单独 reporter。
   - 生产默认使用 structured logs。

3. 整理文档和测试矩阵。
   - `docs/04-data-and-api.md`: 补 API 请求/响应。
   - `docs/07-testing.md`: 补 distributed test matrix。
   - 新增 MQTT5 compliance matrix。

4. 清理工具痕迹和注释风格。
   - 将 `cursor-generated` 之类注释改成工程原因说明。
   - 保持注释使用英文，文档可按当前 `docs/` 中文风格维护。

## 9. 上线准入标准

上线前建议满足以下硬性标准：

| 类别 | 准入标准 |
| --- | --- |
| 功能 | MQTT5 核心 feature matrix 明确支持/不支持；不支持项有文档和配置保护。 |
| 分布式 | 三节点故障注入通过，覆盖 leader 切换、节点宕机、重连、shared failover。 |
| 存储 | Scylla delivery E2E 通过，payload/task/cursor/shared task 语义明确。 |
| 安全 | 生产配置启用 TLS/mTLS 或明确的内网安全边界；管理 API 有鉴权。 |
| 可观测 | `/metrics`、cluster overview、delivery backlog、shared rollback、owner conflict 指标可用。 |
| 运维 | 有生产配置模板、部署文档、回滚方案、容量基线。 |
| 测试 | `make ci`、distributed suite、Scylla integration、race core 全部通过。 |
| 文档 | `docs/` 主文档和本计划中 P0/P1 状态已更新。 |

## 10. 建议验证命令

当前已有基础命令：

```bash
make test-configs
make test-short
make test-race-core
make ci
```

建议新增命令：

```bash
make test-distributed
make test-scylla-integration
make test-mqtt5-compliance
make test-soak-short
```

建议新增脚本：

```bash
scripts/test_distributed_cluster.sh
scripts/fault_kill_leader.sh
scripts/fault_delay_grpc.sh
scripts/check_cluster_readiness.sh
```

## 11. Review Card

上线前需要人工确认：

- Threat snapshot: MQTT client、HTTP admin、node-to-node gRPC、Scylla/Badger 存储边界是否清楚。
- Validation path: MQTT packet、配置、API 输入、topic filter、shared group name 是否都有拒绝路径。
- Secret storage: TLS key、admin password、auth plugin secret 不进入代码和日志。
- Crypto choice: TLS/mTLS 配置使用受信 CA 和强加密套件。
- Auth/session flow: CONNECT auth、enhanced AUTH、session owner fencing、remote close token 已验证。
- Authorization call: ACL 和管理 API 鉴权策略已定义。
- Logging strategy: 不记录密码、token、payload、客户内容；直接标识符按隐私规则处理。
- File/IPC safeguards: 配置路径、证书路径、Badger/Scylla 数据目录有权限边界。
- Native safety checks: Go race、goroutine leak、shutdown tests 已覆盖高风险组件。
- Dependency notes: Dragonboat、gRPC、Gin、Scylla/CQL 依赖版本和升级策略已记录。
- Privacy/retention plan: retained message、payload、session、will task 的 TTL 和删除策略已明确。
- Safety mitigations: rate limit、connection admission、maximum packet size、receive maximum 已启用。
- Tests run: unit、race、distributed、storage integration、soak、manual failover 已完成并归档。

<!-- ai-generated-end -->

<!-- PKB-metadata
last_updated: 2026-05-22
commit: 1cec350
updated_by: human+ai
doc_type: explanation,how-to
-->
