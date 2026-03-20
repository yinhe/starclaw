import json

def fmt(sec):
    h = int(sec) // 3600
    m = (int(sec) % 3600) // 60
    s_int = int(sec) % 60
    ms = int((sec - int(sec)) * 1000)
    return f"{h:02d}:{m:02d}:{s_int:02d},{ms:03d}"

credits = ['词曲：孙辉', '演唱：孙辉', '制作人：孙辉']
lyrics = [
    '窗外雪花 飘个不停', '孤单的夜 格外冷清', '我守着回忆 不肯清醒', '等你回到 我的身边',
    '曾经的暖 都已成冰', '爱过的人 没了音讯', '漫天飞雪 模糊眼睛', '只剩我在 原地苦等',
    '等你在冬季 心痛到不行', '这个冬天 谁来陪我同行', '风雪有多冷 伤就有多深', '我还在原地 盼不到你身影',
    '雪花飘不停 泪湿了眼睛', '这段感情 只剩我痴情', '寒风刺我心 回忆全是冰', '今生只为你 等你到天明',
    '曾经的暖 都已成冰', '爱过的人 没了音讯', '漫天飞雪 模糊眼睛', '只剩我在 原地苦等',
    '等你在冬季 心痛到不行', '这个冬天 谁来陪我同行', '风雪有多冷 伤就有多深', '我还在原地 盼不到你身影',
    '雪花飘不停 泪湿了眼睛', '这段感情 只剩我痴情', '寒风刺我心 回忆全是冰', '今生只为你 等你到天明',
    '等你在冬季 心痛到不行', '这个冬天 谁来陪我同行', '风雪有多冷 伤就有多深', '我还在原地 盼不到你身影',
    '雪花飘不停 泪湿了眼睛', '这段感情 只剩我痴情', '寒风刺我心 回忆全是冰', '今生只为你 等你到天明',
]

vocal_start = 18.0
duration = 210.72
outro = 3.0

srt_lines = []
seq = 1

# Credits in intro (0 to vocal_start)
ci = vocal_start / len(credits)
for i, c in enumerate(credits):
    s = i * ci
    e = s + ci - 0.1
    srt_lines.append(f"{seq}\n{fmt(s)} --> {fmt(e)}\n{c}\n")
    seq += 1

# Singing lyrics from vocal_start
sd = duration - vocal_start - outro
li = sd / len(lyrics)
for i, l in enumerate(lyrics):
    s = vocal_start + i * li
    e = s + li - 0.1
    if e > duration:
        e = duration
    if s >= duration:
        break
    srt_lines.append(f"{seq}\n{fmt(s)} --> {fmt(e)}\n{l}\n")
    seq += 1

srt_text = "\n".join(srt_lines)
with open("/tmp/lyrics.srt", "w", encoding="utf-8") as f:
    f.write(srt_text)

# Build chat API request
scenes = '[{"video_id":"b4220a68-854b-4f79-a84a-24418df6cb21","transition":"flash"},{"video_id":"6daac777-5ef1-498c-b75f-02a1a60cfeb5","transition":"crossfade"},{"video_id":"d75d3d74-c278-4681-9284-00bd1ce8dab8","transition":"fadeblack"},{"video_id":"03920818-b9bc-4022-b58c-fec0a3e0c198","transition":"flash"},{"video_id":"252d22bf-f23d-4907-8dfd-6aacd024ad55","transition":"crossfade"},{"video_id":"6cbf5689-89ae-4ea6-96fe-acd5c8da607d","transition":"fadeblack"},{"video_id":"68c5a8f1-b510-409e-a88d-a051ec865d40","transition":"flash"},{"video_id":"9bdd84e9-6d4d-4a06-ab9d-1e993085a8bc"}]'

msg = (
    f"直接调用 mv_production.compose_pro，参数：\n"
    f"scenes: {scenes}\n"
    f"audio_url: /v1/uploads/12c32925-45e4-4d18-8a98-327d5e3687f1.wav\n"
    f"subtitle_style: auto\n"
    f"lyrics_srt（vocal_start=18秒）：\n{srt_text}\n"
    f"立即发起 function call，lyrics_srt 参数填入上面SRT。"
)

req = {
    "conversation_id": "6cb00e7d-8aba-4a18-b690-60ebc4424606",
    "agent_id": "e8be90b5-d1af-4cdc-a719-7e3af7a6f806",
    "message": msg,
}
with open("/tmp/compose_req.json", "w", encoding="utf-8") as f:
    json.dump(req, f, ensure_ascii=False)

print(f"SRT: {seq-1} entries, first lyric at {vocal_start}s, interval={li:.2f}s")
print(f"JSON request: {len(msg)} chars")
