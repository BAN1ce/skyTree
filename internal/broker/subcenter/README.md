# sub_center 目录导览

## 目录职责
- 提供订阅中心能力：订阅写入、匹配查询、客户端与主题清理、子树导出。
- 是消息路由（delivery router）的核心输入源。

## 子模块
- `memory/`：分片+Trie 的内存实现。
- `raft/`、`statemachine/`、`wal/`：集群与单机持久化实现。
- `wrapper/`：包装并补充事件发射。
