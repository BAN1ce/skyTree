# will_delay 目录说明

## 目录职责
- 实现 MQTT Will Delay 功能。
- 管理延迟遗嘱任务的存储、到期扫描、leader 执行与消息发布。

## 关键代码
- `center.go`：Broker 依赖的 `Center` 接口。
- `scanner.go`：leader 扫描到达触发时间的任务并投递遗嘱消息。
- `internal/state/`：indexed heap + 任务 map 的核心模型和任务校验。
- `wal/`：单机本地 WAL + snapshot 持久化实现。
- `raft/`：集群 Raft client proxy。
- `statemachine/`：WAL/Raft 共用状态机。
- `memory/`：内部/测试用内存实现，不作为外部可配置运行模式。

## 你会看到的行为
- 通过 owner token 校验避免陈旧任务误触发。
- 只有 leader 节点执行扫描发布逻辑。
- 生产启动路径只有 single-node local WAL 和 cluster Raft 两种形态。
