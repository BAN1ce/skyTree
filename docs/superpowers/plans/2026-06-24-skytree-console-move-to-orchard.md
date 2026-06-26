# SkyTree Console 迁移到 Orchard 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 `skyTree` 仓库中的 Web Console 完整迁移到 `orchard` 管理后台，使 `skyTree` 仅保留可供上游调用的运维 API 与控制能力，不再承载任何浏览器页面或前端静态资源。

**Architecture:** `orchard/frontend/console` 新增 `SkyTree Operations` 模块承接现有运维页面；`orchard/backend` 新增受 Orchard JWT 保护的 BFF/代理层，对接单个已配置的 SkyTree 上游；`skyTree` 删除 `/console` 页面路由、静态资源打包与前端目录，但保留当前运维数据与节点控制 API 能力，供 Orchard 调用。

**Tech Stack:** Go, Gin, React, TypeScript, Vite, Ant Design, TanStack Query, JWT, HTTP proxy/BFF

---

## 摘要

- 目标形态是 `Orchard Console + Orchard Backend BFF + SkyTree 纯 API`。
- 本次范围按全量迁移处理：`overview`、`clients`、`sessions`、`subscriptions`、`will-delay`、`retain`、`cluster`、`metrics` 全部迁入 `orchard/frontend/console`。
- 认证统一走 Orchard 登录态；浏览器不再直接访问 SkyTree 的 Basic Auth console。
- 当前只支持单个 SkyTree 集群；`orchard/backend` 通过配置指向一个固定上游。

## 关键接口与边界变更

- `skyTree`
  - 保留 `/api/v1/console/*` 这组运维 API 的能力与数据结构，作为 Orchard 上游依赖。
  - 删除 `/console`、`/console/`、`/console/assets/*` 及对应的 `NoRoute` 前端回退逻辑。
  - 删除 `web/console` 目录及其构建、镜像打包、K8s 本地脚本、beta 检查中与静态资源相关的依赖。
  - `config.Console` 从“读写 Web Console 配置”收敛为“运维 API / 上游控制能力配置”；如无必要，不再要求 `username/password` 仅为页面服务。
- `orchard/backend`
  - 新增一组受 JWT 保护的 SkyTree 管理 API，建议收口在 `/api/v1/skytree/*`。
  - 新增单集群上游配置，例如：`base_url`、超时、可选上游凭证、节点控制开关。
  - 新增 SkyTree 上游客户端与服务层，负责请求转发、错误映射、超时、响应结构标准化。
  - 对 `cluster node action` 这类写操作继续透传，但必须挂在 Orchard 登录鉴权之后。
- `orchard/frontend/console`
  - 在现有侧边导航中增加 `SkyTree Operations` 分组或一级菜单。
  - 将 `skyTree/web/console` 的数据模型、查询逻辑、视图布局迁入 Orchard UI 体系。
  - 前端 API 全部改调 Orchard backend 的 `/api/v1/skytree/*`，不再直连 SkyTree。

## 实施步骤

### 阶段 1：稳定 SkyTree API 作为上游契约

- 盘点 `skyTree/api/console.go` 暴露的所有只读/控制接口，冻结字段名、状态码和错误语义。
- 将页面专属逻辑与 API 专属逻辑拆开：
  - API 路由注册保留。
  - HTML 静态资源、Basic Auth 页面入口、fallback 路由改为可删除或默认关闭。
- 检查 `skyTree/api/console_test.go`，把“页面可访问”测试拆成“API 可访问”与“页面已移除/404”两个目标测试。
- 明确 `skyTree` 配置语义：
  - 保留集群控制客户端相关配置。
  - 将仅服务于浏览器页面的说明文案、示例配置、脚本提示移除或改写。

### 阶段 2：在 Orchard backend 建立 SkyTree BFF

- 在 `orchard/backend/internal/config` 中新增 SkyTree 上游配置结构，至少包含：
  - 上游 `base_url`
  - 请求超时
  - 可选上游认证信息
  - 是否允许节点控制动作
- 新增独立的 SkyTree client/service 模块，封装：
  - `summary`
  - `clients` / `client detail`
  - `subscriptions tree`
  - `share group members`
  - `retain`
  - `will-delay`
  - `cluster nodes`
  - `cluster node actions`
- 在 `orchard/backend/internal/transport/http/router.go` 新增 `/api/v1/skytree/*` 路由组：
  - 全部走现有 JWT `authMiddleware`
  - 返回风格对齐 Orchard 现有 JSON 结构
  - 将上游错误统一映射成 Orchard 可读错误
- 明确后端职责：
  - Orchard 负责鉴权、上游配置、错误治理、未来权限扩展。
  - SkyTree 负责实时运行态数据与节点控制实现。

### 阶段 3：迁移前端到 Orchard Console

- 在 `orchard/frontend/console` 中新增 SkyTree 运维导航与页面壳子。
- 迁移 `skyTree/web/console` 现有页面与接口类型，保留原有信息架构，减少一次性重设计风险。
- 将所有 fetch/query 入口改为 Orchard backend：
  - 从 `/api/v1/console/*` 改为 `/api/v1/skytree/*`
  - 去掉浏览器侧 Basic Auth 假设
  - 保留查询参数与表格/详情页交互习惯
- 统一 Orchard UI 语言与导航：
  - 与现有 tenants/products/devices 模块共享 `Layout`
  - 新增运维入口，不再维持独立应用风格
- `metrics` 页面若当前仅展示固定 `/metrics` 路径信息，优先迁成“链接/说明 + 状态展示”；若依赖更多运行数据，则经 BFF 补足。

### 阶段 4：删除 SkyTree Web 承载物并清理部署脚本

- 删除 `skyTree/web/console` 目录。
- 清理 `skyTree/Dockerfile`、`Dockerfile.k8s` 中对 `web/console/dist` 的复制。
- 清理 `skyTree/scripts/start-k8s-console.sh`、`scripts/check_beta_artifacts.sh`、`Makefile` 中的前端构建与页面访问逻辑。
- 清理 `skyTree/deploy/k8s`、示例配置、runbook、README 中对 `http://.../console` 的文档引用。
- 如 `orchard/deploy` 需要同时发布新运维页面，补充 `orchard` 的部署文档与环境变量说明。

## 测试与验收

- `skyTree` 验收
  - `/api/v1/console/*` 在启用配置下仍可返回原有数据结构。
  - `/console` 与静态资源路径不再提供页面内容。
  - cluster node action 在 Orchard 上游调用场景下仍可工作。
  - 删除前端后，镜像构建、核心单测、console API 测试通过。
- `orchard/backend` 验收
  - 未登录访问 `/api/v1/skytree/*` 返回 401。
  - 已登录访问可透传 summary、clients、retain、cluster 等全部能力。
  - SkyTree 上游超时、401、5xx 时，Orchard 返回明确错误而不是原样泄漏。
  - 写操作接口在上游禁用或配置缺失时有清晰报错。
- `orchard/frontend/console` 验收
  - 通过 Orchard 登录后，可在统一后台访问全部 SkyTree 运维页面。
  - 各页面查询、详情、筛选、节点操作与旧版 console 行为一致或更清晰。
  - 刷新页面后不会丢失路由；移动端至少保证不崩溃、桌面端信息完整可操作。
- 回归场景
  - Orchard 原有 tenants/products/devices/quota 功能不受影响。
  - SkyTree 在完全不构建前端资源时仍可正常启动与提供 broker/API 能力。

## 默认假设

- 本次只支持一个固定的 SkyTree 集群，不做多集群切换器。
- Orchard 是唯一浏览器入口；SkyTree 不再面对终端用户提供 Web 页面。
- SkyTree 现有 `/api/v1/console/*` 字段名尽量保持不变，避免前后端双向重写。
- 不在本次引入细粒度 RBAC；先复用 Orchard 现有登录态，后续再细化权限。
- 如果发现 `skyTree` 的 Basic Auth 同时被外部自动化脚本依赖，则保留为上游机读认证配置，但不再暴露页面用途。
