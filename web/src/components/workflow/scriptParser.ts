// ── Script Markdown Parser ──
// 解析剧本 Markdown，输出广告/短剧通用的 EpisodeData。
//
// 支持格式（最简单的约定，方便用户手写）：
//
//   ---
//   label: Q8BOT 进化史 78s
//   duration: 78
//   description: 13镜·78s·林见月+ZERG+苏蜜
//   type: 企业宣传片
//   resolution: 1080p
//   ratio: 16:9
//   ---
//
//   ## S0 时间长廊启动 (6s)
//   prompt: 蓝紫色光带向远处汇聚, 8 字时间隧道开启 ...
//
//   ## S1 1602 阿姆斯特丹咖啡馆 (4s)
//   prompt: ...
//
// 必填: 至少 1 个 `## S<id> <label> (<duration>s)` heading。
// front-matter 完全可选。

import type { EpisodeData, SceneSpec } from './episodeTypes'

export interface ParseResult {
  data: EpisodeData
  warnings: string[]
}

interface FrontMatter {
  label?: string
  duration?: number
  description?: string
  type?: string
  resolution?: string
  ratio?: string
  cover?: string
}

const SCENE_HEADING_RE = /^##\s+(S\d+)\s+(.+?)\s*\((\d+)s\)\s*$/m
const SCENE_SPLIT_RE = /^##\s+S\d+\s+/m

export function parseScriptMarkdown(md: string): ParseResult {
  const warnings: string[] = []
  let body = md
  let fm: FrontMatter = {}

  // 1. front-matter
  const fmMatch = md.match(/^---\s*\n([\s\S]*?)\n---\s*\n?/)
  if (fmMatch) {
    fm = parseFrontMatter(fmMatch[1])
    body = md.slice(fmMatch[0].length)
  }

  // 2. 切分镜头：每个 `## S<n> ...` 是一个 section
  const sections: string[] = []
  let lastIdx = 0
  const re = new RegExp(SCENE_SPLIT_RE.source, 'gm')
  let m: RegExpExecArray | null
  while ((m = re.exec(body))) {
    if (lastIdx > 0) sections.push(body.slice(lastIdx, m.index))
    lastIdx = m.index
  }
  if (lastIdx > 0) sections.push(body.slice(lastIdx))

  if (sections.length === 0) {
    warnings.push('未发现镜头标题（## S0 标题 (6s)），生成空骨架')
  }

  const scenes: SceneSpec[] = []
  for (const sec of sections) {
    const head = sec.match(SCENE_HEADING_RE)
    if (!head) continue
    const [, id, label, durStr] = head
    const sceneBody = sec.slice(head[0].length)
    const promptMatch = sceneBody.match(/(?:^|\n)(?:prompt|提示词|prompt_zh)\s*[:：]\s*([\s\S]*?)(?:\n\s*\n|\n##|$)/i)
    const prompt = promptMatch ? promptMatch[1].trim() : ''
    const sbMatch = sceneBody.match(/(?:^|\n)(?:storyboard|分镜|cover)\s*[:：]\s*(\S+)/i)
    const storyboard = sbMatch ? sbMatch[1].trim() : undefined
    scenes.push({
      id,
      label: label.trim(),
      duration: Number(durStr) || 0,
      prompt,
      storyboard_url: storyboard,
      storyboard_status: storyboard ? 'succeeded' : undefined,
      storyboard_use_as_ref: !!storyboard,
      takes: [],
    })
  }

  const totalDuration = fm.duration || scenes.reduce((acc, s) => acc + s.duration, 0)
  const label = fm.label || (scenes.length ? `导入剧本 ${new Date().toISOString().slice(0, 10)}` : '空白剧本')
  const description = fm.description || `${scenes.length}镜 · ${totalDuration}s · 导入`

  const data: EpisodeData = {
    category: 'scene',
    label,
    season: 0,
    duration: totalDuration,
    description,
    cover_url: fm.cover,
    video_resolution: fm.resolution,
    video_ratio: fm.ratio,
    scenes,
    composition: { picked_clips: [], status: 'pending' },
  }

  return { data, warnings }
}

function parseFrontMatter(raw: string): FrontMatter {
  const out: FrontMatter = {}
  for (const line of raw.split(/\r?\n/)) {
    const m = line.match(/^\s*([a-zA-Z_]+)\s*[:：]\s*(.+?)\s*$/)
    if (!m) continue
    const k = m[1].toLowerCase()
    const v = m[2].replace(/^["']|["']$/g, '')
    switch (k) {
      case 'label':
      case 'title':
        out.label = v
        break
      case 'duration':
      case '总时长':
        out.duration = Number(v) || undefined
        break
      case 'description':
      case 'desc':
        out.description = v
        break
      case 'type':
      case 'category':
      case '类型':
        out.type = v
        break
      case 'resolution':
        out.resolution = v
        break
      case 'ratio':
      case 'aspect':
        out.ratio = v
        break
      case 'cover':
      case 'cover_url':
        out.cover = v
        break
    }
  }
  return out
}

// 帮用户生成示例骨架（modal 里的「填充示例」按钮）
export const SAMPLE_SCRIPT_MD = `---
label: 示例剧本 60s
duration: 60
description: 6镜·60s·导入示例
type: 企业宣传片
resolution: 1080p
ratio: 16:9
---

## S0 开场氛围 (8s)
prompt: 城市清晨, 镜头从天空缓慢下移到主角窗前 ...

## S1 困境呈现 (10s)
prompt: 主角在电脑前皱眉, 屏幕上是混乱的数据 ...

## S2 解决方案登场 (12s)
prompt: 产品 UI 浮现, 数据自动归类 ...

## S3 使用过程 (10s)
prompt: 多人协作, 信息流自动流转 ...

## S4 成果展示 (10s)
prompt: 报表生成, 主角微笑 ...

## S5 品牌收尾 (10s)
prompt: 品牌 LOGO + slogan 居中放大 ...
`
