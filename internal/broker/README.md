# broker 目录导览

## 模块职责矩阵
- `core/`
  - 输入：server accept 连接、client 协议包、session/subscription/delivery store、cluster 通知器。
  - 输出：客户端生命周期驱动、投递编排调用、跨节点通知请求。
- `core/orchestrator/delivery/`
  - 输入：publish route result、delivery task store、session owner 查询结果。
  - 输出：普通投递任务落库、wake 与 QoS0 direct 通知。
- `core/orchestrator/session/`
  - 输入：session center 事件与 owner 状态。
  - 输出：会话相关编排动作（清理、唤醒路由、生命周期触发）。
- `core/orchestrator/shared/`
  - 输入：shared routing result、shared domain rollback/strategy 服务。
  - 输出：shared task enqueue/wake/rollback 编排调用。
- `delivery/`
  - 输入：publish packet、sub center 查询结果。
  - 输出：client plan/share group task route result、task/cursor 存储接口。
- `shared_subscription/`
  - 输入：shared task queue、在线成员信息、客户端订阅选项。
  - 输出：共享订阅 candidate 选择、状态转换、回滚与消费执行。
- `session_center/`
  - 输入：client connect/disconnect、session 状态读写请求。
  - 输出：session owner/session payload 持久化与查询。
- `sub_center/`
  - 输入：subscribe/unsubscribe 指令、topic filter。
  - 输出：匹配订阅者、share group 成员、客户端订阅配置。
- `retain/`
  - 输入：retain publish 写入与主题查询。
  - 输出：retain 消息读写能力。
- `will_delay/`
  - 输入：will message 与延迟参数。
  - 输出：到期补发、会话清理协同。
- `plugin/acl/auth/`
  - 输入：连接认证、发布/订阅鉴权上下文。
  - 输出：策略决策与拦截结果。

## 约束
- `orchestrator/*` 只做编排，不承载领域状态机与业务筛选决策。
- shared domain 的状态转换与回滚规则统一收敛在 `shared_subscription/domain/`。

## 阅读建议
1. 先读 `core/` 入口，再沿 `delivery -> shared_subscription -> session_center/sub_center` 追主链路。
2. 最后阅读 `plugin`、`acl`、`auth`、`retain` 等扩展能力。
