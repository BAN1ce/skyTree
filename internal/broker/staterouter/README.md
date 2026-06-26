# staterouter — 会话状态与路由边界

`staterouter` 包是 broker 主流程与「会话状态 / 订阅 / 消息路由」之间的一层抽象边界。它把 broker 在处理 MQTT 连接生命周期时需要的所有状态操作,收敛成一个统一的接口 `Client`,并提供进程内实现 `InProcessClient`。

本文档说明这一层的设计动机、`OwnerToken`(会话所有权令牌)机制的原理,以及它到底解决了什么问题。

---

## 1. 为什么需要这一层

MQTT broker 在处理一次连接时,要反复访问几类底层能力:

- **会话中心(`session.Center`)**:打开 / 恢复会话、Clean Start 重置、保存离线状态、会话所有权接管。
- **订阅中心(`subscription.Center`)**:保存订阅、删除订阅、查询订阅、绑定 owner token。
- **集群节点控制器(`cluster.NodeController`)**:跨节点通知关闭旧连接。
- **消息路由**:把客户端发布的消息投递给订阅者。

如果让 broker 的连接处理代码(`internal/broker/client`)直接散落地调用这些底层模块,会带来两个问题:

1. **耦合**:连接处理逻辑被绑死在某一套具体实现上,难以替换、难以测试。
2. **不一致**:所有权校验(owner token)等横切逻辑会在各处重复,容易遗漏。

`staterouter.Client` 接口把这些操作统一成一个「面向 broker 的边界」(boundary)。它带来的直接好处:

- **隐藏实现细节**:调用方只依赖接口里定义的方法,看不到 `session.Center` / `subscription.Center` 的具体类型。
- **对外稳定**:只要接口签名不变,底层实现可以自由重构。
- **可替换**:`InProcessClient` 是进程内直连实现;将来若要做成跨进程 / RPC 的远程实现,只需提供另一个满足 `Client` 的类型,broker 主流程一行都不用改。这也是为什么所有请求结构体都自带 `BrokerNodeID`、`OwnerToken` 等身份信息 —— 它们天然就是「将来要序列化进 RPC 请求」的字段。
- **易测试**:测试里可用一个 fake 的 `Client`(见 `client/state_router_test.go`)替换真实实现。

文件末尾的:

```go
var _ Client = (*InProcessClient)(nil)
```

是一个**编译期断言**,保证 `*InProcessClient` 始终实现了 `Client` 接口;接口若新增方法而实现未跟上,编译期就会报错。

### 关于「返回接口还是具体类型」

`NewInProcessClient` 返回的是具体类型 `*InProcessClient`,而不是 `Client` 接口。这遵循 Go 的惯例「Accept interfaces, return structs」:构造函数返回具体类型,把「要不要抽象成接口」的决定权留给调用方。调用方在装配时会把它赋值给接口字段使用,从而同时拿到「构造期的具体类型」和「使用期的接口抽象」两方面的好处。

---

## 2. 核心概念:会话所有权(Session Ownership)与 OwnerToken

### 2.1 问题背景

MQTT 协议规定:同一个 `ClientID` 在同一时刻只能有一个活跃会话。当一个客户端用相同 `ClientID` 发起新连接时,broker 必须「接管」(takeover)旧会话,并关闭旧连接。

在**单机** broker 里这很简单:本地连接管理器里找到同 ID 的旧连接,关掉即可。

但在**集群**部署下,旧连接可能挂在**另一个节点**上。于是出现两个棘手问题:

1. **如何跨节点接管?** —— 新连接所在的节点需要知道旧 owner 在哪个节点,并通知它关闭。
2. **如何防止「过期 owner」继续写状态?** —— 网络分区、请求乱序、慢请求等情况下,旧 owner 可能在被接管之后,仍然发来「保存离线状态」「删除订阅」等请求。如果不加防护,它会污染甚至破坏新 owner 的会话状态(典型的脑裂 / split-brain 问题)。

`OwnerToken` 就是为第 2 个问题设计的。

### 2.2 OwnerToken 是什么

`OwnerToken` 是一个**所有权令牌**,本质上是分布式系统里的 **fencing token(防护令牌)**。

- 每次有新连接要接管会话时,连接处理层会生成一个全新的 token(实现上是 `uuid.NewString()`,见 `client/client_handler_connect.go`)。
- 这个 token 通过 `AcquireSession` 写入会话所有者记录(`SessionOwner.OwnerToken`),同时镜像写入订阅中心(`SetClientOwnerToken`),让会话和订阅共用同一个令牌。
- 此后,这个 owner 发起的**所有写操作**都必须带上自己的 token:`SaveOfflineState`、`Subscribe`、`DeleteClientSubscriptions`、`RoutePublish` 等。
- 底层在执行写操作前会校验 token 是否与「当前记录在册的 owner token」一致。一旦有更新的连接接管了会话,旧 token 立刻失效,旧 owner 的写请求会被拒绝。

可以把它理解成一把「带版本号的钥匙」:换锁(接管)之后,旧钥匙再也打不开门,无论它什么时候被人捡起来用。

### 2.3 为什么需要 `BrokerNodeID` / `BrokerInstanceID`

仅有 token 还不足以完成**跨节点接管**这件事本身。请求里还要携带 broker 的身份:

- **`BrokerNodeID`**:声明「现在是哪个节点在抢占这个会话」。它会被写进 `SessionOwner.NodeID`。当 `TakeOverSessionOwner` 发现存在旧 owner 时,会把旧 owner(含其 `NodeID`)返回;新节点据此通过 `ClosePreviousOwner` → `NodeController.RequestCloseClient(prevNodeID, ...)` 跨节点通知旧节点关闭旧连接。没有节点身份,集群级 takeover 无从谈起。
- **`BrokerInstanceID`**:节点实例标识,用于更细粒度地区分 owner(例如同一逻辑节点重启后的不同实例)。

换句话说:**`OwnerToken` 防的是「过期 owner 乱写」,`BrokerNodeID` 解决的是「去哪里关掉旧 owner」**。两者配合,才完整实现了 MQTT 的会话接管语义。

这也解答了一个常见疑问:「为什么获取 session 需要这么复杂的请求,还要带 broker 信息?」—— 因为 `AcquireSession` 做的根本不是「读一个会话」,而是「**以某个 broker 身份,用 fencing token 安全地抢占会话所有权,并接管旧连接**」。这些字段是这套分布式所有权协议的必需输入,而非冗余。

---

## 3. `AcquireSession` 的执行流程

`AcquireSession` 是连接建立(CONNECT)时的核心入口,内部按顺序完成多步操作:

```
AcquireSession(req)
  │
  ├─ 1. validateAcquireSessionRequest(req)          // 校验 ClientID / OwnerToken / BrokerNodeID / NowUnixNano
  │
  ├─ 2. OpenSessionForConnect(...)                  // 打开或恢复持久化会话状态
  │
  ├─ 3. if CleanStart:
  │        ReplaceSessionStateOnCleanStart(...)     // Clean Start:丢弃旧会话状态
  │
  ├─ 4. TakeOverSessionOwner(Owner{NodeID, OwnerToken, Online:true})
  │        └─ 返回旧 owner(若存在,可能在别的节点)
  │
  └─ 5. SetClientOwnerToken(...)                    // 把 owner token 镜像到订阅中心
       │
       └─ 返回 AcquireSessionResponse{ OpenSession, Takeover }
```

调用方(`client/client_handler_connect.go`)拿到响应后还会做两件收尾工作:

- `applyTakenOverOwnerToken`:把 takeover 响应里确认的 owner token 应用到本连接,作为后续所有写操作的凭证。
- `closePreviousSessionOwner`:如果 `Takeover` 表明存在旧 owner,则调用 `ClosePreviousOwner` 跨节点关闭旧连接。

### `NowUnixNano` 为什么由外部传入

`AcquireSession`、`SaveOfflineState` 等都接收 `NowUnixNano` 而不是在内部调用 `time.Now()`。原因:

1. **一致性**:一次操作内多步共用同一个时间戳,避免步骤之间的时钟漂移。
2. **可测试**:测试可注入固定时间,让结果可预期。

这是依赖注入「时间」的常见做法。

---

## 4. 接口方法一览

| 方法 | 作用 | 是否校验 OwnerToken |
|------|------|:----:|
| `AcquireSession` | 打开/恢复会话并抢占所有权(含 Clean Start 重置、旧 owner 接管) | 是 |
| `SaveOfflineState` | 客户端断开时保存离线状态(未完成消息、重放游标、遗嘱清理等) | 是 |
| `Subscribe` | 为 owner 持有的客户端保存订阅,返回最终授予的 QoS(按 MaxQoS 封顶) | 是 |
| `DeleteClientSubscriptions` | 删除 owner 持有客户端的全部订阅 | 是 |
| `ListClientSubscriptions` | 查询客户端当前订阅(只读) | 否 |
| `RoutePublish` | 路由客户端发布的消息给订阅者 | 是 |
| `HasMatchingSubscribers` | 判断某主题是否有匹配订阅者(用于保留消息/遗嘱投递判断,只读) | 否 |
| `ClosePreviousOwner` | 通知并关闭被本节点接管的旧 owner 连接 | —(校验旧 owner 身份) |

**规律**:凡是会**修改状态**的操作都校验 owner token(通过 `validateClientOwnerRequest` 统一校验 `ClientID` + `OwnerToken` + `BrokerNodeID`);**只读**查询(`ListClientSubscriptions`、`HasMatchingSubscribers`)不校验 token,只需要 `ClientID` 和 `BrokerNodeID`。

---

## 5. 组件装配

`InProcessClient` 通过 `NewInProcessClient(InProcessDependencies)` 构造,显式校验必需依赖:

```go
type InProcessDependencies struct {
    SessionCenter      session.Center        // 必需:会话中心
    SubscriptionCenter subscription.Center   // 必需:订阅中心
    NodeController     cluster.NodeController // 可选:跨节点接管才需要,单机可为 nil
    PublishRoute       PublishRouteHandler   // 必需:消息路由处理器
}
```

`PublishRouteHandler` 是一个函数类型,用依赖注入的方式把「消息路由」能力传进来,从而让 `staterouter` 不直接依赖 `delivery` 投递层(降低耦合)。可用 `NewRoutePublishHandler` 把普通函数适配成该类型,并在装配期拒绝 nil。

`InProcessClient` 本身**不持有任何「单个连接」的状态** —— 调用方身份(节点、ClientID、owner token)全部随请求传入。因此一个实例可被所有连接共享,这也正是它能平滑演进成远程实现的前提。

---

## 6. 小结:解决了什么问题

1. **解耦与可替换**:把会话/订阅/路由的底层调用收敛到一个稳定接口后面,broker 主流程不再依赖具体实现,可在进程内实现与未来的远程实现之间无缝切换。
2. **集群下的会话单一所有权**:通过 `BrokerNodeID` 实现「找到并关闭跨节点的旧 owner」,满足 MQTT「同一 ClientID 只能有一个活跃会话」的要求。
3. **防止过期 owner 破坏状态**:通过 `OwnerToken`(fencing token)让被接管的旧 owner 的迟到写请求一律失效,避免脑裂导致的状态污染。
4. **横切校验统一**:所有写操作走同一套 owner 校验逻辑,不重复、不易漏。
5. **可测试性**:时间外部注入、依赖显式校验、接口可被 fake 替换。
