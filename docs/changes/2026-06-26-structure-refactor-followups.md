# 结构优化后续计划：双 internal 收敛、Dockerfile 合并、proto 对齐

<!-- maintained-by: human+ai -->

状态：计划（未执行）
关联：承接 2026-06 的目录/命名优化（仓库卫生、docs/k8s 合并、`pkg/brokerapi`、`internal/broker` 命名统一、`configs`→`etc`、死代码清理、golden 测试修复）。

本文给出三项剩余改造的执行计划。三项相互独立，建议各自单独成 commit / PR，按下文“总体排期”顺序推进。

---

## 一、收敛“双 internal”：`app/` → `internal/app/`

> 状态：✅ 已执行（2026-06-26）。`app/` 已并入 `internal/app/`，`app/internal/*` 已上提为 `internal/app/*`，import 路径、`internal/cmdruntime` 引用方、`.gitignore`（`internal/app/*_center/`）与 repo-map/overview 文档均已同步；`go build ./...` 与 `internal/app`、`internal/cmdruntime` 测试通过。下文为当时的执行计划，保留作记录。

### 现状（迁移前，基于代码实测）

- `app/` 根目录约 305 行：`app.go`、`lifecycle.go`、`options.go`（+ 测试）。
- `app/internal/` 约 3202 行，含 8 个子包：`bootstrap`、`statecenter`、`lifecycle`、`clusterruntime`、`brokerruntime`、`serverruntime`、`consoleruntime`、`storeruntime`。
- 调用链：`cmd/main.go` → `internal/cmdruntime` → `app`（根）→ `app/internal/*` + `internal/*`。
- `app/` 仅被 `internal/cmdruntime` 的 2 个文件引用（`shutdown.go`、`startup_orchestrator.go`），**并非对外公共 API**。
- 依赖方向干净：`app/internal` 引用根 `internal/` 共 13 处；根 `internal/` 引用 `app/internal/` 为 0。

### 问题

模块里同时存在根 `internal/`（核心业务）与 `app/internal/`（app 私有装配），形成两棵 internal 树。`app/` 作为顶层目录却只被内部引用，心智上割裂了“入口在 cmd、私有代码全在 internal”的单一模型。

### 目标结构

```
internal/app/            <- 原 app/ 根文件 (app.go, lifecycle.go, options.go)
internal/app/bootstrap/  <- 原 app/internal/bootstrap
internal/app/statecenter/
internal/app/clusterruntime/
...（其余 6 个子包同理，去掉多余的 internal 层）
```

即：`app/` 整体并入 `internal/app/`，并把 `app/internal/*` 上提一层为 `internal/app/*`（既然已在根 internal 下，无需再嵌套 internal 来保私有）。

### 步骤

1. `git mv app internal/app`。
2. 把 `internal/app/internal/*` 上提：`git mv internal/app/internal/<pkg> internal/app/<pkg>`（8 个子包），删除空的 `internal/app/internal/`。
3. 全仓替换 import 路径（纯字符串、可 grep 验证）：
   - `skyTree/app/internal/` → `skyTree/internal/app/`
   - `skyTree/app"` 与 `skyTree/app/`（根 app 包）→ `skyTree/internal/app`
4. 更新 2 处引用方：`internal/cmdruntime/shutdown.go`、`internal/cmdruntime/startup_orchestrator.go`。
5. 检查包名：根 `app` 包的 `package app` 可保留（目录 basename 仍为 `app`，一致）；子包包名不变。
6. 更新文档引用：`app/README.md`（移动到 `internal/app/README.md`）、`docs/01-repo-map.md`（第 23 行 `app/` 行、第 2 节入口、第 40 行 wiring 提示）、`docs/00-overview.md` 等出现 `app/` 路径处。

### 风险与缓解

- 风险中等：纯 import 路径迁移，无包名/选择子变更，`go build` 可完整捕获遗漏。
- 注意 `app/internal/*_center` 在 `.gitignore`（第 19 行 `app/*_center/`）——这是运行时数据目录忽略规则，迁移后需同步改为 `internal/app/*_center/`，否则运行时数据可能被误提交。**这条容易漏，务必处理。**
- 测试文件（`app_lifecycle_test.go` 等）随迁移，包内引用不受影响。

### 验证

- `go build ./...` 通过。
- `grep -rn 'skyTree/app' --include=*.go .` 应为空（全部变成 `skyTree/internal/app`）。
- `go test ./internal/app/... ./internal/cmdruntime/...` 通过。
- `go run ./cmd/main.go --config ./etc/config.yaml` 能正常启动到 ready（冒烟）。

### 回滚

单一 commit，`git revert` 即可。建议迁移前确保工作区干净。

### 工作量

约 0.5 天。机械迁移为主，主要时间花在验证启动链路与 `.gitignore`/文档同步。

---

## 二、合并两个 Dockerfile（多阶段 / 多 target）

### 现状（diff 实测）

| 文件 | 基础镜像 | 特点 | 被谁引用 |
| --- | --- | --- | --- |
| `Dockerfile` | `ubuntu:24.04` | 装 ca-certificates/tzdata/wget；建 skytree 用户；`HEALTHCHECK` 走 wget；有 shell | `scripts/k8s/dev.sh`（默认 `DOCKERFILE`） |
| `Dockerfile.k8s` | `scratch` | 仅 COPY 三个产物 + `USER 1001:1001`；无 shell、无 apt、无 healthcheck | `scripts/start-k8s-console.sh`、`scripts/check_beta_artifacts.sh`（require_file） |

两者 COPY 的产物完全相同：`.docker-build/skytree`、`web/console/dist`、`etc/config.yaml`。差异只在基础镜像与是否带 healthcheck/shell。

### 目标

单一 `Dockerfile`，用命名构建阶段区分，消除重复的 COPY 段：

```dockerfile
# 公共产物层（被两个目标复用）
FROM scratch AS minimal
COPY --chown=1001:1001 .docker-build/skytree /app/skytree
COPY --chown=1001:1001 web/console/dist /app/web/console/dist
COPY --chown=1001:1001 etc/config.yaml /app/config.yaml
WORKDIR /app
USER 1001:1001
ENTRYPOINT ["/app/skytree"]

# 带运行时依赖与健康检查的完整镜像
FROM ubuntu:24.04 AS full
RUN apt-get update && apt-get install -y ca-certificates tzdata wget \
    && rm -rf /var/lib/apt/lists/*
RUN groupadd -g 1001 skytree && useradd -u 1001 -g skytree -s /bin/bash -m skytree
WORKDIR /app
COPY .docker-build/skytree /app/skytree
COPY web/console/dist /app/web/console/dist
COPY etc/config.yaml /app/config.yaml
RUN mkdir -p /app/data && chmod +x /app/skytree && chown -R skytree:skytree /app
USER skytree
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:9526/health || exit 1
ENTRYPOINT ["/app/skytree"]
```

构建方式：`docker build --target full .`（原 Dockerfile）/ `docker build --target minimal .`（原 Dockerfile.k8s）。

### 步骤

1. 写入合并后的多阶段 `Dockerfile`（含 `minimal`、`full` 两个 target）。
2. 更新引用：
   - `scripts/start-k8s-console.sh:210` 的 `-f Dockerfile.k8s` → `-f Dockerfile --target minimal`。
   - `scripts/k8s/dev.sh:12` 的默认 `DOCKERFILE` → 改为传 `--target full`（或保留默认 ubuntu 行为）。
   - `scripts/check_beta_artifacts.sh:34` 的 `require_file Dockerfile.k8s` → 改为校验 `Dockerfile` 含 `minimal` target（或直接删除该校验项）。
3. 删除 `Dockerfile.k8s`。
4. 更新 `.dockerignore` 如有针对性条目（当前看无需改）。

### 风险与缓解

- 风险中低：构建逻辑不变，只是合并与加 `--target`。`go build`/单测无关。
- **本环境无 Docker，无法实跑构建验证。** 需在本机执行 `docker build --target minimal .` 和 `--target full .` 各跑一次确认。
- `scratch` target 仍要求宿主预编译 `.docker-build/skytree` 与 `web/console/dist` 存在——与现状一致，无新增前置。

### 验证（本机）

- `docker build --target minimal -t skytree:min .` 成功，镜像可在本地 K8s 启动。
- `docker build --target full -t skytree:full .` 成功，`HEALTHCHECK` 生效。
- `make test-beta-assets`（即 `check_beta_artifacts.sh`）通过。

### 工作量

约 0.25 天（含本机两次构建验证）。

---

## 三、重新生成 proto，使生成代码对齐 `brokerapi`（需在本机执行）

### 背景

`pkg/broker` → `pkg/brokerapi` 改名时，发现两个生成文件的 protobuf descriptor 以**长度前缀**内嵌了 `go_package` 路径，直接 sed 会破坏 descriptor 导致运行时 `init` panic。当时的处理：

- `.proto` 源文件的 `go_package` **已更新**为 `brokerapi`（即未来重新生成的事实来源已正确）。
- 两个 `.pb.go` 的 descriptor 路径**临时还原**为旧 `pkg/broker`，以保证字节自洽、运行时正确。

因此目前状态是：源（`.proto`）说 `brokerapi`，生成物（`.pb.go` descriptor 元数据）仍说 `pkg/broker`。这是无害的元数据漂移，但需要一次重新生成来彻底对齐。

### 待对齐的生成物

descriptor 仍指向旧路径的文件：

- `pkg/brokerapi/grpc/nodepb/notify_client_delivery.pb.go`
- `pkg/brokerapi/publish/publishpb/message.pb.go`

对应 `.proto` 源（`go_package` 已是 `brokerapi`）：

- `pkg/brokerapi/grpc/nodepb/close_client.proto`、`notify_client_delivery.proto`
- `pkg/brokerapi/publish/publishpb/message.proto`

> 仓库未发现 `buf.yaml` 或专门的 proto 生成脚本（`gen_mock.sh` 系列是 mockgen，与 proto 无关），说明 proto 目前是手动用 `protoc` 生成的。下面命令需与团队既有 protoc/插件版本对齐。

### 步骤（本机，需装 `protoc` + `protoc-gen-go` + `protoc-gen-go-grpc`）

1. 确认插件版本与现有生成物兼容（避免引入大范围 diff）：
   ```bash
   protoc --version
   protoc-gen-go --version
   protoc-gen-go-grpc --version
   ```
2. 在仓库根目录重新生成这三组（含 gRPC 服务的 nodepb 需带 `--go-grpc_out`）：
   ```bash
   protoc \
     --go_out=. --go_opt=module=github.com/BAN1ce/skyTree \
     --go-grpc_out=. --go-grpc_opt=module=github.com/BAN1ce/skyTree \
     pkg/brokerapi/grpc/nodepb/close_client.proto \
     pkg/brokerapi/grpc/nodepb/notify_client_delivery.proto

   protoc \
     --go_out=. --go_opt=module=github.com/BAN1ce/skyTree \
     pkg/brokerapi/publish/publishpb/message.proto
   ```
   （`go_package` 已含 `;nodepb`/`;publishpb`，配合 `--go_opt=module=...` 会落到正确目录。若团队原先用 `paths=source_relative`，则改用对应写法并在各 proto 目录内生成。）
3. 生成后检查 diff：理想情况下只有两个 `.pb.go` 的 descriptor 内 `go_package` 从 `pkg/broker` 变为 `pkg/brokerapi`（及长度前缀随之变化），不应出现其他语义改动。
4. `go build ./...` 与相关测试通过：
   ```bash
   go build ./...
   go test ./internal/grpc/... ./internal/broker/... ./pkg/brokerapi/...
   ```

### 风险与缓解

- 主要风险是 protoc/插件版本与历史生成物不一致，导致大范围无关 diff。缓解：先在干净分支单独生成，仔细 review diff，必要时锁定与历史一致的插件版本。
- 这是唯一一项**本沙箱无法替你完成**的收尾（无 protoc），其余两项可在此协助执行。

### 工作量

约 0.25 天（主要是核对插件版本与 diff）。

---

## 总体排期与建议

| 顺序 | 项目 | 风险 | 可在沙箱验证 | 预估 |
| --- | --- | --- | --- | --- |
| 1 | proto 重新生成对齐 `brokerapi` | 低 | 否（需本机 protoc） | 0.25 天 |
| 2 | 合并 Dockerfile（多 target） | 中低 | 否（需本机 docker） | 0.25 天 |
| 3 | `app/` → `internal/app/` 收敛 | 中 | 是（go build/test） | 0.5 天 |

排期理由：proto 对齐是上一轮遗留收尾，优先做掉；Dockerfile 合并独立且小；双 internal 收敛 churn 最大、放最后单独成 PR。三项各自独立 commit，互不依赖。

执行前提：先把当前已完成的结构改动按主题拆成若干 commit 落库，保持工作区干净，再开始本计划，避免机械迁移与在途改动混淆。
