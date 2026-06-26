# sub_center/statemachine 目录说明

## 目录职责
- Dragonboat 状态机：应用订阅中心写请求并提供读查询。
- 支持状态快照和恢复。

## 关键代码
- `state_machine.go`：Update/Lookup/Snapshot。
- `codec.go`：更新请求编码与结果解码。
