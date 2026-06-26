# shared_subscription/consumer 目录说明

## 目录职责
- 消费共享订阅队列任务并选择目标客户端。
- 通过 CAS 把任务从 `pending` 置为 `processing`，并写入 client delivery task。

## 关键代码
- `consumer.go`：`Run/Wake/processTask`、在线成员选择、游标推进。

## 你会看到的行为
- 无可用在线成员时会把任务状态回滚。
- 分配成功后会唤醒目标客户端的 delivery runner。
