# pkg 目录导览

## 定位
- `pkg` 是跨层可复用包集合，提供稳定接口、协议模型与基础设施组件。
- `pkg` 不承载 broker 业务编排逻辑，不依赖 `inner`。

## 应该放在 pkg 的内容
- 协议与数据模型：如 `mqtt5/`、`broker/message`、`cluster/dbpb`。
- 端口与契约接口：如 `broker/persistence`、`broker/session`、`broker/subscription`。
- 基础能力：如 `eventbus/`、`retry/`、`scheduler/`、`bufferpool/`、`syncx/`。
- 通用存储实现：如 `storage/*`（与业务编排解耦）。

## 不应放在 pkg 的内容
- broker 业务用例编排与策略决策（应放 `logic` 或 `internal/broker`）。
- 只服务某个 inner 子模块的临时逻辑。
- 依赖 `inner` 才能成立的代码。

## 依赖规则
- 允许：`cmd/app/logic/inner -> pkg`。
- 禁止：`pkg -> inner`。

## 命名约定
- 新增目录命名保持一致风格，避免 `shared_subscription` 与 `sharedsubscription` 并存。
- 优先使用结构体建模业务状态，只有 key 集合无法预定义时才使用 map。
