# session_center/wal 目录说明

## 目录职责
- 单机持久化版 session_center。
- 基于 `internal/localstate/walsm` 把会话状态写入 WAL 并支持快照恢复。

## 关键代码
- `local_wal_center.go`：session.Center 全接口实现。
