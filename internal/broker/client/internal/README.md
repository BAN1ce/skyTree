# core/client/internal 目录说明

## 目录职责
- 存放 `client` 包内部复用的小型基础组件。
- 这些实现不直接对外暴露，服务于协议细节处理。

## 子目录
- `disconnect/`：服务端 DISCONNECT 构造器。
- `packetid/`：Packet ID 生成与映射。
- `topicalias/`：MQTT5 Topic Alias 双向管理。
