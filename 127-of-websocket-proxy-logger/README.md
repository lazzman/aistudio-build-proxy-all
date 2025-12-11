# WebSocket Proxy Logger - 浏览器端代理客户端

这是一个运行在浏览器中的WebSocket客户端应用，它是整个aistudio-build-proxy架构的核心组件。

## 概述

本应用通过 **Gemini AI Studio Build** 功能部署到浏览器中运行，作为WebSocket代理的客户端，负责：

1. **接收HTTP请求**：通过WebSocket从Go代理服务器接收HTTP请求消息
2. **执行实际请求**：使用浏览器的 `fetch()` API向Gemini API发起请求
3. **返回响应**：将响应通过WebSocket发送回Go服务器
4. **提供调试UI**：实时显示连接状态和请求/响应日志

## 为什么需要这个客户端？

### 问题背景

直接从外部服务器调用Gemini API会遇到以下问题：

- 需要有效的Google账号cookie
- 受CORS策略限制
- Cookie管理复杂且易失效

### 解决方案：WebSocket隧道

通过在浏览器中运行此客户端：

- ✅ 继承浏览器的登录状态（cookie自动携带）
- ✅ 绕过CORS（同源请求）
- ✅ 稳定可靠（不依赖DOM结构）
- ✅ 易于调试（结构化消息传递）

## 架构优势：vs DOM代理方式

### 传统DOM代理方式的问题

早期版本可能使用DOM操作来发送请求：

```javascript
// 易出错的DOM操作
document.querySelector("#api-input").value = requestBody;
document.querySelector("#send-button").click();
// 等待并解析DOM中的响应...
```

**问题：**

- 依赖页面DOM结构，Google页面更新会导致代理失效
- 难以处理流式响应
- 调试困难，缺少结构化日志
- 可靠性差，时序问题多

### WebSocket代理方式的优势

```typescript
// 清晰的消息协议
webSocketProxyManager.connect(jwtToken);
// 接收请求 → 执行fetch → 返回响应
```

**优势：**

- 架构清晰，消息协议明确
- 完整支持流式响应
- 易于调试和监控
- 不受页面更新影响

## 技术实现

### 核心模块

#### 1. WebSocket连接管理 (`services/webSocketService.ts`)

**功能：**

- 连接到Go代理服务器 (`ws://127.0.0.1:5345/v1/ws`)
- JWT token认证
- 自动重连（指数退避+抖动）
- 心跳保活（25秒ping间隔）

**关键实现：**

```typescript
// 连接初始化
function connect(jwtToken: string) {
  const wsUrl = `${BASE_WEBSOCKET_URL}?auth_token=${jwtToken}`;
  socket = new WebSocket(wsUrl);
  socket.onopen = onSocketOpen;
  socket.onmessage = onSocketMessage;
  // ...
}

// 心跳保活
function startPing() {
  pingIntervalId = window.setInterval(() => {
    sendToServer({ type: "ping" });
  }, PING_INTERVAL_MS);
}
```

#### 2. HTTP请求代理 (`services/webSocketService.ts:70-195`)

**功能：**

- 接收 `http_request` 消息
- 使用 `fetch()` API执行请求
- 处理流式和非流式响应
- 错误处理和上报

**流式响应处理：**

```typescript
async function handleHttpRequest(request: WSHttpRequestMessage) {
  const response = await fetch(url, fetchOptions);

  if (response.body && response.body.getReader) {
    // 流式响应
    sendToServer({ type: "stream_start", payload: { status, headers } });

    const reader = response.body.getReader();
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      sendToServer({ type: "stream_chunk", payload: { data: chunkData } });
    }

    sendToServer({ type: "stream_end", payload: {} });
  } else {
    // 非流式响应
    const body = await response.text();
    sendToServer({ type: "http_response", payload: { status, headers, body } });
  }
}
```

#### 3. 调试UI (`App.tsx`)

**功能：**

- WebSocket连接状态指示器
- 连接/断开控制按钮
- 实时日志显示
- 自动滚动和日志清理

**UI特性：**

- 彩色日志级别（error/warn/info/status）
- 毫秒级时间戳
- 自动重连提示
- JWT token缺失警告

### 消息协议 (`types.ts`)

#### 服务器→客户端消息

**http_request**：请求代理执行HTTP请求

```typescript
{
  id: "uuid",
  type: "http_request",
  payload: {
    method: "POST",
    url: "https://generativelanguage.googleapis.com/v1beta/...",
    headers: { "Content-Type": "application/json" },
    body: "{...}"
  }
}
```

**pong**：心跳响应

```typescript
{
  type: "pong";
}
```

#### 客户端→服务器消息

**http_response**：非流式响应

```typescript
{
  id: "uuid",
  type: "http_response",
  payload: {
    status: 200,
    headers: {...},
    body: "response text"
  }
}
```

**流式响应序列**：

```typescript
// 1. 流开始
{ id: "uuid", type: "stream_start", payload: { status: 200, headers: {...} } }

// 2. 多个数据块
{ id: "uuid", type: "stream_chunk", payload: { data: "chunk1" } }
{ id: "uuid", type: "stream_chunk", payload: { data: "chunk2" } }

// 3. 流结束
{ id: "uuid", type: "stream_end", payload: {} }
```

**error**：错误上报

```typescript
{
  id: "uuid",
  type: "error",
  payload: {
    code: "FETCH_ERROR",
    message: "Network error"
  }
}
```

**ping**：心跳请求

```typescript
{
  type: "ping";
}
```

## 配置说明

### `config.ts`

```typescript
// JWT token用于WebSocket认证，需与Go服务器配置一致
export const JWT_TOKEN: string | null = "valid-token-user-1";

// WebSocket服务器地址
export const WEBSOCKET_PROXY_URL: string = "ws://127.0.0.1:5345/v1/ws";
```

**注意：**

- `JWT_TOKEN` 必须与Go服务器的 `websocket.go:134` 中的验证逻辑一致
- 对于多用户场景，可以为每个用户配置不同的token

## 部署指南

### 1. 配置文件

编辑 `config.ts`，设置正确的JWT token和WebSocket URL。

### 2. 上传到Gemini AI Studio Build

1. 访问 https://ai.google.dev/aistudio
2. 创建新的Build应用
3. 上传以下文件：
   - `App.tsx`
   - `index.tsx`
   - `index.html`
   - `services/webSocketService.ts`
   - `types.ts`
   - `config.ts`
   - `package.json`
   - `vite.config.ts`
   - `tsconfig.json`

### 3. 启动Go代理服务器

确保Go服务器已运行在 `localhost:5345`。

### 4. 测试连接

1. 打开Gemini Build应用（在浏览器中运行）
2. 观察UI中的WebSocket状态指示器
3. 状态应显示 `CONNECTED`（绿色）
4. 日志中应显示 "Auto-connect successful"

### 5. 发送测试请求

从外部客户端（如Roo/Cline）向 `http://127.0.0.1:5345` 发送Gemini API请求，观察：

- Go服务器日志
- WebSocket代理UI中的日志
- 请求应成功返回Gemini API响应

## 开发指南

### 本地开发

```bash
npm install
npm run dev
```

**注意：**本地开发模式主要用于UI调试，实际代理功能需要部署到Gemini Build才能工作（因为需要浏览器的cookie）。

### 调试技巧

1. **查看WebSocket消息**：打开浏览器开发者工具的Network标签，筛选WS连接
2. **查看应用日志**：所有日志都显示在UI中，并输出到console
3. **模拟请求**：可以使用curl或Postman向Go服务器发送测试请求

### 常见问题

**Q: WebSocket连接失败 "Unauthorized"**

- 检查 `config.ts` 中的 `JWT_TOKEN` 是否与Go服务器配置一致

**Q: 连接成功但请求失败**

- 检查浏览器是否已登录Google账号
- 检查Gemini API的URL是否正确

**Q: 流式响应不工作**

- 确保fetch请求的响应支持ReadableStream
- 检查Go服务器是否正确处理 `stream_start/chunk/end` 消息

## 文件说明

```
127-of-websocket-proxy-logger/
├── App.tsx                      # UI组件，显示连接状态和日志
├── index.tsx                    # React应用入口
├── index.html                   # HTML模板
├── config.ts                    # WebSocket URL和JWT token配置
├── types.ts                     # TypeScript类型定义和消息协议
├── services/
│   └── webSocketService.ts      # WebSocket客户端核心实现
├── package.json                 # 依赖管理
├── vite.config.ts              # Vite构建配置
├── tsconfig.json               # TypeScript配置
└── README.md                   # 本文档
```

## 性能优化

- **心跳间隔**：25秒，可根据网络环境调整
- **重连策略**：指数退避（1s → 2s → 4s → ... → 30s）
- **响应通道缓冲**：Go服务器为每个请求创建带缓冲的通道（10条消息）
- **流式传输**：支持大响应的流式处理，避免内存溢出

## 安全性

- **JWT认证**：所有WebSocket连接都需要有效的JWT token
- **同源策略**：利用浏览器的同源策略保护Gemini API
- **无密钥存储**：不在代码中存储Google账号密钥，依赖浏览器cookie

## 扩展性

### 添加新的消息类型

1. 在 `types.ts` 中定义新的消息接口
2. 在 `webSocketService.ts` 的 `onSocketMessage` 中添加处理逻辑
3. 更新Go服务器的对应处理代码

### 添加请求拦截器

在 `handleHttpRequest` 函数中添加请求修改逻辑：

```typescript
async function handleHttpRequest(request: WSHttpRequestMessage) {
  let { method, url, headers, body } = request.payload;

  // 自定义拦截逻辑
  if (url.includes("/v1beta/models")) {
    // 修改请求...
  }

  const response = await fetch(url, fetchOptions);
  // ...
}
```

## 相关资源

- [主项目README](../README.md)
- [Gemini AI Studio](https://ai.google.dev/aistudio)
- [WebSocket API文档](https://developer.mozilla.org/en-US/docs/Web/API/WebSocket)
- [Fetch API文档](https://developer.mozilla.org/en-US/docs/Web/API/Fetch_API)

## 许可证

与主项目保持一致。
