# Passwall

Passwall是一个代理服务器管理工具，支持导入、测速和提供订阅。

## 三大功能

**导入服务器配置/订阅源：**

1. clash订阅配置文件
2. v2ray/xray订阅配置文件
3. raw配置（vless、vmess、trojan）

**测速能力，支持定时测试，调用接口触发测试**

1. xray
2. mihomo（支持多种协议）
3. trojan

**提供多种订阅配置**

1. clash
2. v2ray/xray配置文件

## 快速开始

### 安装依赖

```bash
go mod download
```

### 配置

编辑`config.yaml`文件，设置数据库、服务器和定时任务等配置。

### 运行

```bash
go run cmd/server/main.go
```

## API接口

### 创建订阅源

**路径:** `/api/create_proxy`

**方法:** POST

**参数:**

| 参数 | 格式       | 是否必填                             | 示例                                                     |
| ------ | ------------ | -------------------------------------- | ---------------------------------------------------------- |
| url  | string     | 与file二选一必填，二者都填以file为准 | socks://Og\=\=@dsm.893843891.xyz:7890#DSM-mihomo |
| file | 二进制文件 | 与url二选一必填，二者都填以file为准  | 二进制文件                                              |
| type | string     | 必填                                 | clash、v2ray、trojan                                     |

### 触发测速

**路径:** `/api/test_proxy_server`

**方法:** POST/GET

**参数:**

| 参数                           | 格式          | 是否必填 | 示例                                                                      |
| -------------------------------- | --------------- | ---------- | --------------------------------------------------------------------------- |
| reload\_subscrib\_config | bool          | 否       | 默认为false，为true则会重新处理所有订阅配置文件                           |
| test\_all                   | bool          | 否       | 默认为**true**，为true则会重新对所有节点多线程测速，参数优先级最高                |
| test\_failed                | bool          | 否       | 默认为false，为true则只对status\=2和3的节点多线程测速，参数优先级第二  |
| test\_speed                 | bool          | 否       | 默认为false，为true则对所有status\=1节点进行单进程测速，参数优先级第三 |
| concurrent                     | int（\>1） | 否       | 默认为5线程测速，test\_speed为true时不生效                             |

### 获取订阅链接

**路径:** `/api/subscribe`

**方法:** GET

**参数:**

| 参数   | 格式   | 是否必填 | 示例                                                     |
| -------- | -------- | ---------- | ---------------------------------------------------------- |
| token  | string | 是       | 配置文件中配置的token                                    |
| type   | string | 否       | share\_url、clash、v2ray。默认为share\_url分享链接 |
| status | list   | 否       | [0,1,2] 返回指定状态的服务器                             |

## 项目结构

```
passwall/
├── api/                      # API接口层
│   ├── middleware/           # 中间件
│   └── handler/              # 请求处理器
├── cmd/                      # 应用入口
│   └── server/               # 服务器入口
├── config/                   # 配置管理
├── internal/                 # 内部包
│   ├── model/                # 数据模型
│   ├── service/              # 业务服务
│   ├── adapter/              # 适配器
│   │   ├── parser/           # 解析器
│   │   ├── tester/           # 测速工具
│   │   └── generator/        # 配置生成器
│   ├── repository/           # 数据仓库
│   └── scheduler/            # 任务调度器
└── pkg/                      # 公共包
    └── utils/                # 工具函数
```

## 许可证

MIT 