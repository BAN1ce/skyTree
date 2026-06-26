# server 目录说明

## 目录职责
- Broker 网络接入层，统一管理多协议 listener（tcp/tls/ws/wss）。
- 对上输出统一 `conn chan net.Conn` 给 core 层消费。

## 关键代码
- `server.go`：listener 生命周期、accept loop、关闭协调。
- `options.go`：TLS/mTLS 配置入口。
- `tls_reloader.go`：证书热加载。
- `mtls.go`：mTLS 校验模式应用。

## 子目录
- `tcp/`、`tlslistener/`、`wslistener/`：各协议 listener 具体实现。
