# sub_center/memory 目录说明

## 目录职责
- 订阅中心内存实现，采用 topic 分片 + SubCore Trie 结构。
- 支持普通订阅与共享订阅匹配，并维护 client 索引与 owner token fencing。

## 关键代码
- `types.go`、`sharding.go`：分片结构与客户端索引。
- `sub_core.go`：topic filter 校验、匹配算法、共享订阅遍历。
- `write_ops.go`、`read_ops.go`：读写 API。
- `snapshot.go`：快照写入/恢复。

## 你会看到的行为
- Create/Delete 支持 owner token 栅栏，避免旧连接误写。
- `GetAllMatchClientV2` 保留重叠订阅，供上层做客户端级去重。
