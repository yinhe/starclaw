# StarAI Server API 文档

一个集成了多种 AI 服务的 Node.js 服务器，支持 OpenAI、Grok、fal.ai 等多个 AI 平台的接口调用。

## 项目结构

```
project/
│
├── config/
│ ├── bullQueues.js      # Bull 队列配置
│ ├── falClient.js       # fal.ai 客户端配置
│ ├── grokClient.js      # Grok API 多客户端配置
│ ├── index.js           # 配置文件入口
│ ├── multer.js          # 文件上传配置
│ ├── openaiClient.js    # OpenAI 客户端配置
│ ├── redis.js           # Redis 配置
│ └── runwayClient.js    # RunwayML 客户端配置
│
├── routes/
│ ├── imageToImageRoutes.js  # 图生图路由
│ └── ... (其他路由文件)
│
├── uploads/             # 上传文件存储
├── audio/               # 音频文件存储
├── server.js            # 主服务器文件
├── package.json         # 项目依赖
├── .env                 # 环境变量配置
├── .env.example         # 环境变量示例
├── GROK_API_README.md   # Grok API 使用文档
├── FAL_AI_API_README.md # fal.ai API 使用文档
└── README.md            # 项目说明文档
```

## 快速开始

### 1. 安装依赖

```bash
npm install
```

### 2. 配置环境变量

复制 `.env.example` 到 `.env` 并填入您的 API 密钥：

```bash
cp .env.example .env
```

### 3. 启动服务器

```bash
npm start
```

服务器将在 `http://localhost:8000` 启动。

## API 接口概览

### 认证

所有 API 请求都需要在请求头中包含 API 密钥：

```
X-API-KEY: your_api_key_here
```

### 主要接口分类

1. **OpenAI 接口** - 文本生成、语音合成等
2. **Grok API 接口** - 多客户端聊天完成
3. **fal.ai 接口** - 视频生成、图像生成
4. **RunwayML 接口** - 视频处理
5. **工具接口** - 文件上传、状态查询等

## 🚀 新增 Grok API 接口

### 多 Grok 客户端支持

服务器现在支持最多 5 个 Grok API 客户端，提供负载均衡和智能路由功能。

#### 获取 Grok 客户端信息

```bash
curl -X GET "http://localhost:8000/grok/clients" \
  -H "X-API-KEY: your_api_key"
```

#### Grok 聊天完成（高级版）

```bash
curl -X POST "http://localhost:8000/grok/chat-completion" \
  -H "Content-Type: application/json" \
  -H "X-API-KEY: your_api_key" \
  -d '{
    "model": "grok-beta",
    "messages": [
      {
        "role": "user",
        "content": "Hello, how are you?"
      }
    ],
    "stream": false,
    "grokClient": "grok-1"
  }'
```

#### Grok 聊天（简化版）

```bash
curl -X POST "http://localhost:8000/grok/chat" \
  -H "Content-Type: application/json" \
  -H "X-API-KEY: your_api_key" \
  -d '{
    "model": "grok-beta",
    "messages": [
      {
        "role": "user",
        "content": "你好，请介绍一下自己"
      }
    ]
  }'
```

**特性**:
- ✅ 智能客户端选择（根据模型自动选择最佳客户端）
- ✅ 负载均衡（轮询分配请求）
- ✅ 流式响应支持
- ✅ 请求中止功能
- ✅ 详细错误处理

## 🎬 新增 fal.ai 视频生成接口

### 1. Veo 3 Fast - 快速视频生成

```bash
curl -X POST "http://localhost:8000/fal/veo3-fast" \
  -H "Content-Type: application/json" \
  -H "X-API-KEY: your_api_key" \
  -d '{
    "prompt": "一只可爱的小猫在花园里玩耍",
    "duration": 5,
    "aspect_ratio": "16:9"
  }'
```

### 2. Veo 3 - 高质量视频生成

```bash
curl -X POST "http://localhost:8000/fal/veo3" \
  -H "Content-Type: application/json" \
  -H "X-API-KEY: your_api_key" \
  -d '{
    "prompt": "夕阳下的海滩，海浪轻拍岸边",
    "duration": 8,
    "aspect_ratio": "16:9"
  }'
```

### 3. Veo 2 - 图片生成视频

```bash
curl -X POST "http://localhost:8000/fal/veo2-image-to-video" \
  -H "Content-Type: application/json" \
  -H "X-API-KEY: your_api_key" \
  -d '{
    "image_url": "https://example.com/image.jpg",
    "prompt": "让图片中的人物开始跳舞",
    "duration": 5
  }'
```

## 🎨 新增 fal.ai 图像生成接口

### Flux Pro Kontext - 高质量图像生成

```bash
curl -X POST "http://localhost:8000/fal/flux-pro-kontext" \
  -H "Content-Type: application/json" \
  -H "X-API-KEY: your_api_key" \
  -d '{
    "prompt": "一幅美丽的山水画，中国传统风格",
    "image_size": "landscape_4_3",
    "num_inference_steps": 28,
    "guidance_scale": 3.5
  }'
```

### 获取 fal.ai 模型信息

```bash
curl -X GET "http://localhost:8000/fal/models" \
  -H "X-API-KEY: your_api_key"
```

## 📝 JavaScript 使用示例

### Grok API 调用

```javascript
// 智能聊天（自动选择最佳客户端）
const chatWithGrok = async () => {
  const response = await fetch('http://localhost:8000/grok/chat', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-API-KEY': 'your_api_key'
    },
    body: JSON.stringify({
      model: 'grok-beta',
      messages: [
        { role: 'user', content: '请解释一下人工智能的发展历程' }
      ]
    })
  });
  
  const result = await response.json();
  console.log('Grok 回复:', result);
};
```

### fal.ai 视频生成

```javascript
// 快速视频生成
const generateVideo = async () => {
  const response = await fetch('http://localhost:8000/fal/veo3-fast', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-API-KEY': 'your_api_key'
    },
    body: JSON.stringify({
      prompt: '一只可爱的小猫在花园里玩耍',
      duration: 5,
      aspect_ratio: '16:9'
    })
  });
  
  const result = await response.json();
  console.log('生成的视频:', result);
};

// 图片生成视频
const imageToVideo = async () => {
  const response = await fetch('http://localhost:8000/fal/veo2-image-to-video', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-API-KEY': 'your_api_key'
    },
    body: JSON.stringify({
      image_url: 'https://example.com/image.jpg',
      prompt: '让图片中的人物开始跳舞',
      duration: 5
    })
  });
  
  const result = await response.json();
  console.log('生成的视频:', result);
};
```

### fal.ai 图像生成

```javascript
// 高质量图像生成
const generateImage = async () => {
  const response = await fetch('http://localhost:8000/fal/flux-pro-kontext', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-API-KEY': 'your_api_key'
    },
    body: JSON.stringify({
      prompt: '一幅美丽的山水画，中国传统风格',
      image_size: 'landscape_4_3'
    })
  });
  
  const result = await response.json();
  console.log('生成的图像:', result);
};
```

## 🐍 Python 使用示例

```python
import requests
import json

# Grok API 调用
def chat_with_grok():
    url = "http://localhost:8000/grok/chat"
    headers = {
        "Content-Type": "application/json",
        "X-API-KEY": "your_api_key"
    }
    data = {
        "model": "grok-beta",
        "messages": [
            {"role": "user", "content": "请介绍一下量子计算的基本原理"}
        ]
    }
    
    response = requests.post(url, headers=headers, json=data)
    result = response.json()
    print("Grok 回复:", result)

# fal.ai 视频生成
def generate_video():
    url = "http://localhost:8000/fal/veo3-fast"
    headers = {
        "Content-Type": "application/json",
        "X-API-KEY": "your_api_key"
    }
    data = {
        "prompt": "一只可爱的小猫在花园里玩耍",
        "duration": 5,
        "aspect_ratio": "16:9"
    }
    
    response = requests.post(url, headers=headers, json=data)
    result = response.json()
    print("生成的视频:", result)

# fal.ai 图像生成
def generate_image():
    url = "http://localhost:8000/fal/flux-pro-kontext"
    headers = {
        "Content-Type": "application/json",
        "X-API-KEY": "your_api_key"
    }
    data = {
        "prompt": "一幅美丽的山水画，中国传统风格",
        "image_size": "landscape_4_3"
    }
    
    response = requests.post(url, headers=headers, json=data)
    result = response.json()
    print("生成的图像:", result)
```

## 🔧 配置说明

### 环境变量配置

```bash
# OpenAI
API_KEY=your_openai_api_key_here

# fal.ai
FAL_KEY=your_fal_api_key_here

# RunwayML
RUNWAYML_API_SECRET=your_runwayml_api_secret_here

# 多个 Grok API 配置
GROK_API_KEY_1=your_grok_api_key_1_here
GROK_BASE_URL_1=https://api.x.ai/v1

GROK_API_KEY_2=your_grok_api_key_2_here
GROK_BASE_URL_2=https://api.x.ai/v1

# ... 最多支持 5 个 Grok 客户端

# 服务器配置
PORT=8000
REDIS_URL=redis://localhost:6379
```

## 📊 监控和日志

服务器启动时会显示配置状态：

```
=== Grok API 配置状态 ===
Grok API grok-1: ✔️
Grok API grok-2: ✔️
Grok API grok-3: ❌
可用的 Grok API 数量: 2
可用的 Grok API: grok-1, grok-2
========================
```

## ⚠️ 注意事项

1. **API 密钥安全**: 确保 `.env` 文件不被提交到版本控制系统
2. **速率限制**: 注意各个 API 的速率限制
3. **成本控制**: 合理使用 AI 服务，注意成本
4. **文件存储**: 生成的媒体文件可能较大，注意存储空间
5. **网络稳定**: 确保服务器网络连接稳定

## 🔗 相关文档

- [Grok API 详细文档](./GROK_API_README.md)
- [fal.ai API 详细文档](./FAL_AI_API_README.md)
- [OpenAI API 文档](https://platform.openai.com/docs)
- [fal.ai 官方文档](https://fal.ai/docs)

## 🆘 故障排除

### 常见问题

1. **API 密钥错误**: 检查 `.env` 文件中的 API 密钥是否正确
2. **网络连接问题**: 确保服务器可以访问外部 API
3. **队列处理缓慢**: fal.ai 接口使用队列处理，可能需要等待
4. **内存不足**: 处理大文件时可能需要更多内存

### 调试技巧

- 查看服务器控制台日志
- 使用 `curl` 命令测试接口
- 检查 Redis 连接状态
- 验证文件上传权限

## 📄 许可证

本项目仅供学习和研究使用。使用第三方 API 服务时，请遵守相应的服务条款。

---

**版本**: 2.0.0  
**更新日期**: 2025-07-13  
**新增功能**: 多 Grok API 支持、fal.ai 视频/图像生成接口
