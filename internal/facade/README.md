# facade 目录说明

## 目录职责
- 对 `publish retry` 提供统一入口（`RetrySchedule`）。
- 把业务重试动作（`RetryWorker`）和底层调度器（`pkg/retry.DelayTaskSchedule`）解耦。
- 提供 `PublishRetry` adapter，由 Broker 显式持有并注入到 client 组件。

## retry publish 是怎么实现的

### 1) 启动阶段：Broker 内部创建 PublishRetry
- 在 `internal/broker/core.NewBroker(...)` 中，Broker 会创建并持有 `facade.PublishRetry`。
- `NewPublishRetry` 内部把 `brokerCore.CallRetry` 和 `brokerCore.CallTimeout` 传给 `pkg/retry.NewSchedule(...)`，作为任务执行和超时回调。
- `Broker.Start(ctx)` 会先启动 publish retry 调度器，再启动 MQTT server 和 client accept loop。

### 2) 运行阶段：任务如何入队
- 发送失败重试入口在 `internal/broker/core/client/client_identity.go` 的 `RetrySend(...)`。
- 当重发失败 (`sendErr != nil`) 且消息带有 `message.RetryInfo` 时，使用 client 组件显式注入的 `publishRetry`。
- 如果 `publishRetry` 未注入，会记录配置错误，不再回退到全局空实现。
- 然后调用：
  - `retrySchedule.Create(retry.NewTask(message.RetryInfo.Key, message, c.ID, message.RetryInfo.IntervalTime))`
- `facade.PublishRetry.Create(...)` 会记录指标后，把任务交给调度器。

### 3) 调度执行阶段：什么时候重试，什么时候超时
- 底层调度器在 `pkg/retry/retry.go`，核心逻辑是 `handleTask(...)`：
  - 若 `task.Data.RetryInfo.IsTimeout()` 为 `true`，走 `timeoutFunc`（即 `brokerCore.CallTimeout`）。
  - 否则走 `callFunc`（即 `brokerCore.CallRetry`）。
- `CallRetry` 会通过 `clientID` 找在线 client，然后再次执行 `c.RetrySend(task.Data)`。
- 如果再次发送失败，`RetrySend` 会再次 `Create(...)`，形成“失败即重新入队”的循环，直到成功或超时。

### 4) 超时后的处理
- `CallTimeout` 会：
  - 增加 `skytree_mqtt_publish_retry_actions_total{action="timeout"}` 指标；
  - 记录 timeout 日志（含 `retry_key/client_id/message_id`）；
  - 断开当前长期未完成 ACK 的 client 连接；
  - 保留持久 session 中的 `outgoing unfinished` 位点，等待客户端下次恢复 session 后继续投递。
- `outgoing unfinished` 的清理发生在 ACK 完成、消息过期、session 过期或 session 删除等明确生命周期节点，不由 retry timeout 删除。
- `RetryInfo.IsTimeout()` 的判定包含两类条件：
  - 达到最大重试次数（默认 3 次）；
  - 或首发时间到现在超过超时时间（默认 30 秒）。

## facade 层提供的关键能力

### 1) 显式依赖注入
- Broker 默认构造真实 `PublishRetry`，并通过 `client.WithPublishRetry(...)` 注入到 client。
- 测试或特殊场景仍可通过 `core.WithPublishRetry(...)` / `Broker.SetPublishRetry(...)` 显式替换 schedule。

### 2) 生命周期控制
- `StartSchedule(ctx)`：由 Broker 在自身启动流程中调用，启动调度器但不作为独立 lifecycle component 阻塞。
- `Close()`：触发 `cancel()`，优雅停止调度器。

### 3) 指标埋点
- `Create(...)`：
  - `skytree_mqtt_publish_retry_tasks_current +1`
  - `skytree_mqtt_publish_retry_actions_total{action="create"} +1`
- `Delete(...)`：
  - `skytree_mqtt_publish_retry_tasks_current -1`
  - `skytree_mqtt_publish_retry_actions_total{action="delete"} +1`

## 注意点（当前实现）
- `PublishRetry.Retry(...)` / `PublishRetry.Timeout(...)` 当前是空实现；实际执行路径使用的是构造时注入的 `CallRetry` / `CallTimeout` 回调。
- `Delete(key)` 能力已提供在 facade 中，但在当前 broker/client 主路径里没有直接调用点；如果后续希望在 ACK 成功后主动取消待重试任务，可在相应确认路径补充 `Delete` 调用。

## 关键代码
- `retry.go`: `PublishRetry` 生命周期、调度器启动和指标埋点。
- `../broker/core/broker_runtime.go`：`CallRetry` / `CallTimeout` 的业务处理。
- `../../pkg/retry/retry.go`：调度与重试/超时分流逻辑。
