# 代理测试服务架构

## 架构说明

代理测试服务已经进行了重构，采用了更加模块化和职责分明的设计：

### 核心服务

- **proxy/tester.go**：代理测试核心服务，负责测试代理的连接和速度
- **proxy/subscription.go**：订阅管理服务，负责刷新订阅和解析代理配置
- **proxy/speedtest.go**：速度测试服务，负责管理代理速度测试结果
- **task/manager.go**：任务管理器，负责管理异步任务的执行和状态

### 适配器

- **proxy_tester_adapter.go**：兼容旧版 ProxyTester 接口的适配器，保证向后兼容性

## 目录结构

```
service/
  ├── proxy/              # 代理相关服务
  │   ├── tester.go       # 代理测试服务
  │   ├── subscription.go # 订阅管理服务
  │   └── speedtest.go    # 速度测试服务
  ├── task/               # 任务相关服务
  │   └── manager.go      # 任务管理器
  ├── proxy_tester_adapter.go    # 适配器
  └── service.go          # 服务初始化
```

## 设计优势

1. **单一职责**：每个服务专注于单一功能，代码更加清晰
2. **依赖注入**：通过构造函数注入依赖，便于测试和维护
3. **接口隔离**：使用小型、专注的接口定义组件间交互
4. **错误处理一致**：统一的错误处理策略
5. **上下文传递**：使用 context 进行上下文传递和取消操作

## 使用方式

可以通过两种方式使用新架构：

1. **旧版接口**：通过 `ProxyTesterAdapter` 继续使用旧版接口（推荐用于兼容现有代码）
2. **新版接口**：直接使用新的服务接口，获得更细粒度的控制

### 示例：使用旧接口（通过适配器）

```go
// 通过 service.go 获取服务
services := service.NewServices(db)
proxyTester := services.ProxyTester

// 测试所有代理
request := &service.TestProxyRequest{
    TestAll: true,
    Concurrent: 5,
}
err := proxyTester.TestProxies(request)
```

### 示例：使用新接口

```go
// 直接使用新接口
taskManager := task.NewManager()
proxyTester := proxy.NewTester(
    proxyRepo,
    speedTestHistoryRepo,
    speedTesterFactory,
    taskManager,
)

// 创建测试请求
request := &proxy.TestRequest{
    Filters: &proxy.ProxyFilter{
        Status: []model.ProxyStatus{model.ProxyStatusOK},
    },
    Concurrent: 5,
}

// 执行测试
ctx := context.Background()
err := proxyTester.TestProxies(ctx, request)
```

## 迁移计划

1. 短期内保持旧版接口兼容性，使用适配器
2. 在新功能中使用新接口
3. 逐步将现有代码迁移到新接口
4. 最终移除旧版代码和适配器 