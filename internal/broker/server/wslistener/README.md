# server/wslistener 目录说明

## 目录职责
- 封装 MQTT over WebSocket listener（支持 ws/wss）。
- 校验 websocket 子协议并转为 `net.Conn` 提供给 broker。

## 关键代码
- `listener.go`：握手校验、连接包装、监听生命周期。
