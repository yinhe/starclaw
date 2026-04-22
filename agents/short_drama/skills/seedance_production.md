# Seedance 2.0 实战生产手册

> EP01-EP04 四集实战沉淀，所有经验编码为可执行规则。

---

## 一、模型参数

```python
{
    "model": "doubao-seedance-2-0-260128",
    "size": "720*1280",          # 9:16 竖屏（抖音优先）
    "duration": 6-10,            # 6s 安全，8s 推荐，10s 极限
    "generate_audio": True,      # 原声可直接用，无需字幕
    "return_last_frame": True,   # 链式生成必须开启
    "watermark": False,
    "category": "short_drama",
}
```

---

## 二、角色一致性方案（v2 — 当前最优解）

### [图N] 标签绑定法

```
[图1]角色名：外观描述
[图2]角色名：外观描述
[图3]角色名：外观描述
严格按参考图还原角色外观，绝不改变服装/发型/体型/配色。
```

- 每镜传**全部出场角色 sheet**（最多 9 张），用逗号拼接 `img_url`
- img_url 顺序必须与 [图N] 标号严格对应
- 标签描述**全集一字不差复用**

### 尾帧链式传递

```
S1 → return_last_frame=True → 提取尾帧
S2 → img_url = "{角色1},{角色2},{角色3},{S1尾帧}"
     prompt 中加 "[图4]上一镜尾帧：场景和角色位置参考"
```

- 尾帧保证场景连续性，**不需要 ref_video_url**
- 去掉 ref_video_url 可**大幅降低隐私过滤误报**

### ⚠️ 不要传 ref_video_url

- ref_video_url + 多张 img_url 同时传 → 隐私过滤必触发
- 尾帧链式传递已足够保证场景连续性
- EP03 S6 连试 15 次不过，去掉 ref_video 后立刻通过

---

## 三、隐私过滤自动重试

Seedance 对含人脸的参考图有隐私过滤器，AI 生成的角色图也会触发。

```python
for attempt in range(1, 16):
    try:
        resp = submit()
        break
    except HTTPError as e:
        if "PrivacyInformation" in str(e) and attempt < 15:
            time.sleep(5)
            continue
        raise
```

- 触发概率与输入图片数量正相关
- 能不传的图就不传（只传出场角色 + 尾帧）
- 最多重试 15 次，间隔 5 秒

---

## 四、Prompt 写作规范

### 结构

```
[图N]标签前缀（全角色）
→ 场景环境描述
→ 角色动作描述（按 [图N] 引用）
→ 情绪/光线/氛围
→ 镜头语言
→ 风格后缀（电影质感，4K，无文字）
```

### 规则

1. **用中文写 prompt** — Seedance 对中文理解优于英文
2. **一条 prompt 只保留 1 个主动作 + 1 个主情绪 + 1 个主运镜**
3. **用 [图N] 引用角色** — 不要重复写外貌描述
4. **具体可拍画面** — 不用抽象概念词
5. **结尾加"电影质感，4K，无文字"** — 避免生成文字水印
6. **不要描写声音** — Seedance 会自动生成音效

### ✅ 好的 prompt

```
[图3]的苏蜜蹲在地上，兴奋地给[图2]的ZERG套上一件粉红色小T恤。
ZERG四条腿僵硬地站着，耳朵耷拉，眼睛眯成缝——满脸嫌弃。
[图1]的林见月坐在沙发上双手捧脸看着这一幕，发出清脆的笑声。
欢乐轻松，暖色调，近地面视角为主。电影质感，4K，无文字
```

### ❌ 差的 prompt

```
一个女孩在给一只小动物穿衣服  （太模糊，无角色绑定）
苏蜜的出租屋里充满了欢乐的气氛  （抽象概念，不可拍）
```

---

## 五、TOS 信任 URL（角色 sheet 上传）

角色三视图需要转为 Volcengine 信任 URL 才能作为 img_url 传给 Seedance。

### 转换方法

使用 Seedream 5.0 lite，`image_strength=0.01`（几乎不改变原图）：

```python
# 1. 本地图片 → base64
# 2. 调用 Seedream 5.0 lite API
# 3. 返回的 result_url 即为信任 URL
# ⚠️ 信任 URL 有效期 24 小时，每次生产前需刷新
```

### 批量刷新

生产一集前，先一次性刷新所有角色 sheet 的 TOS URL，保存到 `_trusted_urls.json`，避免中途过期。

---

## 六、生产流程

```
1. 准备角色 TOS URL（刷新 _trusted_urls.json）
2. 解析剧本 → 提取分镜列表
3. 顺序链式生成：
   S1 → 等完成 → 提取尾帧
   S2 → img_url 加尾帧 → 等完成 → 提取尾帧
   S3 → ...
4. 全部完成 → 标准化（720x1280/24fps/H.264）
5. 拼接（concat copy 或 xfade 转场）
6. 音频混合（原声 + BGM）
7. 叠加片头字幕（drawtext）
8. 输出成片
```

---

## 七、后期合成（FFmpeg）

### 标准化

```bash
ffmpeg -i input.mp4 \
    -vf "scale=720:1280:force_original_aspect_ratio=decrease,pad=720:1280:(ow-iw)/2:(oh-ih)/2" \
    -c:v libx264 -preset fast -crf 18 \
    -r 24 -pix_fmt yuv420p \
    -an output.mp4
```

### 无损拼接（源片段格式一致时）

```bash
ffmpeg -f concat -safe 0 -i concat.txt -c copy output.mp4
```

### 转场拼接

```bash
# xfade 交叉淡化，1.0s
[0:v][1:v]xfade=transition=fade:duration=1.0:offset={dur1-1.0}[v01]
```

### 音频混合

```bash
# normalize=0 手动控制音量，alimiter 削峰
[orig][bgm]amix=inputs=2:duration=first:normalize=0,alimiter=limit=0.95[aout]
```

### 片头字幕（美剧风格 drawtext）

```
虫群              40pt 银灰 左下  1.0s→3.5s 淡入淡出
第一季 落地求生    18pt 暗灰 左下  2.5s→4.5s
第X集 标题        20pt 中灰 左下  3.5s→5.5s
```

---

## 八、成本控制

### 核心数据（EP01-EP04 实测）

| 指标 | 数值 |
|------|------|
| 平均每集生成次数 | ~16 次 |
| 最终可用镜头 | ~6 个 |
| 废片率 | ~59% |
| 平均每集费用 | ~160-320 元 |

### 降本手段

1. **坚持 [图N] 标签 + 尾帧链式** — EP03 v2 六镜一次过
2. **不传 ref_video_url** — 避免隐私过滤重试
3. **先低分辨率预览，确认再正式生成** — 可省 30-50%
4. **批量刷新 TOS URL 后一次性生成全集** — 避免中途过期重做
5. **复用已有素材** — 能改一个镜头就不重做全片

---

## 九、常见问题

| 问题 | 解决 |
|------|------|
| 角色外貌漂移 | 检查 [图N] 标签是否完整、img_url 顺序是否正确 |
| 隐私过滤触发 | 去掉 ref_video_url，减少 img_url 数量 |
| 生成的视频有文字 | prompt 结尾加"无文字" |
| 镜头之间不连续 | 检查尾帧是否正确传递 |
| TOS URL 过期 | 重新用 Seedream 转换，24 小时有效 |
| Seedance 原声质量差 | volume≤0.6 作为底层氛围，主要靠 BGM |
| 现代道具生成不自然 | prompt 重点描写角色反应，不过度描写道具细节 |
