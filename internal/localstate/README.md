# localstate 目录导览

## 目录职责
- 存放单机本地状态持久化基础设施。
- 当前核心是 WAL 状态机引擎，供 session/sub/will-delay 的本地持久化实现复用。

## 子模块
- `walsm/`：通用 WAL + Snapshot 状态机引擎。
