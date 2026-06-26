# session_center/raft 目录说明

## 目录职责
- 把 session_center 接口调用转发到集群 Raft 客户端。
- 负责把业务请求拆分为 write/read 两类调用。

## 关键代码
- `cluster.go`：各 session API 的集群代理实现。
