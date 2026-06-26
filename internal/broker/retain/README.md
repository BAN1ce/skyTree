# retain 目录说明

## 目录职责
- 管理 retained message 的存储、读取与过期清理。
- 支持 topic filter 匹配读取和后台 GC。
- 存储模式跟随 app 层注入的 KeyStore：单机使用本地 KeyStore，集群使用 Raft-backed KeyStore。
- 不提供独立 memory/wal/raft 配置分支。

## 关键代码
- `retain.go`：`Put/Get/Delete/GetByTopicFilter`、`StartGC/RunGCOnce`。

## 你会看到的行为
- 读路径包含 lazy-delete：命中过期消息会顺手删除。
- GC 会清理过期与脏数据记录。
