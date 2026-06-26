# client_alive 目录说明

## 目录职责
- 管理本 node 当前连接的客户端活跃索引。
- 不持久化、不跨 node 复制；节点重启后连接会自然断开，索引丢失是预期行为。
- 通过 clientID map 和按过期时间排序的最小堆，快速扫描本 node 已过期的连接。
- 这是内部运行态组件，不提供外部 memory 模式配置，也不建模为可恢复状态中心。

## 关键代码
- `tracker.go`：`Tracker` 的 `Update`、`Delete`、`ScanExpired`。
- `heap.go`：按 `expireAt` 排序的最小堆实现。

## 你会看到的行为
- 每次收到客户端包时更新本地 `lastAlive` 和 `expireAt`。
- `ScanExpired` 只从堆顶连续弹出已过期客户端，堆顶未过期时立即停止。
- broker 关闭候选客户端前仍会通过 client manager 读取当前连接并二次确认超时状态。
