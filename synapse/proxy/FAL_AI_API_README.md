# fal.ai API 接口文档

本文档介绍了服务器中新增的 fal.ai 接口，包括视频生成和图像生成功能。

## 接口概览

### 视频生成接口

1. **Veo 3 Fast** - 快速文本生成视频
2. **Veo 3** - 高质量文本生成视频  
3. **Veo 2 Image-to-Video** - 图片生成视频

### 图像生成接口

1. **Flux Pro Kontext** - 高质量图像生成

## API 端点详情

### 1. Veo 3 Fast - 快速文本生成视频

**端点**: `POST /fal/veo3-fast`

**描述**: 使用 Google Veo 3 Fast 模型快速生成视频，成本更低，速度更快。

**请求参数**:
```json
{
  "prompt": "一只可爱的小猫在花园里玩耍",
  "duration": 5,
  "aspect_ratio": "16:9"
}
```

**参数说明**:
- `prompt` (必需): 视频描述文本
- `duration` (可选): 视频时长，默认 5 秒，最大 8 秒
- `aspect_ratio` (可选): 视频比例，支持 "16:9", "9:16", "1:1"，默认 "16:9"

**示例请求**:
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

### 2. Veo 3 - 高质量文本生成视频

**端点**: `POST /fal/veo3`

**描述**: 使用 Google Veo 3 模型生成高质量视频。

**请求参数**:
```json
{
  "prompt": "夕阳下的海滩，海浪轻拍岸边",
  "duration": 8,
  "aspect_ratio": "16:9"
}
```

**参数说明**:
- `prompt` (必需): 视频描述文本
- `duration` (可选): 视频时长，默认 5 秒，最大 8 秒
- `aspect_ratio` (可选): 视频比例，支持 "16:9", "9:16", "1:1"，默认 "16:9"

### 3. Veo 2 Image-to-Video - 图片生成视频

**端点**: `POST /fal/veo2-image-to-video`

**描述**: 基于输入图片生成视频。

**请求参数**:
```json
{
  "image_url": "https://example.com/image.jpg",
  "prompt": "让图片中的人物开始跳舞",
  "duration": 5,
  "aspect_ratio": "16:9"
}
```

**参数说明**:
- `image_url` (必需): 输入图片的 URL
- `prompt` (可选): 视频生成的描述文本
- `duration` (可选): 视频时长，默认 5 秒，最大 8 秒
- `aspect_ratio` (可选): 视频比例，支持 "16:9", "9:16", "1:1"，默认 "16:9"

### 4. Flux Pro Kontext - 高质量图像生成

**端点**: `POST /fal/flux-pro-kontext`

**描述**: 使用 Flux Pro Kontext 模型生成高质量图像。

**请求参数**:
```json
{
  "prompt": "一幅美丽的山水画，中国传统风格",
  "image_size": "landscape_4_3",
  "num_inference_steps": 28,
  "guidance_scale": 3.5
}
```

**参数说明**:
- `prompt` (必需): 图像描述文本
- `image_size` (可选): 图像尺寸，默认 "landscape_4_3"
  - 支持: "square_hd", "square", "portrait_4_3", "portrait_16_9", "landscape_4_3", "landscape_16_9"
- `num_inference_steps` (可选): 推理步数，默认 28
- `guidance_scale` (可选): 引导比例，默认 3.5

## 获取模型信息

### 获取所有 fal.ai 模型列表

**端点**: `GET /fal/models`

**描述**: 获取所有可用的 fal.ai 模型信息。

**示例请求**:
```bash
curl -X GET "http://localhost:8000/fal/models" \
  -H "X-API-KEY: your_api_key"
```

**响应示例**:
```json
{
  "models": {
    "video_generation": {
      "veo3-fast": {
        "name": "Veo 3 Fast",
        "description": "快速文本生成视频，成本更低",
        "endpoint": "/fal/veo3-fast",
        "type": "text-to-video",
        "max_duration": 8,
        "supported_ratios": ["16:9", "9:16", "1:1"]
      },
      "veo3": {
        "name": "Veo 3",
        "description": "高质量文本生成视频",
        "endpoint": "/fal/veo3",
        "type": "text-to-video",
        "max_duration": 8,
        "supported_ratios": ["16:9", "9:16", "1:1"]
      },
      "veo2-image-to-video": {
        "name": "Veo 2 Image-to-Video",
        "description": "图片生成视频",
        "endpoint": "/fal/veo2-image-to-video",
        "type": "image-to-video",
        "max_duration": 8,
        "supported_ratios": ["16:9", "9:16", "1:1"]
      }
    },
    "image_generation": {
      "flux-pro-kontext": {
        "name": "Flux Pro Kontext",
        "description": "高质量图像生成",
        "endpoint": "/fal/flux-pro-kontext",
        "type": "text-to-image",
        "supported_sizes": ["square_hd", "square", "portrait_4_3", "portrait_16_9", "landscape_4_3", "landscape_16_9"]
      }
    }
  },
  "total_models": 4,
  "categories": ["video_generation", "image_generation"]
}
```

## 使用示例

### JavaScript/Node.js 示例

```javascript
// Veo 3 Fast 视频生成
const generateVideoFast = async () => {
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

// Flux Pro Kontext 图像生成
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

### Python 示例

```python
import requests
import json

# Veo 2 图片生成视频
def generate_video_from_image():
    url = "http://localhost:8000/fal/veo2-image-to-video"
    headers = {
        "Content-Type": "application/json",
        "X-API-KEY": "your_api_key"
    }
    data = {
        "image_url": "https://example.com/image.jpg",
        "prompt": "让图片中的人物开始跳舞",
        "duration": 5
    }
    
    response = requests.post(url, headers=headers, json=data)
    result = response.json()
    print("生成的视频:", result)

# 获取模型列表
def get_models():
    url = "http://localhost:8000/fal/models"
    headers = {"X-API-KEY": "your_api_key"}
    
    response = requests.get(url, headers=headers)
    models = response.json()
    print("可用模型:", models)
```

## 错误处理

所有接口都会返回标准的错误响应格式：

```json
{
  "error": "错误描述",
  "details": "详细错误信息"
}
```

常见错误码：
- `400`: 请求参数错误
- `401`: API 密钥无效或缺失
- `500`: 服务器内部错误

## 注意事项

1. **API 密钥**: 所有请求都需要在请求头中包含有效的 `X-API-KEY`
2. **队列处理**: 所有 fal.ai 接口都使用队列处理，可能需要等待一段时间
3. **成本控制**: 不同模型的成本不同，请合理使用
4. **文件大小**: 生成的视频和图像文件可能较大，请注意存储空间
5. **速率限制**: 注意 fal.ai 的 API 速率限制

## 监控和日志

服务器会记录所有 fal.ai API 调用的日志，包括：
- 请求参数
- 队列状态更新
- 生成完成状态
- 错误信息

查看服务器日志以获取详细的调用信息和错误排查。

## 支持

如果遇到问题，请检查：
1. API 密钥是否正确配置
2. fal.ai 服务是否正常
3. 网络连接是否稳定
4. 请求参数是否符合要求

更多信息请参考 [fal.ai 官方文档](https://fal.ai/docs)。
