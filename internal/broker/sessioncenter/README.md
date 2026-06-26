# session_center 目录导览

## 目录职责
- 提供会话状态中心（session + owner）能力。
- 统一承载连接接管、离线状态保存、未完成消息恢复、会话过期清理。

## 子模块
- `memory/`：内存实现。
- `raft/`：集群客户端代理。
- `statemachine/`：Raft 状态机应用层。
- `wal/`：单机 WAL 持久化实现。
