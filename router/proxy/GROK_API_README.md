# 多 Grok API 支持文档

## 概述

本服务器现在支持配置和使用多个 Grok API 客户端，提供负载均衡和故障转移功能。

## 配置

### 1. 环境变量配置

在 `.env` 文件中添加以下配置：

```bash
# 多个 Grok API 配置
# Grok API 1 (必需)
GROK_API_KEY_1=your_grok_api_key_1_here
GROK_BASE_URL_1=https://api.x.ai/v1

# Grok API 2 (可选)
GROK_API_KEY_2=your_grok_api_key_2_here
GROK_BASE_URL_2=https://api.x.ai/v1

# Grok API 3 (可选)
GROK_API_KEY_3=your_grok_api_key_3_here
GROK_BASE_URL_3=https://api.x.ai/v1
```

### 2. 配置说明

- **GROK_API_KEY_X**: Grok API 的密钥
- **GROK_BASE_URL_X**: Grok API 的基础 URL（默认为 `https://api.x.ai/v1`）
- 只有配置了 API 密钥的客户端才会被启用
- 支持最多 3 个 Grok API 客户端

## API 端点

### 1. 获取可用的 Grok 客户端

```http
GET /grok/clients
```

**响应示例：**
```json
{
  "availableClients": ["grok-1", "grok-2"],
  "clientsInfo": [
    {
      "name": "grok-1",
      "baseURL": "https://api.x.ai/v1",
      "enabled": true
    },
    {
      "name": "grok-2",
      "baseURL": "https://api.x.ai/v1",
      "enabled": true
    }
  ],
  "count": 2
}
```

### 2. Grok 聊天完成（完整版）

```http
POST /grok/chat-completion
```

**请求参数：**
```json
{
  "model": "grok-beta",
  "messages": [
    {
      "role": "user",
      "content": "Hello, how are you?"
    }
  ],
  "stream": false,
  "stop": ["Human:", "AI:"],
  "grokClient": "grok-1"  // 可选，指定使用的客户端
}
```

**参数说明：**
- `model`: 模型名称（默认：`grok-beta`）
- `messages`: 对话消息数组
- `stream`: 是否启用流式响应（默认：`false`）
- `stop`: 停止词数组（可选）
- `grokClient`: 指定使用的 Grok 客户端名称（可选，不指定则自动负载均衡）

### 3. Grok 聊天（简化版）

```http
POST /grok/chat
```

**请求参数：**
```json
{
  "model": "grok-beta",
  "messages": [
    {
      "role": "user",
      "content": "Hello, how are you?"
    }
  ],
  "stream": false
}
```

## 功能特性

### 1. 负载均衡

- 当不指定 `grokClient` 参数时，系统会自动轮询选择可用的 Grok 客户端
- 分散请求负载，提高系统性能

### 2. 故障转移

- 如果某个 Grok API 客户端不可用，系统会自动使用其他可用的客户端
- 提高系统的可靠性和稳定性

### 3. 流式响应支持

- 支持 Server-Sent Events (SSE) 流式响应
- 实时获取 AI 生成的内容

### 4. 请求中止支持

- 支持客户端主动取消请求
- 避免不必要的资源消耗

## 错误处理

### 常见错误响应

1. **没有可用的 Grok 客户端**
```json
{
  "error": "没有可用的 Grok API 客户端",
  "message": "请检查 .env 文件中的 Grok API 配置"
}
```

2. **指定的客户端不存在**
```json
{
  "error": "指定的 Grok 客户端 'grok-x' 不存在或未启用",
  "availableClients": ["grok-1", "grok-2"]
}
```

3. **API 调用失败**
```json
{
  "error": "Grok API 调用失败",
  "details": "具体错误信息",
  "type": "grok_api_error"
}
```

## 使用示例

### JavaScript/Node.js

```javascript
// 获取可用的 Grok 客户端
const clientsResponse = await fetch('http://localhost:8000/grok/clients', {
  headers: {
    'X-API-KEY': 'your_api_key'
  }
});
const clients = await clientsResponse.json();
console.log('可用的 Grok 客户端:', clients.availableClients);

// 使用 Grok 进行聊天
const chatResponse = await fetch('http://localhost:8000/grok/chat', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'X-API-KEY': 'your_api_key'
  },
  body: JSON.stringify({
    model: 'grok-beta',
    messages: [
      {
        role: 'user',
        content: '你好，请介绍一下自己'
      }
    ]
  })
});
const chatResult = await chatResponse.json();
console.log('Grok 响应:', chatResult);
```

### cURL

```bash
# 获取可用的 Grok 客户端
curl -X GET "http://localhost:8000/grok/clients" \
  -H "X-API-KEY: your_api_key"

# 使用 Grok 进行聊天
curl -X POST "http://localhost:8000/grok/chat" \
  -H "Content-Type: application/json" \
  -H "X-API-KEY: your_api_key" \
  -d '{
    "model": "grok-beta",
    "messages": [
      {
        "role": "user",
        "content": "Hello, how are you?"
      }
    ]
  }'
```

## 监控和日志

服务器启动时会显示 Grok API 的配置状态：

```
=== Grok API 配置状态 ===
Grok API grok-1: ✔️
Grok API grok-2: ✔️
Grok API grok-3: ❌
可用的 Grok API 数量: 2
可用的 Grok API: grok-1, grok-2
========================
```

## 注意事项

1. **API 密钥安全**: 请确保 `.env` 文件不被提交到版本控制系统
2. **速率限制**: 注意各个 Grok API 的速率限制
3. **成本控制**: 多个 API 可能会增加使用成本，请合理配置
4. **监控**: 建议监控各个 API 的使用情况和响应时间

## 故障排除

1. **检查环境变量**: 确保 `.env` 文件中的 Grok API 配置正确
2. **检查网络连接**: 确保服务器可以访问 Grok API 端点
3. **检查 API 密钥**: 确保 API 密钥有效且有足够的配额
4. **查看日志**: 检查服务器日志以获取详细的错误信息
