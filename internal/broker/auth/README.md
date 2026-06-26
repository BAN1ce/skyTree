# auth 目录说明

## 目录职责
- 定义 MQTT5 AUTH 报文外部认证提供者接口。
- 提供 HTTP 与 gRPC 两种 provider 实现。

## 关键代码
- `interface.go`：`AuthProvider` 接口和超时边界规则。
- `http_provider.go`：请求构造、超时控制、响应反序列化、指标上报。
- `grpc_provider.go`：gRPC 连接与认证请求封装。

## 你会看到的行为
- provider 会把 AUTH reason/properties 映射为统一报文。
- 认证调用统一受超时保护，并带有失败分类指标。
