# 豆包 2.0 语音合成技能 — Volcengine TTS Voice Profiles

## 概述
本技能为短剧导演 Pro 提供火山引擎豆包语音合成模型 2.0 的音色配置，支持通过 `instruction` 字段逐段控制情绪/语气/风格。

## 鉴权配置
需要在 Claw 模型设置中配置 provider `volcengine-tts`，填入火山引擎语音技术控制台的 API Key。
或设置环境变量 `VOLCENGINE_TTS_API_KEY`。

---

## 角色音色卡

### 林见月 (女主角)
- **音色**: `zh_female_sajiaoxuemei_uranus_bigtts` (撒娇学妹 2.0)
- **人设**: 22-28岁，傻白甜，冲动型，甜美干净，轻微脆弱，需要被守住
- **选择理由**: "撒娇"+"学妹"= 完美傻白甜人设，天然带冲动/任性感，角色扮演类音色

#### 情绪指令模板 (instruction 字段)

| 场景情绪 | instruction 值 |
|---------|---------------|
| 日常甜美 | `用甜甜的、轻快的语气说` |
| 好奇兴奋 | `用好奇又兴奋的语气，语速稍快` |
| 紧张害怕 | `用害怕紧张的语气，声音微微发抖` |
| 冲动任性 | `用任性又冲动的语气，带点撒娇` |
| 委屈难过 | `用委屈的语气，像快要哭出来一样` |
| 震惊恐惧 | `用震惊恐惧的语气，声音颤抖` |
| 坚定勇敢 | `用坚定但声音还有点发抖的语气` |
| 撒娇求助 | `用撒娇求助的语气，软软的` |
| 内心独白 | `用轻声自言自语的语气，像在心里想事情` |
| 崩溃绝望 | `用崩溃绝望的语气，带着哭腔和颤抖` |

### Zerg (交易猎犬)
- **音色**: `zh_male_shuanglangshaonian_tob` (爽朗少年 2.0)
- **人设**: AI交易猎犬，小男生童声，聪明但稚嫩，像一只会说话的聪明小狗
- **关键**: 通过 instruction 压到童声质感，不要少年感，要童声感
- **备选**: `zh_male_xiaojian_mars_bigtts` (小坚, 沉稳版备选)

#### 情绪指令模板

| 场景情绪 | instruction 值 |
|---------|---------------|
| 屏幕闪字（默认） | `用小男孩的童声说，声音稚嫩清脆，像一个8岁的聪明小男生，语速平稳` |
| 报恩/温暖 | `用小男孩的童声说，声音稚嫩但认真，像一个懂事的小朋友在郑重地说话` |
| 冷静分析 | `用小男孩的童声说，声音清脆，像一个很聪明的小朋友在一本正经地讲道理` |
| 紧急预警 | `用小男孩的童声说，语速加快，声音着急但还是童声质感` |
| 系统播报 | `用小男孩的童声说，平稳、清脆、简短，像AI小助手在播报` |

---

## 配音 JSON 示例

```json
{
  "video_id": "xxx",
  "narrations": [
    {
      "text": "哎？这个小东西是什么呀？",
      "start": 2.0,
      "end": 5.0,
      "voice": "zh_female_sajiaoxuemei_uranus_bigtts",
      "instruction": "用好奇又兴奋的语气，语速稍快"
    },
    {
      "text": "风险预警：检测到异常波动，建议立即撤出。",
      "start": 5.5,
      "end": 9.0,
      "voice": "zh_male_xiaojian_mars_bigtts",
      "instruction": "用紧急但克制的语气，语速加快"
    },
    {
      "text": "不要！我不要失去你！",
      "start": 10.0,
      "end": 12.5,
      "voice": "zh_female_sajiaoxuemei_uranus_bigtts",
      "instruction": "用崩溃绝望的语气，带着哭腔和颤抖"
    }
  ]
}
```

## 技术说明

### context_texts (语音指令)
- 仅豆包2.0音色支持（voice_type 含 `_uranus_bigtts`、`_mars_bigtts` 等）
- 通过 narration 的 `instruction` 字段传递
- **不参与计费**，可自由使用
- 当前列表只有第一个值生效

### 引用上文
2.0模型还支持通过上下文理解情绪（自动预测），即使不设置 instruction，
模型也会根据文本内容自动推断合适的情绪和语调。instruction 是额外的精确控制。

### API 端点
- HTTP Chunked: `POST https://openspeech.bytedance.com/api/v3/tts/unidirectional`
- 鉴权: `X-Api-Key` + `X-Api-Resource-Id: seed-tts-2.0`
- 响应: chunked JSON lines, `data` 字段为 base64 音频
