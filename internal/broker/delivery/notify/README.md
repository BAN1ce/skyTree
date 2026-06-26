# delivery/notify 目录说明

## 目录职责
- 实现客户端投递通知桥接：
  - 本地节点通知：`EventCenter.Emit`；
  - 跨节点通知：`NodeController.NotifyClientDelivery`（gRPC）。
- 统一维护监听器生命周期：`AddListener` / `DeleteListener`。

## 关键代码
- `client_delivery_event.go`：`ClientDeliveryEvent` 接口与 `New(...)` 实现。
- `model.go`：通知 payload 构建与按 client 选项展开逻辑。
