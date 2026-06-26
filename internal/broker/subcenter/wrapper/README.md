# sub_center/wrapper 目录说明

## 目录职责
- 对订阅中心做轻量包装，补充事件总线发射与统一调用入口。

## 关键代码
- `wrapper.go`：转调 subscription.Center，并在关键操作后发事件。
