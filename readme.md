# Docker版 aistudio-build-proxy

集成 无头浏览器 + WebSocket代理 + 实时日志查看器

**主要特性:**

- 完整的 Gemini API 代理支持（包括 Function Calling/Tool 调用）
- 基于 WebSocket 的请求隧道，绕过 CORS 限制
- Camoufox 指纹浏览器自动化，保持会话稳定性
- 实时日志查看 Web UI（http://localhost:5345/logs-ui/）
- 自动修复 Roo/Cline 等客户端的请求格式问题
- 模块化 Go 代码架构，易于维护和扩展
- **WebSocket代理模式**：使用浏览器端 WebSocket 客户端替代易出错的 DOM 操作

问题: ~~当前cookie导出方式导出的cookie可能时效较短.~~ 指纹浏览器导出cookie很稳

## 使用方法:

### 前置步骤：部署WebSocket代理客户端到Gemini AI Studio

本项目的核心是WebSocket代理架构，需要先将 `127-of-websocket-proxy-logger/` 中的React应用部署到Gemini AI Studio Build功能中。

**部署步骤：**

1. 访问 [Gemini AI Studio](https://ai.google.dev/aistudio)
2. 创建新的Build应用（或使用现有应用）
3. 将 `127-of-websocket-proxy-logger/` 目录中的所有文件上传到Build项目：
   - `App.tsx`, `index.tsx`, `index.html`
   - `services/webSocketService.ts`
   - `types.ts`, `config.ts`
   - `package.json`, `vite.config.ts`, `tsconfig.json`
4. 在 `config.ts` 中配置：
   ```typescript
   export const JWT_TOKEN: string | null = "valid-token-user-1";
   export const WEBSOCKET_PROXY_URL: string = "ws://127.0.0.1:5345/v1/ws";
   ```
5. Build会在浏览器中运行此应用，应用启动后会自动连接到本地的Go代理服务器

**工作原理：**

- Gemini Build在浏览器中运行你的React应用
- 应用继承浏览器的登录状态（cookie）
- 应用通过WebSocket连接到本地Go服务器
- Go服务器将API请求转发给应用，应用代表浏览器执行真实的Gemini API调用

### 主要配置步骤

1. 导出Cookie到项目`camoufox-py/cookies/`文件夹下

   #### 更稳定的方法：

   用指纹浏览器开个新窗口登录 google, 然后到指纹浏览器`编辑窗口`，把 cookie 复制出来用，然后删除浏览器窗口就行，这个 cookie 超稳！！！

    <details>
       <summary>旧方法（不再推荐）：cookie很容易因为主账户的个人使用活动导致导出的cookie失效。</summary>
    (1) 安装导出Cookie的插件, 这里推荐 [Global Cookie Manager浏览器插件](https://chromewebstore.google.com/detail/global-cookie-manager/bgffajlinmbdcileomeilpihjdgjiphb)
    
    (2) 使用插件导出浏览器内所有涉及`google`的Cookie
    
    导出Cookie示例图:
    ![Global Cookie Manager](/img/Global_Cookie_Manager.png)
    ![Global Cookie Manager2](/img/Global_Cookie_Manager2.png)
    
    (3) 粘贴到项目 `camoufox-py/cookies/[自己命名].json` 中
    </details>

2. 修改浏览器配置`camoufox-py/config.yaml`

   (1) 在`camoufox-py`下, 将示例配置文件`config.yaml.example`, 重命名为 `config.yaml`, 然后修改`config.yaml`

   (2) 实例 1 的`cookie_file` 填入自己创建 cookie文件名

   (3) (可选项) `url` 默认为项目提供的AIStudio Build 链接(会连接本地5345的ws服务), 可修改为自己的

   (4) (可选项) proxy配置指定浏览器使用的代理服务器

3. 修改`docker-compose.yml`

   (1) 自己设置一个 `AUTH_API_KEY` , 最后自己调 gemini 时要使用该 apikey 调用, 不支持无 key

4. 在项目根目录, 通过`docker-compose.yml`启动Docker容器

   (1) 运行命令启动容器

   ```bash
   docker compose up -d
   ```

5. 等待一段时间后, 通过 http://127.0.0.1:5345 和 自己设置的`AUTH_API_KEY`使用.

   注1: 由于只是反代Gemini, 因此[接口文档](https://ai.google.dev/api)和Gemini API: `https://generativelanguage.googleapis.com`端点 完全相同, 使用只需将该url替换为`http://127.0.0.1:5345`即可, 支持原生Google Search、代码执行等工具。

   注2: Cherry Studio等工具使用时, 务必记得选择提供商为 `Gemini`。

## 日志查看

### 1. Web UI 实时日志查看器（推荐）

访问 **http://localhost:5345/logs-ui/** 查看实时日志，功能包括：

- 实时显示所有请求/响应
- 按日志级别筛选（ERROR/WARN/INFO/DEBUG）
- 搜索功能
- 查看完整的请求体、响应体和转换详情
- 显示工具定义转换和移除的字段
- 自动刷新（可手动关闭）
- 下载日志为 JSON

### 2. Docker 日志

```bash
docker logs [容器名]
# 或持续查看
docker compose logs -f
```

### 3. 单独查看 camoufox-py 日志

camoufox-py/logs/app.log

且每次运行, logs下会有一张截图

## 容器资源占用:

![Containers Stats](/img/Containers_Stats.png)
本图为仅使用一个cookie的占用

## 运行效果示例:

快速模型首字吐出很快,表明该代理网络较好,本程序到google链路通畅

![running example](/img/running_example.gif)

如果使用推理模型慢,那就是 aistudio 的问题, 和本项目没关系

## 技术架构

### WebSocket代理模式详解

本项目采用 **WebSocket隧道** 架构，通过在浏览器中运行的React应用作为请求代理，替代了传统的DOM操作方式。

#### 为什么使用WebSocket代理？

传统的DOM代理方式存在以下问题：

- **脆弱性高**：依赖页面DOM结构，任何页面更新都可能导致代理失效
- **维护困难**：需要频繁适配Google页面变化
- **调试困难**：DOM操作难以追踪和调试
- **可靠性差**：容易因页面加载时序问题导致失败

WebSocket代理模式的优势：

- **架构清晰**：请求流程完全可控，不依赖页面DOM
- **稳定可靠**：不受Google页面更新影响
- **易于调试**：所有请求和响应都经过结构化的消息传递
- **性能优秀**：支持流式响应，实时双向通信
- **扩展性强**：可轻松添加请求拦截、修改、日志等功能

#### WebSocket代理客户端 (127-of-websocket-proxy-logger/)

这是一个React + TypeScript应用，通过 **Gemini AI Studio Build** 功能部署到浏览器中运行。

**核心功能：**

1. **WebSocket连接管理** (`services/webSocketService.ts:307-349`)
   - 自动连接到Go代理服务器的WebSocket端点 (`ws://127.0.0.1:5345/v1/ws`)
   - JWT token认证
   - 自动重连机制（指数退避 + 抖动）
   - 心跳保活（25秒ping间隔）

2. **HTTP请求代理** (`services/webSocketService.ts:70-195`)
   - 接收来自Go服务器的 `http_request` 消息
   - 使用浏览器的 `fetch()` API发起实际请求到Gemini API
   - 支持流式和非流式响应
   - 自动处理 `stream_start` / `stream_chunk` / `stream_end` 消息

3. **实时日志UI** (`App.tsx`)
   - 显示WebSocket连接状态
   - 记录所有请求/响应事件
   - 提供连接/断开控制
   - 自动滚动和日志清理功能

**部署方式：**

通过Gemini AI Studio的Build功能，将此React应用部署到指纹浏览器中。浏览器自动加载cookie并运行此应用，使其能够：

- 以登录用户身份访问Gemini API
- 绕过CORS限制（同源请求）
- 通过WebSocket与外部Go服务器通信

**配置文件：**

- `config.ts:18,26` - 配置JWT token和WebSocket URL
- `types.ts` - 定义WebSocket消息协议

### 请求流程

```
客户端（Roo/Cline等） → Go 代理服务器 → WebSocket → 浏览器客户端（React App） → Gemini API
                      ↑                                                              ↓
                      └──────────────────────── WebSocket ←──────────────────────────┘
```

**详细流程：**

1. 客户端发送HTTP请求到Go服务器 (`:5345/v1beta/...`)
2. Go服务器认证请求（检查API Key）
3. Go服务器将HTTP请求封装为WebSocket消息 (`golang/proxy.go:68-78`)
4. 通过WebSocket发送 `http_request` 消息到浏览器客户端
5. 浏览器客户端接收消息并执行 `fetch()` 调用
6. 浏览器客户端处理响应：
   - 非流式：发送 `http_response` 消息
   - 流式：发送 `stream_start` → `stream_chunk` → `stream_end` 消息序列
7. Go服务器接收WebSocket消息并构建HTTP响应 (`golang/proxy.go:109-315`)
8. 返回最终响应给客户端

### 模块说明

#### Go 代理服务器 (golang/)

- **main.go** - 主程序入口，HTTP 路由配置
- **pool.go** - WebSocket 连接池管理，负载均衡（Round-robin）
- **websocket.go** - WebSocket 消息处理，心跳机制
- **proxy.go** - HTTP 代理逻辑，流式响应处理
- **transformers.go** - 请求转换器：
  - 修复 `functionDeclarations` → `function_declarations`（camelCase → snake_case）
  - 修复 `parametersJsonSchema` → `parameters`
  - 移除 Gemini 不支持的 OpenAPI 字段：`additionalProperties`, `default`, `optional`, `maximum`, `oneOf`
  - 移除 `systemInstruction` 中的无效 `role` 字段
  - 转换 `thinkingLevel` → `thinkingBudget`
- **logging.go** - 日志缓冲区管理（循环缓冲，1000条）

#### WebSocket代理客户端详细说明 (127-of-websocket-proxy-logger/)

这不仅是日志查看器，更是整个代理架构的核心组件：

**双重角色：**

1. **WebSocket客户端**：作为浏览器端代理，实际执行对Gemini API的请求
2. **调试UI**：提供实时日志和连接状态监控

**技术栈：**

- React + TypeScript
- Vite构建工具
- WebSocket原生API
- Fetch API用于HTTP请求代理

**关键实现：**

- `services/webSocketService.ts` - 完整的WebSocket客户端实现
  - 连接管理、心跳保活、自动重连
  - HTTP请求代理逻辑
  - 流式响应处理
- `App.tsx` - UI组件，显示连接状态和日志
- `config.ts` - WebSocket URL和JWT token配置
- `types.ts` - WebSocket消息协议定义（与Go服务器对应）

**消息协议：**

- 服务器→客户端：`http_request`, `pong`
- 客户端→服务器：`http_response`, `stream_start`, `stream_chunk`, `stream_end`, `error`, `ping`

#### 日志查看器 (log-viewer/)

- React + Vite 构建的实时日志 Web UI
- 实时轮询显示Go代理服务器的结构化日志
- 按级别筛选、搜索、导出功能
- 独立于WebSocket代理客户端，通过HTTP接口获取日志

#### Python 浏览器自动化 (camoufox-py/)

- 使用 Camoufox（基于 Firefox 的指纹浏览器）
- 自动加载 Cookie，保持登录状态
- 处理 Google 认证页面和弹窗
- 通过 WebSocket 接收请求并转发到 Gemini API

### API 兼容性

本项目完全兼容 Gemini API 官方接口，支持：

- ✅ 文本生成（包括流式和非流式）
- ✅ Function Calling / Tool 使用
- ✅ 代码执行工具
- ✅ Google Search 工具
- ✅ 思考模式（Thinking）
- ✅ 系统指令
- ✅ 多轮对话

### 已知问题与解决方案

**问题**: Roo/Cline 发送的工具定义格式与 Gemini API 要求不一致

**解决方案**: `transformers.go` 自动修复以下问题：

1. 字段命名转换（camelCase → snake_case）
2. 移除不支持的 JSON Schema 字段
3. 修复 systemInstruction 和 thinkingConfig 格式

所有转换都会记录在日志查看器中，方便调试和验证。
