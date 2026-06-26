# session_center/memory 目录说明

## 目录职责
- 会话中心的内存实现。
- 管理 `Session` 与 `SessionOwner`，支持 owner token fencing 和过期删除。

## 关键代码
- `core.go`：Open/TakeOver/SaveOffline/DeleteExpired/Snapshot 全流程。

## 你会看到的行为
- `SaveOfflineState` 会校验 owner token，忽略陈旧 owner 的写入。
- 读取时会隐藏已过期会话并在清理任务中删除。
