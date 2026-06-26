# internal/disconnect 目录说明

## 目录职责
- 统一构造服务端下发的 MQTT5 DISCONNECT 报文。
- 覆盖协议错误、包过大、receive maximum 超限、topic alias 错误等场景。

## 关键代码
- `builder.go`：通用 DISCONNECT builder 和属性拼装。
- `protocol_error.go`：协议级错误映射。
- `packet_size.go`、`receive_maximum.go`、`topic_alias.go`：专项错误报文构建。
