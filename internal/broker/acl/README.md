# acl 目录说明

## 目录职责
- 提供 MQTT 主题级 ACL（发布/订阅）鉴权能力。
- 支持从本地文件或 KeyStore 加载规则，并热切换内存 evaluator。

## 关键代码
- `manager.go`：规则管理、加载优先级（文件优先）、Upsert/Delete、默认拒绝策略。
- `policy.go`：`AllowPublish/AllowSubscribe` 判定流程（先 deny 后 allow）。
- `match.go`：topic 匹配与 filter superset 判断。
- `source_file.go`、`source_keystore.go`：规则来源实现。

## 你会看到的行为
- 支持按 `username/client_id` 精细鉴权。
- 文件模式开启时会阻止对 KeyStore 规则直接写入。
