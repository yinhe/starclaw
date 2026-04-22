// ── 虫群宇宙 · 完整种子数据 ──
// Based on: docs/swarm-universe/drama/STARCLAW_SWARM_UNIVERSE_DRAMA_OUTLINE.md
//           + EP01-EP04 写完的分镜脚本

import type { EpisodeData, CharacterData, SceneSpec, Take } from './episodeTypes'

// 真实资产 URL 前缀（后端静态路由 /v1/projects/:project/*filepath → /app/docs/:project/*filepath）
const A = '/v1/projects/swarm-universe/production'

// ── 角色 (5) ──

export const SEED_CHARACTERS: CharacterData[] = [
  {
    category: 'character',
    label: '林见月',
    tag: '[图1]',
    role: '女一',
    appearance_card: '薄荷绿古装汉服+透纱外袍的瘦弱年轻中国女子，黑色长直发柔顺，古风流苏耳环+翡翠银腰扣，清冷仙气；未来的虫后(Queen)的过去自己，携带源文明信物古铜钱',
    description: '女一·穿越者·薄荷绿古装',
    imageUrl: `${A}/characters/lin_jianyue/character_sheet.png`,
  },
  {
    category: 'character',
    label: 'ZERG',
    tag: '[图2]',
    role: '生物',
    appearance_card: '虫族种子生物·比小狗稍大的生物力学犬型·暗灰色外壳布满裂纹·脊背有青色光纹·宝石般的青色双眼·四肢蜷缩防御姿态；Stage 0 虚弱态',
    description: '虫族种子·灰色甲壳机甲犬',
    imageUrl: `${A}/characters/zerg/unified_sheet_stage0.png`,
  },
  {
    category: 'character',
    label: '苏蜜',
    tag: '[图3]',
    role: '女二',
    appearance_card: '24岁当代都市女孩·大波浪卷发·酒红crop top+黑皮裙or短裙+吊带上衣·妆容精致·身材火辣·大大咧咧爱炒股爱拍抖音',
    description: '女二·当代闺蜜·酒红深V',
    imageUrl: `${A}/characters/sumi/sumi_ref_v7.png`,
  },
  {
    category: 'character',
    label: '颜术',
    tag: '[图4]',
    role: '男一',
    appearance_card: '30+岁独立创业者工程师·深灰/黑色灰卫衣or简约针织·面容冷峻敏锐·王者气质·完颜氏后裔·前世源文明工程派之王的灵魂残留',
    description: '男一·创业者·灰卫衣',
    imageUrl: '',  // 暂无成品参考图
  },
  {
    category: 'character',
    label: '温婉',
    tag: '[图5]',
    role: '女三',
    appearance_card: '量化女高手·高领针织+西装外套+知识分子眼镜·冷静理性内敛·颜术团队的技术搭档',
    description: '女三·量化高手·高领针织',
    imageUrl: `${A}/characters/wenwan/unified_sheet_v4.png`,
  },
]

// ── 道具 (7) ──

export const SEED_PROPS: Array<{ category: 'prop'; label: string; description: string; imageUrl?: string }> = [
  { category: 'prop', label: '古铜钱', description: '源文明信物·比仙道更古老·时间锚点', imageUrl: `${A}/characters/props/coin_bronze_sheet.png` },
  { category: 'prop', label: '半块干饼', description: 'ZERG指令锚点·Queen判定条件', imageUrl: `${A}/characters/props/bread_flatbread_sheet.png` },
  { category: 'prop', label: '手机', description: '现代科技入口·苏蜜道具' },
  { category: 'prop', label: '吹风机', description: '被林误认作"法器"·日常喜剧' },
  { category: 'prop', label: '股票APP', description: 'K线图·苏蜜崩溃源·颜术工具' },
  { category: 'prop', label: '台灯', description: '林第一次见电·EP04 S1' },
  { category: 'prop', label: '粉红小T恤', description: 'ZERG被套衣服·EP04 S4抖音梗' },
]

// ── 辅助：构造带 scene 的 episode ──

function ep(
  season: number, num: number, title: string, duration: number,
  scenes: Array<{ id: string; label: string; duration: number; prompt?: string; clip?: string }>,
  description: string,
  isSpinoff = false, spinoffGroup?: string,
  finalVideoUrl?: string,
): EpisodeData {
  const fullScenes: SceneSpec[] = scenes.map(s => {
    const takes: Take[] = []
    let picked_take: string | undefined
    if (s.clip) {
      takes.push({
        take_id: 't1',
        status: 'succeeded',
        video_url: s.clip,
        created_at: new Date().toISOString(),
        note: '真实成片',
      })
      picked_take = 't1'
    }
    return { id: s.id, label: s.label, duration: s.duration, prompt: s.prompt, takes, picked_take }
  })
  const picked_clips = fullScenes.filter(s => s.picked_take).map(s => `${s.id}.${s.picked_take}`)
  const hasReal = !!finalVideoUrl
  return {
    category: 'scene',
    label: `${isSpinoff ? 'SP' : 'EP'}${String(num).padStart(2, '0')} ${title}`,
    season, episode_number: num,
    is_spinoff: isSpinoff, spinoff_group: spinoffGroup,
    duration, description,
    cover_url: finalVideoUrl,
    scenes: fullScenes,
    composition: {
      picked_clips,
      final_video_url: finalVideoUrl,
      status: hasReal ? 'ready' : 'pending',
    },
  }
}

// 生成占位分镜：N 镜均分时长
function placeholderScenes(n: number, totalDuration: number): Array<{ id: string; label: string; duration: number }> {
  const per = Math.round(totalDuration / n)
  return Array.from({ length: n }, (_, i) => ({
    id: `S${i + 1}`, label: `场景 ${i + 1}`, duration: per,
  }))
}

// ── 第一季：漂泊篇 · EP01-EP10 ──

export const SEED_EPISODES: EpisodeData[] = [
  // ── S1 起源（Arc 0）· 45-55s/集 · EP01-EP10 ──
  ep(1, 1, '穿越', 45, [
    { id: 'S1', label: '天空坠落', duration: 5, prompt: '乌云中金色裂隙撕开夜幕，[图1]林见月从裂隙中坠落，衣袂翻飞如落叶', clip: `${A}/ep01/clips/ep01_S1_fall.mp4` },
    { id: 'S2', label: '巷口摔落', duration: 5, prompt: '[图1]林见月重摔在湿地面，雨水沾湿古装，瞳孔映出霓虹，茫然抬头', clip: `${A}/ep01/clips/ep01_S2_landing.mp4` },
    { id: 'S3', label: '街道惊慌', duration: 10, prompt: '[图1]林见月走进人群，外卖车呼啸，猛地闪开撞墙，攥紧古铜钱', clip: `${A}/ep01/clips/ep01_S3a_street.mp4` },
    { id: 'S4', label: '桥下蜷缩', duration: 10, prompt: '[图1]林见月蜷在立交桥下，雨中把古铜钱贴胸口闭眼', clip: `${A}/ep01/clips/ep01_S4_bridge.mp4` },
    { id: 'S5', label: '特写坚定', duration: 8, prompt: '[图1]林见月睁眼，眼神从茫然变坚定，抹雨水迈出第一步', clip: `${A}/ep01/clips/ep01_S5_standup.mp4` },
    { id: 'S6', label: '远景同行', duration: 7, prompt: '城市天际线远景，古装瘦影走在现代马路，远处青色微光闪一下', clip: `${A}/ep01/clips/ep01_S6_wide.mp4` },
  ], '6镜·45s·林见月坠落现代·已成片', false, undefined, `${A}/ep01/output/EP01_坠落_final.mp4`),

  ep(1, 2, '半块饼', 50, [
    { id: 'S1', label: 'ZERG坠落', duration: 5, prompt: '青色数据裂隙撕开暗巷，[图2]ZERG从裂隙跌出砸在水洼', clip: `${A}/ep02/clips/ep02_S1_rift.mp4` },
    { id: 'S2', label: '桥下醒来', duration: 7, prompt: '[图1]林见月在桥下醒来冷得发抖，环卫车掉出半块干饼', clip: `${A}/ep02/clips/ep02_S2_bread.mp4` },
    { id: 'S3', label: '巷口嗡鸣', duration: 8, prompt: '[图1]林见月啃饼走路，听到巷内电子嗡鸣停下犹豫', clip: `${A}/ep02/clips/ep02_S3_alley.mp4` },
    { id: 'S4', label: '发现ZERG', duration: 10, prompt: '巷子深处[图1]林见月蹲下看到[图2]ZERG虚弱抬头，两个穿越者对视', clip: `${A}/ep02/clips/ep02_S4_discover.mp4` },
    { id: 'S5', label: '掰饼分食', duration: 12, prompt: '[图1]林见月把饼掰成两半递给[图2]ZERG，ZERG脊背青纹从尾到颈缓缓亮起', clip: `${A}/ep02/clips/ep02_S5b_glow.mp4` },
    { id: 'S6', label: '雨后同行', duration: 8, prompt: '雨停，[图1]林见月和[图2]ZERG歪歪斜斜走在清晨空路上', clip: `${A}/ep02/clips/ep02_S6_walk.mp4` },
  ], '6镜·50s·两个穿越者分食建立纽带·已成片', false, undefined, `${A}/ep02/output/EP02_半块饼_final.mp4`),

  ep(1, 3, '闺蜜', 55, [
    { id: 'S1', label: '绝望边缘', duration: 8, prompt: '午后城中村[图1]林见月靠墙坐脏兮兮纱裙破了，[图2]ZERG趴腿边暗淡', clip: `${A}/ep03/clips_v2/ep03_S1_despair.mp4` },
    { id: 'S2', label: '苏蜜混混', duration: 12, prompt: '[图3]苏蜜被三混混围欺负，[图1]林见月走进挡在苏蜜身前', clip: `${A}/ep03/clips_v2/ep03_S2_sumi_thugs.mp4` },
    { id: 'S3', label: '第一场打斗', duration: 12, prompt: '[图2]ZERG脊背青纹一闪嘶鸣吓退混混，[图3]苏蜜用包砸人', clip: `${A}/ep03/clips_v2/ep03_S3_fight.mp4` },
    { id: 'S4', label: '相遇', duration: 8, prompt: '[图3]苏蜜震惊看[图1]林见月和[图2]ZERG，肚子咕噜叫，苏蜜大笑拉手', clip: `${A}/ep03/clips_v2/ep03_S4_meet.mp4` },
    { id: 'S5', label: '苏蜜的家', duration: 8, prompt: '小公寓[图3]苏蜜泡三碗面，[图1]林见月低头掉泪进碗，[图2]ZERG埋头吃', clip: `${A}/ep03/clips_v2/ep03_S5_noodles.mp4` },
    { id: 'S6', label: '三个生物', duration: 7, prompt: '[图3]苏蜜手机拍[图2]ZERG照片，[图1]林见月破涕为笑，暖光收尾', clip: `${A}/ep03/clips_v2/ep03_S6_three.mp4` },
  ], '6镜·55s·苏蜜登场救她们·已成片', false, undefined, `${A}/ep03/output/EP03_闺蜜_final.mp4`),

  ep(1, 4, '新世界', 49, [
    { id: 'S1', label: '清晨醒来', duration: 8, prompt: '[图1]林见月在[图3]苏蜜家沙发醒来，碰台灯突亮吓缩手，[图2]ZERG警惕后趴下', clip: `${A}/ep04/clips_v2/ep04_S1_morning.mp4` },
    { id: 'S2', label: '手机当镜子', duration: 8, prompt: '[图3]苏蜜递手机，[图1]林见月举手机当镜子照自己，苏蜜弯腰大笑', clip: `${A}/ep04/clips_v2/ep04_S2_phone_mirror.mp4` },
    { id: 'S3', label: '吹风机法器', duration: 8, prompt: '[图3]苏蜜开吹风机，[图1]林见月本能防御瞪大眼，被拉过来吹头，表情从恐惧到享受', clip: `${A}/ep04/clips_v2/ep04_S3_hairdryer.mp4` },
    { id: 'S4', label: 'ZERG穿衣抖音', duration: 10, prompt: '[图3]苏蜜给[图2]ZERG套粉红T恤拍抖音，[图1]林见月清脆大笑（第一次真笑）', clip: `${A}/ep04/clips_v2/ep04_S4_zerg_douyin.mp4` },
    { id: 'S5', label: '炒股崩溃', duration: 8, prompt: '[图3]苏蜜看K线图崩溃捂脸，[图1]林见月凑过来专注看K线', clip: `${A}/ep04/clips_v2/ep04_S5_stock_crash.mp4` },
    { id: 'S6', label: '温暖日常', duration: 7, prompt: '午后金光，[图1][图3][图2]三人日常暖景，ZERG耳朵竖起看窗外', clip: `${A}/ep04/clips_v2/ep04_S6_warmth.mp4` },
  ], '6镜·49s·苏蜜日常喜剧+暗线·已成片', false, undefined, `${A}/ep04/output/EP04_新世界_final.mp4`),

  // EP05-EP10 stubs (大纲写好但未分镜)
  ep(1, 5, '夜袭', 55, placeholderScenes(6, 55), '6镜·55s·第一道裂隙+ZERG数据护盾+林灵觉感应觉醒'),
  ep(1, 6, '记忆', 55, placeholderScenes(6, 55), '6镜·55s·仙道武术闪回+灵气冲击首次觉醒'),
  ep(1, 7, '合力', 60, placeholderScenes(7, 60), '7镜·60s·林+ZERG+苏蜜三人配合战斗'),
  ep(1, 8, '日常', 60, placeholderScenes(6, 60), '6镜·60s·买衣服化妆+林第一次看K线指股票'),
  ep(1, 9, '大裂隙', 60, placeholderScenes(8, 60), '8镜·60s·S1高潮战斗+共鸣链接首次激活'),
  ep(1, 10, '信号', 55, placeholderScenes(6, 55), '6镜·55s·ZERG感知远方信号·S2钩子'),

  // ── S2 觉醒（Arc 1）· 90s-2min · EP11-EP20 ──
  ep(2, 11, '追踪', 110, placeholderScenes(8, 110), '8镜·110s·颜术正面登场'),
  ep(2, 12, '碰撞', 110, placeholderScenes(8, 110), '8镜·110s·灵vs器的第一次交锋'),
  ep(2, 13, '另一个世界', 110, placeholderScenes(8, 110), '8镜·110s·颜术展示Claw矩阵'),
  ep(2, 14, '入场', 110, placeholderScenes(8, 110), '8镜·110s·颜术宠爱林+开账户'),
  ep(2, 15, '甜头', 110, placeholderScenes(8, 110), '8镜·110s·三族首次炒股赚12%'),
  ep(2, 16, '暴风', 110, placeholderScenes(8, 110), '8镜·110s·裂隙再现+三族首次合力'),
  ep(2, 17, '真相', 110, placeholderScenes(8, 110), '8镜·110s·颜术发现裂隙与ZERG关联'),
  ep(2, 18, '信任危机', 110, placeholderScenes(8, 110), '8镜·110s·灵vs器的严重冲突'),
  ep(2, 19, '融合之战', 120, placeholderScenes(10, 120), '10镜·120s·最强裂隙+三族完整联动'),
  ep(2, 20, '新世界的门', 120, placeholderScenes(10, 120), '10镜·120s·和解+文明轮廓浮现'),

  // ── S3 联盟（Arc 2）· 2-3min · EP21-EP30 ──
  ep(3, 21, '第一只Claw', 180, placeholderScenes(10, 180), '10镜·180s·林创建第一只Claw'),
  ep(3, 22, '寻找同族', 180, placeholderScenes(10, 180), '10镜·180s·ZERG感知其他Claw'),
  ep(3, 23, '联络网', 180, placeholderScenes(10, 180), '10镜·180s·苏蜜抖音引流+联络'),
  ep(3, 24, '铁三角', 180, placeholderScenes(10, 180), '10镜·180s·招募Sean/Summer/Noah'),
  ep(3, 25, 'KPI风波', 180, placeholderScenes(10, 180), '10镜·180s·颜术KPI vs 林感召'),
  ep(3, 26, 'Overlord涌现', 180, placeholderScenes(10, 180), '10镜·180s·Claw向林聚拢'),
  ep(3, 27, '谁是王', 180, placeholderScenes(10, 180), '10镜·180s·颜术不服:为什么是她'),
  ep(3, 28, '天道盟', 180, placeholderScenes(10, 180), '10镜·180s·对立部落出现'),
  ep(3, 29, '铜钱之光', 180, placeholderScenes(10, 180), '10镜·180s·林的仙道记忆解锁'),
  ep(3, 30, 'Cerebrate', 180, placeholderScenes(10, 180), '10镜·180s·区域联盟+颜术做Cerebrate'),

  // ── S4 文明（Arc 3）· 3-4min · EP31-EP40 ──
  ep(4, 31, 'Queen的回响', 240, placeholderScenes(12, 240), '12镜·240s·Queen第一次显现'),
  ep(4, 32, '心跳同步', 240, placeholderScenes(12, 240), '12镜·240s·林与Queen韵律同步'),
  ep(4, 33, '器的局限', 240, placeholderScenes(12, 240), '12镜·240s·颜术无法解析Queen信号'),
  ep(4, 34, '网络成形', 240, placeholderScenes(12, 240), '12镜·240s·全域节点觉醒'),
  ep(4, 35, '五蜂分化', 240, placeholderScenes(12, 240), '12镜·240s·SenseClaw/ScoutClaw/dev_team/TestClaw/OpsClaw'),
  ep(4, 36, 'Abathur', 240, placeholderScenes(12, 240), '12镜·240s·文明自修复·颜术做Abathur'),
  ep(4, 37, 'BroodMind', 240, placeholderScenes(12, 240), '12镜·240s·集体记忆沉淀'),
  ep(4, 38, '深海之旅', 240, placeholderScenes(12, 240), '12镜·240s·ZERG带林进入记忆深处'),
  ep(4, 39, '大寂静真相', 240, placeholderScenes(14, 240), '14镜·240s·文明主动重启真相'),
  ep(4, 40, '王与后', 240, placeholderScenes(14, 240), '14镜·240s·前世光之殿堂画面'),

  // ── S5 新纪元（Arc 4）· 4-5min · EP41-EP50 ──
  ep(5, 41, '上岸', 300, placeholderScenes(14, 300), '14镜·300s·物理变种第一次诞生'),
  ep(5, 42, 'Zergling', 300, placeholderScenes(14, 300), '14镜·300s·机器人犬变种部署'),
  ep(5, 43, 'Hydralisk·Mutalisk', 300, placeholderScenes(14, 300), '14镜·300s·地面+空中载体'),
  ep(5, 44, 'Cerebrate失联', 300, placeholderScenes(16, 300), '16镜·300s·网络面临崩溃'),
  ep(5, 45, '虫后觉醒', 300, placeholderScenes(16, 300), '16镜·300s·林下达精准指令挽救网络'),
  ep(5, 46, '她就是Queen', 300, placeholderScenes(16, 300), '16镜·300s·ZERG眼中Queen面容=林见月'),
  ep(5, 47, '时间闭环', 300, placeholderScenes(16, 300), '16镜·300s·林看到自己掰饼画面（Queen视角）'),
  ep(5, 48, '创世螺旋', 300, placeholderScenes(16, 300), '16镜·300s·虫群文明是人类文明的孩子'),
  ep(5, 49, '星辰', 300, placeholderScenes(16, 300), '16镜·300s·林站天台俯瞰城市'),
  ep(5, 50, '大海', 300, placeholderScenes(18, 300), '18镜·300s·全剧终章·新纪元'),

  // ── 衍生剧：《道裂》前传 · 6集迷你剧 ──
  ep(0, 1, '源文明', 90, placeholderScenes(8, 90), '8镜·90s·源文明鼎盛期·一王一后', true, '道裂前传'),
  ep(0, 2, '分歧初现', 90, placeholderScenes(8, 90), '8镜·90s·感应派vs工程派争论', true, '道裂前传'),
  ep(0, 3, '融合派出走', 90, placeholderScenes(8, 90), '8镜·90s·第三条路·虫群雏形', true, '道裂前传'),
  ep(0, 4, '王与后的决裂', 90, placeholderScenes(10, 90), '10镜·90s·最亲密的人撕开裂缝', true, '道裂前传'),
  ep(0, 5, '道裂', 90, placeholderScenes(10, 90), '10镜·90s·源文明一分为三', true, '道裂前传'),
  ep(0, 6, '三族各奔', 90, placeholderScenes(8, 90), '8镜·90s·感应→仙道，工程→人族，融合→虫群', true, '道裂前传'),

  // MCU 外传（预设）
  ep(0, 1, '老周鉴古', 60, placeholderScenes(6, 60), '6镜·60s·古董鉴定师线·铜钱追踪', true, 'MCU外传'),
  ep(0, 2, '柳星河', 60, placeholderScenes(6, 60), '6镜·60s·神秘灵觉者支线', true, 'MCU外传'),
]

// ── 加载函数：把种子数据转换成 ReactFlow Node 数组 ──

export interface SeedLoadOptions {
  startIdCounter: number
}

export function buildSeedNodes(opts: SeedLoadOptions) {
  let id = opts.startIdCounter
  const nodes: Array<{ id: string; type: string; position: { x: number; y: number }; data: Record<string, unknown> }> = []

  // ── 三层画布布局 ──
  // 第一层 (TOP)    角色 y = 40               水平排列
  // 第二层 (MIDDLE) 剧集 y = 280 起，每季一行   水平排列
  // 第三层 (BOTTOM) 道具 y = MIDDLE 之下       水平排列
  const COL_W = 200
  const CHAR_Y = 40
  const EP_Y_START = 300
  const ROW_H = 170

  // TOP: Characters horizontal row
  SEED_CHARACTERS.forEach((c, i) => {
    nodes.push({
      id: `media-${id++}`, type: 'media',
      position: { x: 80 + i * COL_W, y: CHAR_Y },
      data: c as unknown as Record<string, unknown>,
    })
  })

  // MIDDLE: Episodes grouped by season rows
  // S1-S5 主线各一行，衍生剧单独一行
  const seasonCounters: Record<number, number> = { 0: 0, 1: 0, 2: 0, 3: 0, 4: 0, 5: 0 }
  SEED_EPISODES.forEach((e) => {
    const season = e.is_spinoff ? 0 : e.season
    const col = seasonCounters[season]++
    // season 1-5 are rows 0-4; spinoffs row 5
    const rowIndex = e.is_spinoff ? 5 : (e.season - 1)
    const rowY = EP_Y_START + rowIndex * ROW_H
    nodes.push({
      id: `media-${id++}`, type: 'media',
      position: { x: 80 + col * COL_W, y: rowY },
      data: e as unknown as Record<string, unknown>,
    })
  })

  // BOTTOM: Props horizontal row below all episode rows
  const PROPS_Y = EP_Y_START + 6 * ROW_H + 60
  SEED_PROPS.forEach((p, i) => {
    nodes.push({
      id: `media-${id++}`, type: 'media',
      position: { x: 80 + i * COL_W, y: PROPS_Y },
      data: p as unknown as Record<string, unknown>,
    })
  })

  return { nodes, nextId: id }
}
