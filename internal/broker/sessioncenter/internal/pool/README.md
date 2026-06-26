# session_center/internal/pool 目录说明

## 目录职责
- 为 session_center 状态机提供请求对象池，减少 GC 压力。

## 关键代码
- `pool.go`：各类 `proto_session` 请求结构的 `sync.Pool`。
