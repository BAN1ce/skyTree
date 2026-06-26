# shared_subscription/manager 目录说明

## 目录职责
- 共享订阅协调器：管理 consumer 生命周期和客户端-共享组关系。
- 处理离线延迟回滚、取消订阅回滚、可选 leader 续约。

## 关键代码
- `manager.go`：`OnClientOnline/Offline/Unsubscribe`、`ensureConsumer`、rollback 机制。

## 你会看到的行为
- 为每个 shareGroup 保持一个 consumer。
- 可结合 leader election 做跨节点消费协调。
