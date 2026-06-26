# shared_subscription 目录导览

## 目录职责
- 实现共享订阅任务消费与协调。
- 把 `$share/...` 产生的任务分发到具体在线客户端。

## 子模块
- `consumer/`：单 share group 消费循环。
- `manager/`：生命周期管理、组跟踪、回滚协调。
