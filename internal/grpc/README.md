# grpc 目录说明

## 目录职责
- 节点间 gRPC 服务端实现。
- 主要能力：客户端投递通知转发、会话接管时远端关闭客户端连接。

## 关键代码
- `server.go`：gRPC 服务启动/关闭、TLS/mTLS 选项。
- `service_client_delivery_notify.go`：跨节点投递唤醒与 QoS0 直发通知。
- `service_close_client.go`：按 owner token 关闭被接管连接。

## 你会看到的行为
- 支持强制 TLS 或显式 allow_insecure。
- 关闭客户端时使用 owner token fencing 防止误关新连接。
