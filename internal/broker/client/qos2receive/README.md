# qos2receive 目录说明

## 目录职责
- 管理客户端上行 QoS2 的“已收未完结”消息状态。
- 用于 PUBREC/PUBREL/PUBCOMP 流程中的去重与恢复。

## 关键代码
- `qos2_receiver.go`：内存存取、TTL 清理、并发访问保护。
