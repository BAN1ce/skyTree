# sub_center/raft 目录说明

## 目录职责
- 将订阅中心接口请求代理到集群客户端。

## 关键代码
- `cluster.go`：CreateSub/DeleteSub/Match 查询/OwnerToken 等全部 API 转发。
