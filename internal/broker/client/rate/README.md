# rate 目录说明

## 目录职责
- 提供客户端级令牌桶实现。
- 主要用于控制并发发送窗口和背压。

## 关键代码
- `bucket.go`：`GetToken/TryGetToken/PutToken`。
