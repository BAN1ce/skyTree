# SkyTree 分布式 MQTT5 Broker 不足与潜在 Bug 审计

日期: 2026-05-19  
范围: 架构文档 + 核心代码静态审计 + 回归测试信号

## 审计方法

1. 阅读仓库现有架构/流程文档（`docs/00-overview.md`、`docs/02-architecture.md`、`docs/03-workflows.md`、`docs/04-data-and-api.md`）。
2. 重点审计分布式关键路径：
   - Raft 集群注册与健康检查
   - session/subscription 本地 WAL 与状态机
   - shared subscription 路由与消费
   - 节点间 gRPC 通信
3. 运行测试验证基线：
   - `make test-short`（通过）
   - `make test-race-core`（通过）

> 说明：本报告聚焦“分布式 MQTT5 broker 视角”的不足与潜在 bug。即使当前测试通过，仍可能在大规模、异常网络或成员变更场景中暴露问题。

## 结论总览

当前项目在单元测试和并发基础正确性方面表现较好，但在“分布式一致性验证、健康探测设计、WAL 失败恢复、运维可观测细粒度”上仍有明显短板。  
尤其是以下 3 项，建议优先修复：

1. **健康检查使用写请求探活**，会污染 Raft 日志并可能导致业务状态机被异常请求干扰。
2. **WAL 写入顺序为“先落 WAL 再执行状态机”**，状态机执行失败后会把坏日志永久写入，重启回放可持续失败。
3. **健康检查未按本节点实际承载分片过滤**，在分片部署场景容易产生误报（节点被标记不健康但实际未承载该分片）。

---

## 问题清单（按优先级）

### P0 - 高风险

#### 1) 健康检查通过 `Write` 执行，可能污染一致性日志并引发副作用

- 位置: `internal/cluster/health_checker.go`
- 证据:
  - `performHealthCheck()` 调用 `client.Write(ctx, []byte("health_check"))`
  - 该请求会走 Raft `SyncPropose`，进入状态机 `Update()`
  - 各业务状态机的 `Update()` 期望的是 protobuf 请求；`"health_check"` 不符合协议
- 风险:
  - 每次探活都会追加日志，增加 Raft 日志与快照负担。
  - 对严格解析的状态机会触发反序列化错误，导致健康状态抖动。
  - 探活逻辑与业务写路径耦合，故障域扩大。
- 建议:
  - 优先改为只读探活（例如 `Read` + noop query）或 raft 内置 leader/readiness 探测。
  - 至少定义统一的“健康检查专用请求类型”，由状态机显式处理为 no-op，不落业务副作用。

#### 2) WAL 引擎写入顺序可导致“坏日志持久化”

- 位置: `internal/localstate/walsm/engine.go`
- 证据:
  - `Write()` 里先 `e.wal.Save(...)`，后执行 `e.sm.Update(updateBytes)`
  - 如果 `Update` 返回 error，WAL 条目已持久化
  - 后续重启回放同一条目，可能再次失败
- 风险:
  - 单节点持久化模式可能进入“重启即失败”的恢复循环。
  - WAL 数据与状态机应用状态不一致，排障复杂。
- 建议:
  - 至少对失败场景增加可恢复策略（如记录 poison entry 并跳过机制、隔离坏条目）。
  - 更稳妥方案是将失败语义明确化：状态机 `Update` 不应返回非确定性错误，校验前置到入队前。
  - 增加故障注入测试：模拟 `Update` 失败后重启恢复路径。

### P1 - 中高风险

#### 3) 健康检查按节点承载分片过滤需防回归

- 位置:
  - `pkg/cluster/raft/raft.go`（`RegisterStateMachinesWithMembers` 会跳过非成员分片）
  - `app/app.go`（`buildHealthChecker` 通过 `cluster.StartedClusterIDs()` 注入本节点已启动 cluster）
  - `internal/cluster/health_checker.go`（`HostedClusterIDs` 非空时仅注册本节点承载 cluster）
- 当前状态:
  - 主路径已过滤本节点未承载的 Raft cluster，原“初始化时对所有分片都创建 client 并检查”的结论不再成立。
  - `internal/cluster/health_checker_test.go` 已覆盖 `HostedClusterIDs` 过滤逻辑。
  - `app/health_checker_test.go` 已覆盖本节点没有已启动 cluster 时不启动健康检查，避免空 hosted 列表退回全量检查。
- 剩余风险:
  - 如果后续重新启用 delivery queue Raft shards，需要同时传入 shard 数量和本节点承载的 shard cluster IDs；否则可能出现漏检或退化为全量检查。
- 建议:
  - 保留 app 层回归测试，确保健康检查只使用 `cluster.StartedClusterIDs()` 返回的本节点已启动 cluster。
  - 若引入 per-shard membership，健康检查维度继续区分：
    - **Node-hosted shards**（本节点承载）做可写/可读探测
    - **Non-hosted shards** 仅汇总 leader 可达性或通过成员表观察

#### 4) Shared subscription 组成员返回顺序不稳定（已修复）

- 位置: `internal/broker/subcenter/memory/shared_subscription.go`
- 当前状态:
  - map 聚合成员后已按 `clientID + topicFilter` 稳定排序再返回。
  - `internal/broker/subcenter/memory/memory_test.go` 已覆盖重复查询的稳定顺序。
- 剩余风险:
  - 接口注释仍未明确返回顺序语义，调用方不易知道可依赖的排序规则。
- 建议:
  - 在 API/接口文档中明确返回顺序为 `clientID`、`topicFilter` 升序。

### P2 - 中风险（工程不足）

#### 5) 分布式路径测试深度不够，尤其是跨节点故障场景

- 现象:
  - 核心测试覆盖较广，但多个 raft 封装层本身缺少直接测试（例如若干 `.../raft` 包显示 `[no test files]`）。
  - 当前测试主要验证功能与 race，缺少“网络分区、leader 切换、慢节点、成员重配置”等分布式故障注入。
- 风险:
  - 平时 CI 全绿，但线上复杂拓扑下才暴露时序问题。
- 建议:
  - 增加最小三节点集群集成测试矩阵：
    - leader 切换期间 session owner fencing
    - shared task 在节点掉线时的回滚与重分配
    - 局部网络超时下 gRPC 重连与重复投递幂等

#### 6) 观测信号偏“组件级”，缺少“语义级”分布式 SLO 指标

- 现象:
  - 已有基础 metrics 与健康接口，但缺少面向业务语义的高价值指标（例如跨节点重复投递率、共享订阅回滚率、owner token 冲突率）。
- 风险:
  - 出现“消息延迟抖动/偶发重复”时，难快速定位到 session/sub/shared 哪一层。
- 建议:
  - 新增 MQTT 语义相关指标与审计日志：
    - shared task pending->processing 超时回滚次数
    - owner token 冲突拒绝次数
    - 远程 close-client 请求成功率与时延分位

---

## 已验证的正向信号

1. `make test-short` 通过。
2. `make test-race-core` 通过。
3. session/sub/will-delay/shared subscription 等核心模块已有较完善的基础测试覆盖。

这说明项目基础质量不低，但仍需补齐“分布式异常场景”防线。

## 建议的修复优先级路线图

### 第一阶段（立即）

1. 修复健康检查写探活问题（P0）。
2. 为 WAL 引擎补充失败恢复保护与测试（P0）。
3. 健康检查只覆盖本节点承载分片（P1）。

### 第二阶段（近期）

1. Shared member 返回结果稳定排序（P1）。
2. 补一批三节点故障注入测试（P2）。

### 第三阶段（持续）

1. 建立分布式 MQTT 语义 SLO 指标看板（P2）。
2. 把本报告中的高风险项纳入 CI 的回归用例。

---

## 一句话结论

SkyTree 当前已具备“可运行的分布式 MQTT5 broker 核心能力”，但要达到“生产级高可用与高可观测”，应优先修复健康检查与 WAL 失败恢复这两处核心可靠性缺口，并系统补强分布式故障注入测试。
