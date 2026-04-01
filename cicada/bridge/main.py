"""
Cicada 🪰 蝉 — AI 电话机器人智能体 Bridge
FastAPI 服务，端口 :8099
"""

import asyncio
import csv
import io
import json
import logging
import os
import sys
from contextlib import asynccontextmanager
from datetime import datetime
from typing import Optional

import uvicorn
import yaml
from fastapi import FastAPI, HTTPException, UploadFile, File, Form, Query
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import FileResponse, JSONResponse
from pydantic import BaseModel

from call_engine import CallEngine
from compliance import ComplianceChecker
from crm_manager import CRMManager
from intent_classifier import IntentClassifier, INTENT_LEVELS
from recorder import Recorder
from scheduler import CallScheduler
from script_engine import ScriptEngine
from sip_client import CloopenClient, MockSIPClient
from voice_pipeline import LLMClient, VoicePipeline

# ── Logging ─────────────────────────────────────────────

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(name)s] %(levelname)s %(message)s",
    datefmt="%H:%M:%S",
)
logger = logging.getLogger("cicada")

# ── Config ──────────────────────────────────────────────

def load_config() -> dict:
    """加载配置文件"""
    config_paths = [
        "config.local.yaml",
        "config.yaml",
        os.path.join(os.path.dirname(__file__), "config.yaml"),
    ]
    for path in config_paths:
        if os.path.exists(path):
            with open(path, "r", encoding="utf-8") as f:
                cfg = yaml.safe_load(f)
                logger.info(f"[config] loaded from {path}")
                return cfg
    logger.warning("[config] no config file found, using defaults")
    return {}


CFG = load_config()

# 从环境变量覆盖关键配置
def env(key: str, default: str = "") -> str:
    return os.environ.get(key, default)

SERVER_PORT = int(env("CICADA_PORT", str(CFG.get("server", {}).get("port", 8099))))
SERVER_HOST = CFG.get("server", {}).get("host", "0.0.0.0")

# 存储路径
STORAGE = CFG.get("storage", {})
DATA_DIR = env("CICADA_DATA_DIR", STORAGE.get("data_dir", "./data/cicada"))
DB_PATH = env("CICADA_DB_PATH", STORAGE.get("db_path", os.path.join(DATA_DIR, "cicada.db")))
RECORDINGS_DIR = STORAGE.get("recordings_dir", os.path.join(DATA_DIR, "recordings"))
os.makedirs(DATA_DIR, exist_ok=True)
os.makedirs(RECORDINGS_DIR, exist_ok=True)

# SIP 配置
SIP_CFG = CFG.get("sip", {})
SIP_SID = env("CLOOPEN_ACCOUNT_SID", SIP_CFG.get("account_sid", ""))
SIP_TOKEN = env("CLOOPEN_AUTH_TOKEN", SIP_CFG.get("auth_token", ""))
SIP_APPID = env("CLOOPEN_APP_ID", SIP_CFG.get("app_id", ""))
CALLBACK_URL = SIP_CFG.get("callback_url", f"http://localhost:{SERVER_PORT}/callback/call-status")
RECORDING_CALLBACK_URL = SIP_CFG.get("recording_callback_url", f"http://localhost:{SERVER_PORT}/callback/recording")

# Voice 配置
VOICE_CFG = CFG.get("voice", {})
DASHSCOPE_KEY = env("DASHSCOPE_API_KEY", VOICE_CFG.get("dashscope_api_key", ""))

# LLM 配置
LLM_CFG = CFG.get("llm", {})
LLM_BASE_URL = env("LLM_BASE_URL", LLM_CFG.get("base_url", "https://api.star-ai.net/v1"))
LLM_API_KEY = env("LLM_API_KEY", LLM_CFG.get("api_key", ""))
LLM_MODEL = LLM_CFG.get("model", "qwen-turbo")

# Scheduler 配置
SCHED_CFG = CFG.get("scheduler", {})
COMPLIANCE_CFG = CFG.get("compliance", {})

# ── Init Services ───────────────────────────────────────

crm = CRMManager(DB_PATH)

# SIP Client（无账号时用 Mock）
if SIP_SID and SIP_TOKEN:
    sip = CloopenClient(SIP_SID, SIP_TOKEN, SIP_APPID, SIP_CFG.get("rest_url", "https://app.cloopen.com:8883"))
else:
    logger.warning("[init] no SIP credentials, using MockSIPClient")
    sip = MockSIPClient()

# LLM Client
llm = LLMClient(
    base_url=LLM_BASE_URL,
    api_key=LLM_API_KEY,
    model=LLM_MODEL,
    temperature=LLM_CFG.get("temperature", 0.3),
    max_tokens=LLM_CFG.get("max_tokens", 200),
)

# 话术引擎
scripts_dir = os.path.join(os.path.dirname(__file__), "..", "scripts")
script_engine = ScriptEngine(scripts_dir if os.path.exists(scripts_dir) else None)

# 意向分类
classifier = IntentClassifier(llm)

# 录音管理
recorder = Recorder(RECORDINGS_DIR)

# 合规检查
compliance = ComplianceChecker(crm, COMPLIANCE_CFG)

# 外呼引擎
engine = CallEngine(sip, None, crm, classifier, compliance, recorder)
engine.max_concurrent = SCHED_CFG.get("max_concurrent", 10)

# 调度器
scheduler = CallScheduler(SCHED_CFG, crm, engine)

# ── Pydantic Models ─────────────────────────────────────

class DialRequest(BaseModel):
    customer_id: int
    campaign_id: int
    phone: str
    display_num: str = ""
    script_industry: str = "general"
    script_variables: dict = {}

class CampaignCreate(BaseModel):
    name: str
    industry: str = "general"
    script_id: Optional[int] = None
    daily_limit: int = 800
    caller_numbers: list[str] = []
    schedule_start: str = "09:00"
    schedule_end: str = "18:00"

class CampaignStart(BaseModel):
    display_num: str
    script_industry: str = "general"
    script_variables: dict = {}

class CustomerUpdate(BaseModel):
    name: Optional[str] = None
    intent_level: Optional[str] = None
    status: Optional[str] = None
    assigned_to: Optional[str] = None
    next_follow_at: Optional[str] = None
    tags: Optional[list[str]] = None

class ScriptCreate(BaseModel):
    name: str
    industry: str = "general"
    greeting: str = ""
    key_points: list[str] = []
    qa_library: list[dict] = []
    objections: list[dict] = []
    closing: dict = {}
    voice: str = "longxiaochun"

# ── App Lifecycle ───────────────────────────────────────

@asynccontextmanager
async def lifespan(app: FastAPI):
    logger.info(f"🪰 Cicada Bridge starting on :{SERVER_PORT}")
    logger.info(f"   DB: {DB_PATH}")
    logger.info(f"   SIP: {'Cloopen' if SIP_SID else 'Mock'}")
    logger.info(f"   LLM: {LLM_MODEL} @ {LLM_BASE_URL}")
    yield
    # Shutdown
    await sip.close()
    await llm.close()
    await recorder.close()
    logger.info("🪰 Cicada Bridge stopped")

app = FastAPI(
    title="Cicada 🪰 蝉 — AI 电话机器人",
    version="1.0.0",
    lifespan=lifespan,
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# ── Health ──────────────────────────────────────────────

@app.get("/health")
async def health():
    return {
        "status": "ok",
        "service": "cicada-bridge",
        "version": "1.0.0",
        "timestamp": datetime.utcnow().isoformat(),
    }

@app.get("/stats")
async def stats():
    overview = crm.get_overview()
    sched = scheduler.get_status()
    eng = engine.get_stats()
    disk = recorder.get_disk_usage()
    return {
        "overview": overview,
        "scheduler": sched,
        "engine": eng,
        "storage": disk,
    }

# ── Call Control ────────────────────────────────────────

@app.post("/call/dial")
async def call_dial(req: DialRequest):
    """发起单通外呼"""
    prompt = script_engine.build_system_prompt(req.script_industry, req.script_variables)
    display = req.display_num
    if not display:
        numbers = SIP_CFG.get("caller_numbers", [])
        display = numbers[0]["number"] if numbers else ""

    result = await engine.dial(
        customer_id=req.customer_id,
        campaign_id=req.campaign_id,
        phone=req.phone,
        display_num=display,
        script_prompt=prompt,
        callback_url=CALLBACK_URL,
    )
    if not result.get("success"):
        raise HTTPException(status_code=400, detail=result.get("error", "dial failed"))
    return result

@app.post("/call/hangup")
async def call_hangup(call_sid: str = Form(...)):
    """挂断通话"""
    result = await engine.hangup(call_sid)
    if not result.get("success"):
        raise HTTPException(status_code=404, detail=result.get("error"))
    return result

@app.get("/call/active")
async def call_active():
    """获取所有活跃通话"""
    return {"calls": engine.get_active_calls()}

@app.get("/call/status/{call_sid}")
async def call_status(call_sid: str):
    """获取通话状态"""
    call = engine.active_calls.get(call_sid)
    if call:
        return call.to_dict()
    # 从DB查
    session = crm.get_session()
    try:
        from crm_manager import CallRecord
        record = session.query(CallRecord).filter_by(call_sid=call_sid).first()
        if record:
            return record.to_dict()
    finally:
        session.close()
    raise HTTPException(status_code=404, detail="call not found")

# ── Callbacks (云通信平台回调) ──────────────────────────

@app.post("/callback/call-status")
async def callback_call_status(
    callSid: str = Form(None),
    action: str = Form(None),
    status: str = Form(None),
    **kwargs,
):
    """通话状态回调（容联云）"""
    sid = callSid or ""
    st = action or status or ""
    logger.info(f"[callback] call-status: sid={sid} status={st}")
    if sid and st:
        await engine.on_call_status(sid, st)
    return {"status": "ok"}

@app.post("/callback/recording")
async def callback_recording(
    callSid: str = Form(None),
    recordUrl: str = Form(None),
    **kwargs,
):
    """录音完成回调"""
    sid = callSid or ""
    url = recordUrl or ""
    logger.info(f"[callback] recording: sid={sid} url={url[:60]}")
    if sid and url:
        await engine.on_recording_ready(sid, url)
    return {"status": "ok"}

@app.post("/callback/dtmf")
async def callback_dtmf(
    callSid: str = Form(None),
    dtmf: str = Form(None),
    **kwargs,
):
    """DTMF 按键回调"""
    logger.info(f"[callback] dtmf: sid={callSid} key={dtmf}")
    if dtmf and compliance.check_dtmf_unsubscribe(dtmf):
        call = engine.active_calls.get(callSid)
        if call:
            compliance.handle_unsubscribe(call.callee, reason=f"DTMF退订(按键{dtmf})")
            await engine.hangup(callSid)
    return {"status": "ok"}

# ── Campaign Management ─────────────────────────────────

@app.post("/campaign")
async def campaign_create(req: CampaignCreate):
    """创建外呼任务"""
    campaign = crm.create_campaign(
        name=req.name,
        industry=req.industry,
        script_id=req.script_id,
        daily_limit=req.daily_limit,
        caller_numbers=json.dumps(req.caller_numbers),
        schedule_start=req.schedule_start,
        schedule_end=req.schedule_end,
    )
    return campaign.to_dict()

@app.get("/campaign")
async def campaign_list(status: Optional[str] = None):
    """列出外呼任务"""
    campaigns = crm.list_campaigns(status)
    return {"campaigns": [c.to_dict() for c in campaigns]}

@app.get("/campaign/{campaign_id}")
async def campaign_get(campaign_id: int):
    """获取任务详情"""
    campaign = crm.get_campaign(campaign_id)
    if not campaign:
        raise HTTPException(status_code=404, detail="campaign not found")
    return campaign.to_dict()

@app.post("/campaign/{campaign_id}/start")
async def campaign_start(campaign_id: int, req: CampaignStart):
    """启动外呼任务"""
    campaign = crm.get_campaign(campaign_id)
    if not campaign:
        raise HTTPException(status_code=404, detail="campaign not found")

    prompt = script_engine.build_system_prompt(req.script_industry, req.script_variables)
    await scheduler.start_campaign(
        campaign_id=campaign_id,
        script_prompt=prompt,
        display_num=req.display_num,
        callback_url=CALLBACK_URL,
    )
    return {"status": "started", "campaign_id": campaign_id}

@app.post("/campaign/pause")
async def campaign_pause():
    """暂停外呼"""
    await scheduler.pause_campaign()
    return {"status": "paused"}

@app.post("/campaign/resume")
async def campaign_resume(req: CampaignStart):
    """恢复外呼"""
    prompt = script_engine.build_system_prompt(req.script_industry, req.script_variables)
    await scheduler.resume_campaign(prompt, req.display_num, CALLBACK_URL)
    return {"status": "resumed"}

@app.post("/campaign/stop")
async def campaign_stop():
    """停止外呼"""
    await scheduler.stop_campaign()
    return {"status": "stopped"}

@app.get("/campaign/progress")
async def campaign_progress():
    """外呼进度"""
    return scheduler.get_status()

# ── Customer CRM ────────────────────────────────────────

@app.get("/customers")
async def customer_list(
    campaign_id: Optional[int] = None,
    intent_level: Optional[str] = None,
    status: Optional[str] = None,
    search: Optional[str] = None,
    page: int = 1,
    page_size: int = 20,
):
    customers, total = crm.list_customers(
        campaign_id=campaign_id,
        intent_level=intent_level,
        status=status,
        search=search,
        page=page,
        page_size=page_size,
    )
    return {
        "customers": [c.to_dict() for c in customers],
        "total": total,
        "page": page,
        "page_size": page_size,
    }

@app.get("/customers/{customer_id}")
async def customer_get(customer_id: int):
    customer = crm.get_customer(customer_id)
    if not customer:
        raise HTTPException(status_code=404, detail="customer not found")
    return customer.to_dict()

@app.put("/customers/{customer_id}")
async def customer_update(customer_id: int, req: CustomerUpdate):
    session = crm.get_session()
    try:
        from crm_manager import Customer
        customer = session.query(Customer).get(customer_id)
        if not customer:
            raise HTTPException(status_code=404, detail="customer not found")
        if req.name is not None:
            customer.name = req.name
        if req.intent_level is not None:
            customer.intent_level = req.intent_level
        if req.status is not None:
            customer.status = req.status
        if req.assigned_to is not None:
            customer.assigned_to = req.assigned_to
        if req.next_follow_at is not None:
            customer.next_follow_at = datetime.fromisoformat(req.next_follow_at)
        if req.tags is not None:
            customer.tags = json.dumps(req.tags, ensure_ascii=False)
        customer.updated_at = datetime.utcnow()
        session.commit()
        return customer.to_dict()
    finally:
        session.close()

@app.post("/customers/import")
async def customer_import(
    campaign_id: int = Form(...),
    file: UploadFile = File(...),
):
    """批量导入客户号码 (CSV: phone,name,industry,region)"""
    content = await file.read()
    text = content.decode("utf-8-sig")  # 兼容 Excel 导出的 BOM
    reader = csv.DictReader(io.StringIO(text))

    phones = []
    for row in reader:
        phone = row.get("phone", "").strip()
        if phone:
            phones.append({
                "phone": phone,
                "name": row.get("name", "").strip(),
                "industry": row.get("industry", "").strip(),
                "region": row.get("region", "").strip(),
            })

    if not phones:
        raise HTTPException(status_code=400, detail="no valid phone numbers found")

    result = crm.import_customers(campaign_id, phones)
    return result

# ── Call Records ────────────────────────────────────────

@app.get("/calls")
async def calls_list(
    campaign_id: Optional[int] = None,
    customer_id: Optional[int] = None,
    status: Optional[str] = None,
    page: int = 1,
    page_size: int = 20,
):
    records, total = crm.list_call_records(
        campaign_id=campaign_id,
        customer_id=customer_id,
        status=status,
        page=page,
        page_size=page_size,
    )
    return {
        "calls": [r.to_dict() for r in records],
        "total": total,
        "page": page,
        "page_size": page_size,
    }

@app.get("/calls/{call_id}")
async def calls_get(call_id: int):
    record = crm.get_call_record(call_id)
    if not record:
        raise HTTPException(status_code=404, detail="call not found")
    return record.to_dict()

@app.get("/calls/{call_id}/recording")
async def calls_recording(call_id: int):
    """下载录音文件"""
    record = crm.get_call_record(call_id)
    if not record or not record.recording_path:
        raise HTTPException(status_code=404, detail="recording not found")
    if not os.path.exists(record.recording_path):
        raise HTTPException(status_code=404, detail="recording file missing")
    return FileResponse(
        record.recording_path,
        media_type="audio/wav",
        filename=f"{record.call_sid}.wav",
    )

# ── Scripts ─────────────────────────────────────────────

@app.get("/scripts")
async def scripts_list(industry: Optional[str] = None):
    """列出话术模板"""
    db_scripts = crm.list_scripts(industry)
    builtin = script_engine.list_scripts()
    return {
        "scripts": [s.to_dict() for s in db_scripts],
        "builtin": builtin,
    }

@app.post("/scripts")
async def scripts_create(req: ScriptCreate):
    """创建自定义话术"""
    script = crm.create_script(
        name=req.name,
        industry=req.industry,
        greeting=req.greeting,
        key_points=json.dumps(req.key_points, ensure_ascii=False),
        qa_library=json.dumps(req.qa_library, ensure_ascii=False),
        objections=json.dumps(req.objections, ensure_ascii=False),
        closing=json.dumps(req.closing, ensure_ascii=False),
        voice=req.voice,
    )
    return script.to_dict()

@app.get("/scripts/builtin")
async def scripts_builtin():
    """列出内置话术模板"""
    return {"scripts": script_engine.list_scripts()}

@app.get("/scripts/builtin/{industry}")
async def scripts_builtin_detail(industry: str):
    """获取内置话术详情"""
    script = script_engine.get_script(industry)
    return script

# ── Analytics ───────────────────────────────────────────

@app.get("/analytics/overview")
async def analytics_overview(campaign_id: Optional[int] = None):
    return crm.get_overview(campaign_id)

@app.get("/analytics/intent-levels")
async def analytics_intent_levels():
    """意向等级定义"""
    return {"levels": INTENT_LEVELS}

# ── Recordings ──────────────────────────────────────────

@app.get("/recordings")
async def recordings_list(date: Optional[str] = None):
    """列出录音文件"""
    return {"recordings": recorder.list_recordings(date)}

@app.get("/recordings/usage")
async def recordings_usage():
    """录音存储用量"""
    return recorder.get_disk_usage()

# ── Compliance ──────────────────────────────────────────

@app.get("/compliance/stats")
async def compliance_stats():
    return compliance.get_stats()

@app.post("/compliance/blacklist")
async def compliance_blacklist_add(phone: str = Form(...), reason: str = Form("manual")):
    """手动加入黑名单"""
    compliance.handle_unsubscribe(phone, reason)
    return {"status": "blacklisted"}

# ── Entry Point ─────────────────────────────────────────

if __name__ == "__main__":
    uvicorn.run(
        "main:app",
        host=SERVER_HOST,
        port=SERVER_PORT,
        reload=True,
        log_level="info",
    )
