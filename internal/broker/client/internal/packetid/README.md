# internal/packetid 目录说明

## 目录职责
- 管理 MQTT Packet ID 的分配与关联。
- 支持 PacketID 与 topic 的映射，用于 QoS 流程追踪。

## 关键代码
- `packet_id.go`：`PacketIDFactory`、字符串序列化、`PacketIDTopic` 映射。
