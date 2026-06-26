# delivery/event 目录说明

## 目录职责
- 定义投递通知事件结构与枚举。
- 作为 `delivery runner` 唤醒与 QoS0 直发通知的协议载体。
- 与 `internal/broker/delivery/notify` 配合使用：本目录只放模型，不放桥接逻辑。

## 关键代码
- `types.go`：`Kind` 与 `Notify`。
