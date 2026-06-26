# plugin 目录说明

## 目录职责
- 提供 Broker 插件机制：报文钩子、客户端生命周期钩子、错误钩子。
- 内置 ACL、认证、AUTH 报文、监控、错误处理插件装配能力。

## 关键代码
- `types.go`：插件函数签名与 `Plugins` 聚合结构。
- `manager.go`：按事件顺序执行插件链（失败即返回）。
- `builder.go`：快速组装默认插件集合。
- `acl.go`、`auth.go`、`auth_packet.go`、`metrics.go`：内置插件实现。

## 你会看到的行为
- CONNECT/SUB/PUBLISH 等关键路径都可挂插件。
- ACL 插件和 AUTH 插件在连接早期即参与决策。
