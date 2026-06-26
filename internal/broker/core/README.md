# core 目录说明

## 目录职责
- Broker 运行时总编排层：启动监听、接受连接、创建 Client、关闭清理。
- 串联发布路由、消息落库、投递唤醒、共享订阅与 will delay 扫描。

## 关键代码
- `broker.go`：`Broker` 结构体与依赖注入、ACL 插件挂载。
- `broker_runtime.go`：`Start/Close`、连接 accept loop、周期清理任务。
- `broker_delivery.go`：发布完成后的路由、任务写入、客户端唤醒。
- `option.go`：外部注入 store/center/plugin 的配置入口。

## 你会看到的行为
- 支持优雅关停广播 DISCONNECT。
- QoS0 在配置下可走“直发不落库”快路径。
