# session_center/internal 目录说明

## 目录职责
- 存放 session_center 的内部基础设施代码。
- 主要用于状态机 apply 路径上的对象复用和性能优化。

## 子目录
- `pool/`：protobuf 请求对象池。
