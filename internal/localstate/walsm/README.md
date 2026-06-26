# localstate/walsm 目录说明

## 目录职责
- 提供通用“本地状态机 + etcd WAL + 快照”引擎。
- 让业务状态机具备单机持久化、重启恢复与定期快照能力。

## 关键代码
- `engine.go`：写入 pending/commit 记录、回放、快照触发。
- `wal_store.go`：WAL 打开/修复/保存。
- `snapshot_store.go`：快照文件保存、读取与清理。

## 你会看到的行为
- 回放时会跳过未提交 pending 记录，避免脏重放。
- 采用“WAL + 快照”双层恢复策略。
