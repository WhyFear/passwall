# AGENT.md

This file provides guidance to Claude Code (claude.ai/code)/Codex when working with code in this repository.

## 常用命令

### 后端（Go）

后端代码位于 `passwall/`，Go module 名称为 `passwall`，当前声明 Go 版本为 1.24.4。

```powershell
# 安装/整理后端依赖
cd passwall; go mod tidy

# 运行后端（需要 CONFIG_PATH 指向配置文件，且必须设置 PASSWALL_TOKEN）
cd passwall; $env:CONFIG_PATH = "config.yaml"; $env:PASSWALL_TOKEN = "your_token"; go run ./cmd/server

# 构建后端服务
cd passwall; go build -o passwall-server ./cmd/server

# 运行全部 Go 测试
cd passwall; go test ./...

# 运行单个包测试
cd passwall; go test ./internal/adapter/parser

# 运行单个测试函数
cd passwall; go test ./internal/adapter/parser -run TestName
```

后端配置默认读取当前工作目录下的 `config.yaml`；也可通过 `CONFIG_PATH` 指定。`PASSWALL_TOKEN` 是必需环境变量。Scamalytics 相关配置由 `SCAMALYTICS_HOST`、`SCAMALYTICS_USER`、`SCAMALYTICS_API_KEY` 注入，缺失时会清空并记录日志。

### 前端（React）

前端代码位于 `web/`，使用 React 18、Ant Design 5 和 Create React App（react-scripts）。

```powershell
# 安装前端依赖
cd web; npm install

# 启动前端开发服务器
cd web; npm start

# 构建前端静态文件
cd web; npm run build

# 运行前端测试
cd web; npm test

# 运行单个前端测试（CRA/Jest 交互式模式支持按文件名或测试名过滤）
cd web; npm test -- --testNamePattern="test name"
```

前端 API 基础地址在 `web/src/config.js` 中配置，当前指向 `/web/api`（相对路径，生产环境应与后端同源）。`process.env.REACT_APP_API_BASE_URL` 可覆盖。

### Docker

```powershell
# 构建镜像
docker build -t passwall .

# 使用 compose 启动应用和 PostgreSQL
docker compose up -d
```

`Dockerfile` 会先构建 `web/`，再构建 `passwall/cmd/server`，运行镜像中静态资源位于 `/app/web/build`。`docker-compose.yml` 暴露 `8080:8080`，并通过环境变量设置 `PASSWALL_TOKEN` 和 `CONFIG_PATH`。

## 高层架构

### 后端启动流程

`passwall/cmd/server/main.go` 是服务入口：

1. `config.LoadConfig()` 读取 YAML 和环境变量。
2. `repository.InitDB()` 使用 GORM 初始化数据库。
3. `service.NewServices()` 组装仓储、解析器、生成器、测速器、任务管理、订阅、代理、配置、IP 检测和流量统计服务。
4. `scheduler.NewScheduler()` 根据系统配置和订阅配置注册 cron 任务。
5. `api.SetupRouter()` 创建 Gin 路由并挂载 API、Web API 和前端静态文件。

服务关闭时，入口会等待 `SIGINT`/`SIGTERM`，停止调度器；如果启用 Clash API 流量统计，也会停止统计服务。

### 配置与认证

配置结构定义在 `passwall/config/config.go`。配置文件负责 server、database、proxy、ip_check、clash_api、cron_jobs、default_sub 等字段；敏感 token 不来自 YAML，而是来自 `PASSWALL_TOKEN`。

认证中间件位于 `passwall/api/middleware/auth.go`：

- `/api` 下的开放订阅接口使用 query token 认证。
- `/api/v1` 和 `/web/api` 使用 `Authorization: Bearer <token>`。

前端 token 存在 localStorage，由 `web/src/api/index.js` 的 axios 请求拦截器注入 Authorization header；401 响应会清空 token 并触发 token 弹窗。

### API、服务和数据层

路由集中在 `passwall/api/router.go`。Handler 位于 `passwall/api/handler/`，只负责 HTTP 入参/响应和调用 service；核心业务逻辑在 `passwall/internal/service/`，数据访问在 `passwall/internal/repository/`，持久化模型在 `passwall/internal/model/`。

`service.NewServices()` 是后端依赖装配中心：

- parser factory 注册 `share_url` 和 `clash` 解析器，代码在 `passwall/internal/adapter/parser/`。
- generator factory 注册 `clash` 和 `share_link` 生成器，代码在 `passwall/internal/adapter/generator/`。
- speed tester factory 注册 Clash core 测速器，代码在 `passwall/internal/adapter/speedtester/`。
- task manager 在 `passwall/internal/service/task/` 中管理异步任务状态和资源访问冲突检测。
- 订阅与代理服务主要在 `passwall/internal/service/proxy/` 中。
- IP 信息与流媒体解锁检测在 `passwall/internal/detector/` 和 `passwall/internal/service/ip_detector.go` 中。

数据库初始化在 `passwall/internal/repository/database.go`：

- `sqlite` 模式会对主要模型执行 AutoMigrate，并设置 WAL/busy_timeout 等 PRAGMA。
- `postgres` 模式只自动迁移 `SubscriptionConfig` 和 `SystemConfig`，避免干扰已有业务表结构。

### 任务管理器与资源访问控制

`passwall/internal/service/task/task_manager.go` 管理全局任务和资源级任务的生命周期。关键接口 `TaskManager` 提供 `StartTaskWithSpec`、`CancelTask`、`FinishTask`、`GetAllStatus` 等方法。

**资源访问声明**：通过 `TaskSpec.Accesses` 声明任务所需资源和访问模式（`AccessModeRead` / `AccessModeWrite`）。预定义资源有 `ResourceProxies`、`ResourceSubscriptions`、`ResourceSpeedHistory`、`ResourceIPDetection`。启动时，有冲突（同资源写-写或读-写）的活跃任务会拒绝新任务。

**哨兵错误**：`ErrTaskConflict`（资源访问冲突）和 `ErrTaskAlreadyRunning`（同类型任务已在运行）。Handler 应使用 `errors.Is()` 判断，不要用字符串匹配错误消息。

**TaskRun 包装器**：`StartRunWithSpec` 返回 `*TaskRun`，封装进度累加（`IncrementProgress`）和幂等完成（`Finish`/`FinishWithContextMessage`）。

**测试支持**：`NewTaskManagerWithTimeout` 创建带自定义取消超时的任务管理器，避免并行测试下修改包级变量。

**并发策略**：同一资源的只读操作允许并发；涉及写入的任务互斥执行。详见 `internal/service/task/task_manager.go:hasConflictingAccessLocked`。

### 调度器

`passwall/internal/scheduler/scheduler.go` 使用 `robfig/cron/v3` 且启用秒级 cron 表达式。调度器会加载全局 `cron_jobs`，并为启用自动更新的订阅配置创建独立任务。修改系统配置或订阅配置时，相关 service 会把 scheduler 注入后触发重载或更新任务。

`cron_job_executor.go` 负责执行单个 cron job 的完整流程：依次执行测速、自动封禁、IP 检测、webhook。panic 恢复时会遍历 `GetAllStatus()` 清理所有活跃任务（全局和资源级）。

### 代理服务子模块

`passwall/internal/service/proxy/` 包含多个职责分离的文件：

- `proxy_service.go` — 对外代理操作（筛选查询、封禁、置顶等）。
- `tester.go` — 代理测速（单代理和批量，支持并发限制和取消）。
- `proxy_syncer.go` — 订阅解析后的代理同步（增/改/跳过/去重）。
- `subscription_refresher.go` — 订阅刷新（下载、解析、同步、触发后续测试）。
- `subscription_status.go` — 订阅状态标记（有效/无效）和同步结果日志。

### IP 检测

`passwall/internal/service/ip_detector.go` 提供单体检测（`Detect`）和批量检测（`BatchDetect`）。批量检测支持并发控制、取消，失败数量会汇总到任务完成消息。检测结果通过 `ip_detect_persister.go` 持久化（IP 地址、基础信息、解锁信息）。

### 前端结构

`web/src/index.js` 挂载 React 应用，`web/src/App.js` 定义整体布局、菜单和路由：

- `/`：订阅链接页，页面文件 `web/src/pages/SubscriptionPage.js`。
- `/nodes`：节点列表页，页面文件 `web/src/pages/NodesPage.js`。
- `/config`：系统配置页，页面文件 `web/src/pages/ConfigPage.js`。

节点列表页拆分为多个模块，位于 `web/src/pages/nodes/`：
- `useNodesQuery.js` — 数据请求 hook（支持 AbortController 取消）。
- `nodeColumns.js` — 表格列定义和列设置菜单生成。
- `nodeFormatters.js` — 速度、流量、风险等格式化函数。
- `nodeTags.js` — 状态标签和状态信息组件。
- `nodeQueryUtils.js` — 查询参数构建工具。
- `NodeBatchActions.js` — 批量操作区域组件。
- `NodeDetailModal.js` — 节点详情弹窗组件。

API 封装集中在 `web/src/api/index.js`，导出 `subscriptionApi`、`nodeApi`、`taskApi`、`configApi`。通用组件在 `web/src/components/`，token、cron、时间和任务相关工具在 `web/src/utils/`。

## 项目特定注意事项

- 仓库根目录的 `config.yaml.example` 是 Docker/部署示例；`passwall/config.yaml` 是后端本地运行时默认配置文件。
- 当前仓库包含已安装的 `web/node_modules/`，代码搜索和架构梳理时通常应排除它。
- Gin 会在后端中服务 `./web/build`，生产构建或 Docker 镜像需要先生成前端 build。
- 后端 cron 表达式使用 6 字段（含秒），例如 `0 0 6,18 * * *`。
- Handler 与 service 之间的错误传递应使用 sentinel error（如 `task.ErrTaskConflict`）+ `errors.Is()`，不要依赖字符串匹配。
- 任务访问冲突需要区分错误类型：`ErrTaskAlreadyRunning`（同类型任务已在运行）和 `ErrTaskConflict`（资源访问冲突），两者在 HTTP 层都返回 "task running" 语义。
- 前端 `useNodesQuery` 使用 `AbortController`：新请求自动取消旧请求，组件卸载时取消 in-flight 请求。向 `api.getProxies` 传递 `signal` 参数。
