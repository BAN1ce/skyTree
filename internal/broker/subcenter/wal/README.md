# sub_center/wal 目录说明

## 目录职责
- 单机持久化版订阅中心。
- 基于 `localstate/walsm` 对订阅状态进行 WAL 落盘与恢复。

## 关键代码
- `local_wal_center.go`：订阅中心全接口实现。
