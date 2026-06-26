# internal/topicalias 目录说明

## 目录职责
- 实现 MQTT5 Topic Alias 的上行/下行管理。
- 负责 alias 建立、复用、回滚和合法性校验。

## 关键代码
- `manager.go`：`ApplyDownlink/ApplyUplink/RevertDownlinkAlias`。
- `errors.go`：Topic Alias 相关错误定义。
