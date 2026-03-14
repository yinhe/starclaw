# StarAI Proxy 服务 - 完整接口文档

这是 StarAI 项目的代理服务模块，提供统一的 AI 服务接口，集成了多个主流AI服务提供商。

## 🚀 功能特性

- 🤖 **多AI模型支持**: OpenAI GPT、Grok、Fal.ai、RunwayML
- 🎨 **图像生成**: 文本生成图像、图像转换、图生图
- 🎬 **视频生成**: 文本生成视频、图像生成视频 (Veo 2/3)
- 🗣️ **语音服务**: TTS文本转语音、STT语音转文本
- 📄 **文档处理**: PDF、Word、Excel文档解析
- 🔄 **异步处理**: 同步/异步任务处理模式
- 📊 **任务管理**: 完整的任务状态跟踪
- 🔐 **安全认证**: API密钥验证
- ⚡ **实时通信**: WebSocket支持
- 📈 **限流保护**: API访问频率限制

## 🛠️ 技术栈

- **Node.js** + **Express**: 后端服务框架
- **Socket.IO**: 实时通信
- **Redis**: 缓存和任务状态存储
- **Bull Queue**: 任务队列管理
- **Multer**: 文件上传处理
- **Sharp**: 图像处理
- **Joi**: 参数验证

## 🚀 快速开始

### 环境要求

- Node.js >= 16.0.0
- Redis 服务器
- 相关AI服务的API密钥

### 安装与启动

```bash
# 安装依赖
npm install

# 配置环境变量
cp .env.example .env
# 编辑 .env 文件，填入相关API密钥

# 启动服务
./start.sh
# 或者
npm start

# 停止服务
./stop.sh
```

### 基础配置

- **服务端口**: 8000 (可通过 `PORT` 环境变量修改)
- **Base URL**: `http://localhost:8000`
- **认证方式**: API Key (Header: `X-API-KEY`)
- **Content-Type**: `application/json`

---

# 📚 完整API接口文档

## 🔐 认证说明

所有API请求（除静态文件访问外）都需要在请求头中包含API密钥：

```http
X-API-KEY: your-api-key
```

**免认证路径**:
- `/audio/*` - 音频文件访问
- `/uploads/*` - 上传文件访问  
- `/videos/*` - 视频文件访问

---

## 🤖 OpenAI GPT 聊天接口

### POST `/chat/completions`

**功能**: OpenAI GPT模型聊天完成

**请求参数**:
```json
{
  "model": "gpt-4",
  "messages": [
    {
      "role": "user",
      "content": "Hello, how are you?"
    }
  ],
  "stream": false,
  "temperature": 0.7,
  "max_tokens": 2000,
  "stop": ["\n"]
}
```

**响应示例**:
```json
{
  "id": "chatcmpl-xxx",
  "object": "chat.completion",
  "created": 1677652288,
  "model": "gpt-4",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello! I'm doing well, thank you for asking."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 9,
    "completion_tokens": 12,
    "total_tokens": 21
  }
}
```

**支持的模型**:
- `gpt-4`
- `gpt-4-turbo`
- `gpt-3.5-turbo`
- 等OpenAI支持的所有模型

---

## 🧠 Grok AI 聊天接口

### POST `/grok/chat/completions`

**功能**: Grok AI模型聊天完成（完整版，支持多客户端负载均衡）

**请求参数**:
```json
{
  "model": "grok-beta",
  "messages": [
    {
      "role": "user",
      "content": "Explain quantum computing"
    }
  ],
  "stream": false,
  "grokClient": "grok1",
  "stop": ["\n"]
}
```

**特殊参数**:
- `grokClient` (可选): 指定使用的Grok客户端，不指定则自动选择

**响应格式**: 与OpenAI兼容的响应格式

### POST `/grok/chat`

**功能**: Grok AI简化聊天接口（自动客户端选择）

**请求参数**:
```json
{
  "model": "grok-beta",
  "messages": [
    {
      "role": "user",
      "content": "Hello Grok!"
    }
  ],
  "stream": false
}
```

**支持的模型**:
- `grok-beta`
- `grok-2-latest`
- 等Grok支持的模型

### GET `/grok/models`

**功能**: 获取可用的Grok模型和客户端信息

**响应示例**:
```json
{
  "availableClients": ["grok1", "grok2"],
  "supportedModels": ["grok-beta", "grok-2-latest"],
  "clientsInfo": {
    "grok1": {
      "name": "Grok Client 1",
      "enabled": true,
      "supportedModels": ["grok-beta"]
    }
  }
}
```

---

## 🎨 Fal.ai 图像生成接口

### 图生图服务 `/fal/image-to-image/*`

#### POST `/fal/image-to-image/submit`

**功能**: 提交图生图任务

**请求参数**:
```json
{
  "model": "fal-ai/flux/dev",
  "image_url": "https://example.com/image.jpg",
  "prompt": "Transform this image into a painting",
  "strength": 0.95,
  "num_inference_steps": 40,
  "seed": 12345,
  "guidance_scale": 3.5,
  "sync_mode": false,
  "num_images": 1,
  "enable_safety_checker": true,
  "webhookUrl": "https://your-webhook.com/callback"
}
```

**参数说明**:
- `model`: 必需，Fal.ai模型路径
- `image_url`: 必需，源图像URL
- `prompt`: 必需，图像转换提示词
- `strength`: 转换强度 (0-1)
- `sync_mode`: 是否同步模式
- `num_images`: 生成图像数量 (1-10)

**同步模式响应**:
```json
{
  "images": [
    {
      "url": "https://fal.media/files/xxx.jpg",
      "width": 1024,
      "height": 1024
    }
  ],
  "prompt": "Transform this image into a painting",
  "seed": 12345,
  "timings": {},
  "has_nsfw_concepts": [false]
}
```

**异步模式响应**:
```json
{
  "request_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

#### POST `/fal/image-to-image/status`

**功能**: 查询任务状态

**请求参数**:
```json
{
  "model": "fal-ai/flux/dev",
  "requestId": "550e8400-e29b-41d4-a716-446655440000",
  "logs": true
}
```

#### POST `/fal/image-to-image/result`

**功能**: 获取任务结果

**请求参数**:
```json
{
  "model": "fal-ai/flux/dev",
  "requestId": "550e8400-e29b-41d4-a716-446655440000"
}
```

#### DELETE `/fal/image-to-image/cancel`

**功能**: 取消任务

**请求参数**:
```json
{
  "request_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

### Flux Pro Kontext - 高质量图像生成

#### POST `/fal/flux-pro-kontext`

**功能**: 使用Flux Pro Kontext模型生成高质量图像

**请求参数**:
```json
{
  "prompt": "A beautiful landscape with mountains and lakes",
  "image_size": "landscape_4_3",
  "num_inference_steps": 28,
  "guidance_scale": 3.5,
  "num_images": 1,
  "enable_safety_checker": true
}
```

**支持的图像尺寸**:
- `square_hd` (1024x1024)
- `square` (512x512)
- `portrait_4_3` (768x1024)
- `portrait_16_9` (576x1024)
- `landscape_4_3` (1024x768)
- `landscape_16_9` (1024x576)

### Flux Krea - 创意图像生成

#### POST `/fal/flux-krea`

**功能**: 使用Flux Krea模型生成创意艺术图像，支持多种艺术风格

**请求参数**:
```json
{
  "prompt": "A surreal landscape with floating islands and magical creatures",
  "image_size": "landscape_4_3",
  "num_inference_steps": 28,
  "guidance_scale": 3.5,
  "num_images": 1,
  "enable_safety_checker": true,
  "seed": 42
}
```

**参数说明**:
- `prompt`: 必需，图像生成提示词
- `image_size`: 图像尺寸，支持与Flux Pro Kontext相同的尺寸选项
- `num_inference_steps`: 推理步数，默认28
- `guidance_scale`: 引导比例，默认3.5
- `num_images`: 生成图像数量，默认1
- `enable_safety_checker`: 是否启用安全检查，默认true
- `seed`: 随机种子，可选

**响应示例**:
```json
{
  "images": [
    {
      "url": "https://fal.media/files/krea-xxx.jpg",
      "width": 1024,
      "height": 768,
      "content_type": "image/jpeg"
    }
  ],
  "prompt": "A surreal landscape with floating islands and magical creatures",
  "seed": 42,
  "timings": {
    "inference": 3.2
  },
  "has_nsfw_concepts": [false]
}
```

**特色功能**:
- 🎨 **创意风格**: 支持多种艺术和创意风格
- 🖼️ **风格迁移**: 能够将不同艺术风格应用到生成的图像
- 🌟 **艺术生成**: 专门优化用于艺术创作和创意设计
- 🎭 **多样化输出**: 相同提示词可产生更多样化的结果

**使用建议**:
- 适合创意设计、艺术创作、概念图生成
- 提示词可以包含艺术风格描述，如"oil painting style"、"watercolor"等
- 建议使用较高的guidance_scale值(3.5-7.5)以获得更好的风格表现

---

## 🎬 视频生成接口

### Veo 3 Fast - 快速视频生成

#### POST `/fal/veo3-fast`

**功能**: 使用Veo 3 Fast模型快速生成视频

**请求参数**:
```json
{
  "prompt": "A cat playing in the garden",
  "duration": 5,
  "aspect_ratio": "16:9",
  "seed": 12345
}
```

**参数说明**:
- `prompt`: 必需，视频生成提示词
- `duration`: 视频时长（秒）
- `aspect_ratio`: 宽高比 ("16:9", "9:16", "1:1")
- `seed`: 随机种子

### Veo 3 - 标准视频生成

#### POST `/fal/veo3`

**功能**: 使用Veo 3模型生成高质量视频

**请求参数**: 与Veo 3 Fast相同

### Veo 2 Image-to-Video - 图像生成视频

#### POST `/fal/veo2-image-to-video`

**功能**: 从图像生成视频

**请求参数**:
```json
{
  "image_url": "https://example.com/image.jpg",
  "prompt": "Make the cat move and play",
  "duration": 5,
  "aspect_ratio": "16:9"
}
```

### Veo 3 Image-to-Video - 图像生成视频（标准版）

#### POST `/fal/veo3-image-to-video`

**功能**: 使用Veo 3模型从图像生成高质量视频

**请求参数**:
```json
{
  "image_url": "https://example.com/image.jpg",
  "prompt": "A person walking through a magical forest",
  "duration": 5,
  "aspect_ratio": "16:9",
  "motion_strength": 0.8
}
```

**参数说明**:
- `image_url`: 必需，源图像URL
- `prompt`: 可选，视频生成提示词，描述希望的动作或场景
- `duration`: 视频时长（秒），默认5秒
- `aspect_ratio`: 宽高比，支持 "16:9", "9:16", "1:1"
- `motion_strength`: 运动强度 (0.0-1.0)，默认0.8

**响应示例**:
```json
{
  "video": {
    "url": "https://fal.media/files/veo3-video-xxx.mp4",
    "width": 1280,
    "height": 720,
    "duration": 5.0
  },
  "prompt": "A person walking through a magical forest",
  "timings": {
    "inference": 45.2
  }
}
```

### Veo 3 Fast Image-to-Video - 图像生成视频（快速版）

#### POST `/fal/veo3-fast-image-to-video`

**功能**: 使用Veo 3 Fast模型从图像快速生成视频，成本更低

**请求参数**:
```json
{
  "image_url": "https://example.com/image.jpg",
  "prompt": "The character starts dancing",
  "duration": 5,
  "aspect_ratio": "16:9",
  "motion_strength": 0.8
}
```

**参数说明**: 与Veo 3 Image-to-Video相同

**响应格式**: 与Veo 3 Image-to-Video相同

**特色对比**:

| 特性 | Veo 3 Image-to-Video | Veo 3 Fast Image-to-Video |
|------|---------------------|---------------------------|
| 质量 | 高质量 | 中等质量 |
| 速度 | 较慢 | 快速 |
| 成本 | 较高 | 低成本 |
| 适用场景 | 专业制作 | 快速原型 |

**使用建议**:
- **Veo 3 Image-to-Video**: 适合需要高质量输出的专业制作
- **Veo 3 Fast Image-to-Video**: 适合快速原型制作和测试
- 使用高分辨率图像可获得更好的结果
- `motion_strength`参数可以控制视频中的运动幅度

### RunwayML 视频生成

#### POST `/generate-video`

**功能**: 使用RunwayML生成视频

**请求参数**:
```json
{
  "prompt": "A serene lake at sunset",
  "duration": 10,
  "resolution": "1280x720"
}
```

---

## 🗣️ 语音服务接口

### 文本转语音 (TTS)

#### POST `/create-speech`

**功能**: 将文本转换为语音

**请求参数**:
```json
{
  "model": "tts-1",
  "voice": "alloy",
  "input": "Hello, this is a test of text-to-speech."
}
```

**支持的声音**:
- `alloy`
- `echo`
- `fable`
- `onyx`
- `nova`
- `shimmer`

**响应**: 返回音频文件流

### 语音转文本 (STT)

#### POST `/post-transcription`

**功能**: 将音频转换为文本

**请求类型**: `multipart/form-data`

**请求参数**:
- `audio`: 音频文件 (文件上传)
- `model`: 模型名称 (默认: "whisper-1")
- `response_format`: 响应格式 ("json", "text", "srt", "verbose_json", "vtt")
- `timestamp_granularities`: 时间戳粒度

**响应示例**:
```json
"This is the transcribed text from the audio file."
```

---

## 📄 文档处理接口

### POST `/upload-and-parse`

**功能**: 上传并解析文档（PDF、Word、Excel等）

**请求类型**: `multipart/form-data`

**支持的文件类型**:
- PDF (.pdf)
- Word (.doc, .docx)
- Excel (.xls, .xlsx)
- 文本文件 (.txt)

**响应示例**:
```json
{
  "filename": "document.pdf",
  "content": "Extracted text content from the document...",
  "metadata": {
    "pages": 10,
    "fileSize": 1024000
  }
}
```

### Stable Audio - 高质量音频生成
**接口**: `POST /fal/stable-audio`  
**描述**: 生成高质量音频和音乐，支持长时间生成和负面提示  
**认证**: 需要API密钥

**请求参数**:
```json
{
  "prompt": "relaxing piano melody with soft strings",
  "duration": 30,
  "negative_prompt": "drums, percussion, loud sounds",
  "seed": 42
}
```

**参数说明**:
- `prompt` (必需): 音频描述文本
- `duration` (可选): 音频时长，默认30秒，最大90秒
- `negative_prompt` (可选): 负面提示，描述不想要的元素
- `seed` (可选): 随机种子，用于可重复生成

**响应示例**:
```json
{
  "audio_url": "https://fal.media/files/stable/audio.wav",
  "duration": 30.0,
  "sample_rate": 44100,
  "seed_used": 42
}
```

### AudioLDM 2 - 通用音频生成
**接口**: `POST /fal/audioldm2`  
**描述**: 通用音频生成模型，支持音乐、音效等多种音频类型  
**认证**: 需要API密钥

**请求参数**:
```json
{
  "prompt": "birds chirping in a forest with gentle wind",
  "duration": 10,
  "guidance_scale": 3.5,
  "num_inference_steps": 200,
  "seed": 123
}
```

**参数说明**:
- `prompt` (必需): 音频描述文本
- `duration` (可选): 音频时长，默认10秒，最大20秒
- `guidance_scale` (可选): 引导强度，默认3.5
- `num_inference_steps` (可选): 推理步数，默认200
- `seed` (可选): 随机种子

**响应示例**:
```json
{
  "audio_url": "https://fal.media/files/audioldm/audio.wav",
  "duration": 10.0,
  "sample_rate": 16000,
  "inference_time": 15.2
}
```

### 获取音频模型列表
**接口**: `GET /fal/models`  
**描述**: 获取所有可用的Fal.ai模型信息，包括音频生成模型  
**认证**: 需要API密钥

**响应示例**:
```json
{
  "models": {
    "audio_generation": {
      "musicgen-stereo": {
        "name": "MusicGen Stereo Large",
        "description": "立体声音乐生成，支持多种音乐风格",
        "endpoint": "/fal/musicgen-stereo",
        "type": "text-to-music",
        "max_duration": 30,
        "features": ["stereo_output", "music_generation", "style_control"]
      },
      "stable-audio": {
        "name": "Stable Audio",
        "description": "高质量音频和音乐生成",
        "endpoint": "/fal/stable-audio",
        "type": "text-to-audio",
        "max_duration": 90,
        "features": ["high_quality", "long_duration", "negative_prompts"]
      },
      "audioldm2": {
        "name": "AudioLDM 2",
        "description": "通用音频生成，支持音乐、音效等",
        "endpoint": "/fal/audioldm2",
        "type": "text-to-audio",
        "max_duration": 20,
        "features": ["versatile", "music_and_effects", "fine_control"]
      }
    }
  },
  "total_models": 8,
  "categories": ["video_generation", "image_generation", "audio_generation"]
}
        "endpoint": "/fal/veo3"
      },
      "veo2-image-to-video": {
        "name": "Veo 2 Image-to-Video",
        "description": "图像生成视频",
        "endpoint": "/fal/veo2-image-to-video",
{{ ... }}
        "type": "image-to-video",
        "max_duration": 8,
        "supported_ratios": ["16:9", "9:16", "1:1"]
      },
      "veo3-image-to-video": {
        "name": "Veo 3 Image-to-Video",
        "description": "Veo 3图像生成视频，高质量效果",
        "endpoint": "/fal/veo3-image-to-video",
        "type": "image-to-video",
        "max_duration": 8,
        "supported_ratios": ["16:9", "9:16", "1:1"],
        "features": ["motion_control", "high_quality"]
      },
      "veo3-fast-image-to-video": {
        "name": "Veo 3 Fast Image-to-Video",
        "description": "Veo 3快速图像生成视频，成本更低",
        "endpoint": "/fal/veo3-fast-image-to-video",
        "type": "image-to-video",
        "max_duration": 8,
        "supported_ratios": ["16:9", "9:16", "1:1"],
        "features": ["motion_control", "fast_generation"]
      }
    },
    "image_generation": {
      "flux-pro-kontext": {
        "name": "Flux Pro Kontext",
        "description": "高质量图像生成",
        "endpoint": "/fal/flux-pro-kontext",
        "type": "text-to-image",
        "supported_sizes": ["square_hd", "square", "portrait_4_3", "portrait_16_9", "landscape_4_3", "landscape_16_9"]
      },
      "flux-krea": {
        "name": "Flux Krea",
        "description": "创意图像生成，支持多种艺术风格",
        "endpoint": "/fal/flux-krea",
        "type": "text-to-image",
        "supported_sizes": ["square_hd", "square", "portrait_4_3", "portrait_16_9", "landscape_4_3", "landscape_16_9"],
        "features": ["creative_styles", "artistic_generation", "style_transfer"]
      }
    }
  },
  "categories": ["video_generation", "image_generation"]
}
```

---

## 🔄 WebSocket 实时通信

### 连接地址
```
ws://localhost:8000
```

### 支持的事件

#### 客户端监听事件
- `taskCompleted`: 任务完成通知
- `taskFailed`: 任务失败通知
- `taskCanceled`: 任务取消通知
- `taskProgress`: 任务进度更新

#### 事件数据格式
```javascript
// 任务完成
socket.on('taskCompleted', (data) => {
  console.log('Task completed:', data)
  // data: { images, prompt, seed, timings, has_nsfw_concepts }
})

// 任务失败
socket.on('taskFailed', (data) => {
  console.log('Task failed:', data)
  // data: { error }
})

// 任务取消
socket.on('taskCanceled', (data) => {
  console.log('Task canceled:', data)
  // data: { message }
})
```

---

## 📁 静态文件访问

### 音频文件
```
GET /audio/{filename}
```

### 上传文件
```
GET /uploads/{filename}
```

### 视频文件
```
GET /videos/{filename}
```

---

## ⚠️ 错误处理

### 常见错误码

- `400 Bad Request`: 请求参数错误
- `401 Unauthorized`: API密钥无效或缺失
- `404 Not Found`: 资源不存在
- `429 Too Many Requests`: 请求频率超限
- `500 Internal Server Error`: 服务器内部错误
- `503 Service Unavailable`: 服务不可用

### 错误响应格式
```json
{
  "error": "错误描述",
  "details": "详细错误信息",
  "type": "error_type"
}
```

---

## 🔧 配置说明

### 环境变量配置

```bash
# 服务配置
PORT=8000
API_KEY=your-api-key

# OpenAI配置
OPENAI_API_KEY=your-openai-key

# Fal.ai配置
FAL_KEY=your-fal-key

# RunwayML配置
RUNWAYML_API_SECRET=your-runway-secret

# Grok配置 (支持多个客户端)
GROK_API_KEY_1=your-grok-key-1
GROK_API_KEY_2=your-grok-key-2
GROK_ENABLED_1=true
GROK_ENABLED_2=true

# Redis配置
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
```

### 限流配置

- **窗口时间**: 15分钟
- **最大请求数**: 100次/IP
- **错误消息**: "Too many requests, please try again later."

---

## 📈 性能和监控

### 任务队列状态

服务使用Redis和Bull队列管理异步任务，支持：
- 任务状态跟踪
- 失败重试机制
- 任务取消功能
- 实时进度通知

### 日志记录

服务会记录以下信息：
- API调用日志
- 错误详情
- 任务执行状态
- 性能指标

---

## 🚀 使用示例

### JavaScript/Node.js 示例

```javascript
const axios = require('axios')

const client = axios.create({
  baseURL: 'http://localhost:8000',
  headers: {
    'X-API-KEY': 'your-api-key',
    'Content-Type': 'application/json'
  }
})

// GPT聊天
async function chatWithGPT() {
  const response = await client.post('/chat/completions', {
    model: 'gpt-4',
    messages: [{ role: 'user', content: 'Hello!' }]
  })
  console.log(response.data)
}

// 生成图像 - Flux Pro Kontext
async function generateImage() {
  const response = await client.post('/fal/flux-pro-kontext', {
    prompt: 'A beautiful sunset over mountains',
    image_size: 'landscape_4_3'
  })
  console.log(response.data)
}

// 生成创意图像 - Flux Krea
async function generateCreativeImage() {
  const response = await client.post('/fal/flux-krea', {
    prompt: 'A surreal landscape with floating islands, oil painting style',
    image_size: 'landscape_4_3',
    guidance_scale: 5.0,
    seed: 42
  })
  console.log(response.data)
}

// 生成视频
async function generateVideo() {
  const response = await client.post('/fal/veo3-fast', {
    prompt: 'A cat playing in the garden',
    duration: 5,
    aspect_ratio: '16:9'
  })
  console.log(response.data)
}

// 图片生成视频 - Veo 2
async function imageToVideo() {
  const response = await client.post('/fal/veo2-image-to-video', {
    image_url: 'https://example.com/image.jpg',
    prompt: '让图片中的人物开始跳舞',
    duration: 5
  })
  console.log('生成的视频:', response.data)
}

// 图片生成视频 - Veo 3 (高质量)
async function imageToVideoVeo3() {
  const response = await client.post('/fal/veo3-image-to-video', {
    image_url: 'https://example.com/portrait.jpg',
    prompt: 'A person walking through a magical forest',
    duration: 5,
    aspect_ratio: '16:9',
    motion_strength: 0.8
  })
  console.log('Veo 3生成的视频:', response.data)
}

// 图片生成视频 - Veo 3 Fast (快速版)
async function imageToVideoVeo3Fast() {
  const response = await client.post('/fal/veo3-fast-image-to-video', {
    image_url: 'https://example.com/character.jpg',
    prompt: 'The character starts dancing',
    duration: 5,
    aspect_ratio: '16:9',
    motion_strength: 0.9
  })
  console.log('Veo 3 Fast生成的视频:', response.data)
}
```

### Python 示例

```python
import requests
import json

class StarAIClient:
    def __init__(self, api_key, base_url='http://localhost:8000'):
        self.base_url = base_url
        self.headers = {
            'X-API-KEY': api_key,
            'Content-Type': 'application/json'
        }
    
    def chat_gpt(self, messages, model='gpt-4'):
        response = requests.post(
            f'{self.base_url}/chat/completions',
            headers=self.headers,
            json={
                'model': model,
                'messages': messages
            }
        )
        return response.json()
    
    def generate_image(self, prompt, image_size='landscape_4_3'):
        response = requests.post(
            f'{self.base_url}/fal/flux-pro-kontext',
            headers=self.headers,
            json={
                'prompt': prompt,
                'image_size': image_size
            }
        )
        return response.json()
    
    def generate_creative_image(self, prompt, image_size='landscape_4_3', guidance_scale=5.0, seed=None):
        payload = {
            'prompt': prompt,
            'image_size': image_size,
            'guidance_scale': guidance_scale
        }
        if seed:
            payload['seed'] = seed
            
        response = requests.post(
            f'{self.base_url}/fal/flux-krea',
            headers=self.headers,
            json=payload
        )
        return response.json()
    
    def generate_video(self, prompt, duration=5):
        response = requests.post(
            f'{self.base_url}/fal/veo3-fast',
            headers=self.headers,
            json={
                'prompt': prompt,
                'duration': duration,
                'aspect_ratio': '16:9'
            }
        )
        return response.json()
    
    def image_to_video_veo3(self, image_url, prompt=None, duration=5, motion_strength=0.8):
        payload = {
            'image_url': image_url,
            'duration': duration,
            'aspect_ratio': '16:9',
            'motion_strength': motion_strength
        }
        if prompt:
            payload['prompt'] = prompt
            
        response = requests.post(
            f'{self.base_url}/fal/veo3-image-to-video',
            headers=self.headers,
            json=payload
        )
        return response.json()
    
    def image_to_video_veo3_fast(self, image_url, prompt=None, duration=5, motion_strength=0.8):
        payload = {
            'image_url': image_url,
            'duration': duration,
            'aspect_ratio': '16:9',
            'motion_strength': motion_strength
        }
        if prompt:
            payload['prompt'] = prompt
            
        response = requests.post(
            f'{self.base_url}/fal/veo3-fast-image-to-video',
            headers=self.headers,
            json=payload
        )
        return response.json()

# 使用示例
client = StarAIClient('your-api-key')

# 聊天
result = client.chat_gpt([
    {'role': 'user', 'content': 'Hello, how are you?'}
])
print(result)

# 生成图像 - Flux Pro Kontext
image_result = client.generate_image('A beautiful landscape')
print(image_result)

# 生成创意图像 - Flux Krea
creative_result = client.generate_creative_image(
    'A surreal landscape with floating islands, watercolor style',
    image_size='landscape_4_3',
    guidance_scale=5.0,
    seed=42
)
print(creative_result)

# 生成视频
video_result = client.generate_video('A cat playing')
print(video_result)

# 图像生成视频 - Veo 3 (高质量)
veo3_video = client.image_to_video_veo3(
    'https://example.com/portrait.jpg',
    prompt='A person walking through a magical forest',
    duration=5,
    motion_strength=0.8
)
print('Veo 3视频生成结果:', veo3_video)

# 图像生成视频 - Veo 3 Fast (快速版)
veo3_fast_video = client.image_to_video_veo3_fast(
    'https://example.com/character.jpg',
    prompt='The character starts dancing',
    duration=5,
    motion_strength=0.9
)
print('Veo 3 Fast视频生成结果:', veo3_fast_video)
```

---

## 📞 技术支持

如有问题或需要技术支持，请：

1. 查看日志文件获取详细错误信息
2. 确认API密钥配置正确
3. 检查网络连接和服务状态
4. 验证请求参数格式

---

**版本**: v1.0.0  
**最后更新**: 2025-01-07  
**维护者**: StarAI Team
