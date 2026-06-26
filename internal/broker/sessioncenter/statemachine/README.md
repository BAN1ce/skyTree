# session_center/statemachine 目录说明

## 目录职责
- Dragonboat 状态机实现：把日志请求 apply 到 `memory.Core`。
- 处理请求编解码、快照保存与恢复。

## 关键代码
- `state_machine.go`：Update/Lookup/Snapshot。
- `codec.go`：请求封装编码。
