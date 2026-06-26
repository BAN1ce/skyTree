# `pkg/scheduler`

`pkg/scheduler` 提供基于 `min-heap` 的内存调度器，供 `publish retry` 等延迟任务场景使用。

## 当前实现

- `PassiveScheduler` 维护 `heap + map`，按任务到期时间索引。
- `ActiveScheduler` 在 `ticker` 驱动下轮询 `PassiveScheduler`，执行已到期任务回调。
- 当前实现不是传统按槽推进的时间轮，不存在 `slot_num` 这一类容量调优参数。

## 与 publish retry 的关系

`publish retry` 通过 `pkg/retry.DelayTaskSchedule` 使用本调度器：

- 业务重试间隔由 `broker.message_retry.interval` 决定。
- 调度器 tick 粒度由 `broker.message_retry.scheduler_interval` 决定。
- 最大重试次数与超时分别由 `broker.message_retry.max_retry_count`、`broker.message_retry.max_timeout` 控制。

## Benchmark

命令：

```bash
go test ./pkg/scheduler -run '^$' -bench 'BenchmarkPassiveSchedulerWorkloadProfiles' -benchmem -benchtime=200ms
```

本机实测环境：

- Date: `2026-06-24`
- CPU: `Apple M4 Pro`
- OS/Arch: `darwin/arm64`

结论：

- 无到期任务时，tick 成本很低。
- 主要成本来自批量创建任务和单次 tick 内同时到期的大批任务。
- 当前 heap 调度器在 `30,000` 个挂起任务、`3,000` 个任务同 tick 到期时仍保持在可接受范围内。

## 建议

- 小规模负载优先调 `broker.message_retry.interval`，不要试图寻找不存在的 `slot_num`。
- 需要更快补发时，可减小 `broker.message_retry.scheduler_interval` 与 `broker.message_retry.interval`，但应结合重试风暴风险一起评估。
- 如果未来调度语义再变化，应继续保持命名与实现一致，避免重新引入 “scheduler 之外其实是别的东西” 的误导。
