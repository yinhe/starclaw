"""
Cicada 🪰 CRM Manager — 客户管理 + SQLite 持久化
"""

import json
import hashlib
from datetime import datetime, timedelta
from typing import Optional
from enum import Enum

from sqlalchemy import (
    create_engine, Column, Integer, String, Text, Float,
    DateTime, Boolean, ForeignKey, Index, event
)
from sqlalchemy.orm import declarative_base, sessionmaker, Session, relationship

Base = declarative_base()


class IntentLevel(str, Enum):
    A = "A"  # 强意向
    B = "B"  # 较强意向
    C = "C"  # 一般意向
    D = "D"  # 弱意向
    E = "E"  # 明确拒绝
    F = "F"  # 无效号码


class CustomerStatus(str, Enum):
    PENDING = "pending"
    ACTIVE = "active"
    CONVERTED = "converted"
    BLACKLISTED = "blacklisted"


class CampaignStatus(str, Enum):
    DRAFT = "draft"
    RUNNING = "running"
    PAUSED = "paused"
    COMPLETED = "completed"


class CallStatus(str, Enum):
    DIALING = "dialing"
    RINGING = "ringing"
    CONNECTED = "connected"
    TALKING = "talking"
    HANGUP = "hangup"
    NO_ANSWER = "no_answer"
    REJECTED = "rejected"
    FAILED = "failed"
    TRANSFERRED = "transferred"
    ERROR = "error"


# ── Models ──────────────────────────────────────────────

class Customer(Base):
    __tablename__ = "customers"

    id = Column(Integer, primary_key=True, autoincrement=True)
    campaign_id = Column(Integer, ForeignKey("campaigns.id"), index=True)
    phone = Column(String(200))  # 加密存储
    phone_hash = Column(String(64), unique=True, index=True)
    name = Column(String(50), default="")
    industry = Column(String(30), default="")
    region = Column(String(30), default="")
    intent_level = Column(String(1), default="", index=True)
    intent_score = Column(Integer, default=0)
    tags = Column(Text, default="[]")
    key_interests = Column(Text, default="[]")
    summary = Column(Text, default="")
    total_calls = Column(Integer, default=0)
    last_call_at = Column(DateTime, nullable=True)
    next_follow_at = Column(DateTime, nullable=True)
    status = Column(String(20), default=CustomerStatus.PENDING)
    assigned_to = Column(String(50), default="")
    source = Column(String(30), default="")
    created_at = Column(DateTime, default=datetime.utcnow)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)

    calls = relationship("CallRecord", back_populates="customer", lazy="dynamic")

    def to_dict(self):
        return {
            "id": self.id,
            "campaign_id": self.campaign_id,
            "name": self.name,
            "phone_masked": self._mask_phone(),
            "industry": self.industry,
            "region": self.region,
            "intent_level": self.intent_level,
            "intent_score": self.intent_score,
            "tags": json.loads(self.tags) if self.tags else [],
            "key_interests": json.loads(self.key_interests) if self.key_interests else [],
            "summary": self.summary,
            "total_calls": self.total_calls,
            "last_call_at": self.last_call_at.isoformat() if self.last_call_at else None,
            "next_follow_at": self.next_follow_at.isoformat() if self.next_follow_at else None,
            "status": self.status,
            "assigned_to": self.assigned_to,
            "source": self.source,
            "created_at": self.created_at.isoformat() if self.created_at else None,
        }

    def _mask_phone(self) -> str:
        if not self.phone or len(self.phone) < 7:
            return "***"
        return self.phone[:3] + "****" + self.phone[-4:]


class CallRecord(Base):
    __tablename__ = "call_records"

    id = Column(Integer, primary_key=True, autoincrement=True)
    customer_id = Column(Integer, ForeignKey("customers.id"), index=True)
    campaign_id = Column(Integer, ForeignKey("campaigns.id"), index=True)
    call_sid = Column(String(64), unique=True, index=True)
    caller_number = Column(String(20), default="")
    callee_number = Column(String(20), default="")
    direction = Column(String(10), default="outbound")
    status = Column(String(20), default=CallStatus.DIALING)
    duration = Column(Integer, default=0)
    intent_level = Column(String(1), default="")
    intent_score = Column(Integer, default=0)
    transcript = Column(Text, default="")
    summary = Column(Text, default="")
    recording_url = Column(String(500), default="")
    recording_path = Column(String(500), default="")
    ai_analysis = Column(Text, default="{}")
    started_at = Column(DateTime, default=datetime.utcnow)
    ended_at = Column(DateTime, nullable=True)
    created_at = Column(DateTime, default=datetime.utcnow)

    customer = relationship("Customer", back_populates="calls")

    def to_dict(self):
        return {
            "id": self.id,
            "customer_id": self.customer_id,
            "campaign_id": self.campaign_id,
            "call_sid": self.call_sid,
            "direction": self.direction,
            "status": self.status,
            "duration": self.duration,
            "intent_level": self.intent_level,
            "intent_score": self.intent_score,
            "transcript": self.transcript,
            "summary": self.summary,
            "recording_url": self.recording_url,
            "recording_path": self.recording_path,
            "ai_analysis": json.loads(self.ai_analysis) if self.ai_analysis else {},
            "started_at": self.started_at.isoformat() if self.started_at else None,
            "ended_at": self.ended_at.isoformat() if self.ended_at else None,
        }


class Campaign(Base):
    __tablename__ = "campaigns"

    id = Column(Integer, primary_key=True, autoincrement=True)
    name = Column(String(100))
    industry = Column(String(30), default="")
    script_id = Column(Integer, ForeignKey("scripts.id"), nullable=True)
    status = Column(String(20), default=CampaignStatus.DRAFT)
    total_numbers = Column(Integer, default=0)
    called_count = Column(Integer, default=0)
    connected_count = Column(Integer, default=0)
    intent_a_count = Column(Integer, default=0)
    intent_b_count = Column(Integer, default=0)
    daily_limit = Column(Integer, default=800)
    caller_numbers = Column(Text, default="[]")
    schedule_start = Column(String(5), default="09:00")
    schedule_end = Column(String(5), default="18:00")
    schedule_days = Column(String(20), default="1,2,3,4,5,6")
    created_at = Column(DateTime, default=datetime.utcnow)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)

    def to_dict(self):
        return {
            "id": self.id,
            "name": self.name,
            "industry": self.industry,
            "script_id": self.script_id,
            "status": self.status,
            "total_numbers": self.total_numbers,
            "called_count": self.called_count,
            "connected_count": self.connected_count,
            "intent_a_count": self.intent_a_count,
            "intent_b_count": self.intent_b_count,
            "daily_limit": self.daily_limit,
            "caller_numbers": json.loads(self.caller_numbers) if self.caller_numbers else [],
            "schedule_start": self.schedule_start,
            "schedule_end": self.schedule_end,
            "schedule_days": self.schedule_days,
            "connect_rate": round(self.connected_count / max(self.called_count, 1) * 100, 1),
            "created_at": self.created_at.isoformat() if self.created_at else None,
        }


class Script(Base):
    __tablename__ = "scripts"

    id = Column(Integer, primary_key=True, autoincrement=True)
    name = Column(String(100))
    industry = Column(String(30), default="")
    greeting = Column(Text, default="")
    key_points = Column(Text, default="[]")
    qa_library = Column(Text, default="[]")
    objections = Column(Text, default="[]")
    closing = Column(Text, default="{}")
    voice = Column(String(30), default="longxiaochun")
    is_builtin = Column(Boolean, default=False)
    created_at = Column(DateTime, default=datetime.utcnow)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)

    def to_dict(self):
        return {
            "id": self.id,
            "name": self.name,
            "industry": self.industry,
            "greeting": self.greeting,
            "key_points": json.loads(self.key_points) if self.key_points else [],
            "qa_library": json.loads(self.qa_library) if self.qa_library else [],
            "objections": json.loads(self.objections) if self.objections else [],
            "closing": json.loads(self.closing) if self.closing else {},
            "voice": self.voice,
            "is_builtin": self.is_builtin,
            "created_at": self.created_at.isoformat() if self.created_at else None,
        }


class Blacklist(Base):
    __tablename__ = "blacklist"

    id = Column(Integer, primary_key=True, autoincrement=True)
    phone_hash = Column(String(64), unique=True, index=True)
    reason = Column(String(200), default="")
    created_at = Column(DateTime, default=datetime.utcnow)
    expires_at = Column(DateTime, nullable=True)


# ── CRM Manager ────────────────────────────────────────

class CRMManager:
    def __init__(self, db_path: str):
        self.engine = create_engine(f"sqlite:///{db_path}", echo=False)
        Base.metadata.create_all(self.engine)
        self.SessionLocal = sessionmaker(bind=self.engine)

    def get_session(self) -> Session:
        return self.SessionLocal()

    @staticmethod
    def hash_phone(phone: str) -> str:
        return hashlib.sha256(phone.strip().encode()).hexdigest()

    # ── Customer CRUD ──

    def create_customer(self, phone: str, campaign_id: int, **kwargs) -> Customer:
        session = self.get_session()
        try:
            phone_hash = self.hash_phone(phone)
            existing = session.query(Customer).filter_by(phone_hash=phone_hash).first()
            if existing:
                return existing

            customer = Customer(
                phone=phone,
                phone_hash=phone_hash,
                campaign_id=campaign_id,
                name=kwargs.get("name", ""),
                industry=kwargs.get("industry", ""),
                region=kwargs.get("region", ""),
                source=kwargs.get("source", "import"),
            )
            session.add(customer)
            session.commit()
            session.refresh(customer)
            return customer
        finally:
            session.close()

    def get_customer(self, customer_id: int) -> Optional[Customer]:
        session = self.get_session()
        try:
            return session.query(Customer).get(customer_id)
        finally:
            session.close()

    def list_customers(
        self,
        campaign_id: Optional[int] = None,
        intent_level: Optional[str] = None,
        status: Optional[str] = None,
        search: Optional[str] = None,
        page: int = 1,
        page_size: int = 20,
    ) -> tuple[list[Customer], int]:
        session = self.get_session()
        try:
            q = session.query(Customer)
            if campaign_id:
                q = q.filter(Customer.campaign_id == campaign_id)
            if intent_level:
                q = q.filter(Customer.intent_level == intent_level)
            if status:
                q = q.filter(Customer.status == status)
            if search:
                q = q.filter(Customer.name.contains(search))

            total = q.count()
            customers = q.order_by(Customer.intent_score.desc()).offset(
                (page - 1) * page_size
            ).limit(page_size).all()
            return customers, total
        finally:
            session.close()

    def update_customer_intent(
        self, customer_id: int, level: str, score: int,
        key_interests: list[str] = None, summary: str = ""
    ):
        session = self.get_session()
        try:
            customer = session.query(Customer).get(customer_id)
            if not customer:
                return
            customer.intent_level = level
            customer.intent_score = score
            if key_interests:
                customer.key_interests = json.dumps(key_interests, ensure_ascii=False)
            if summary:
                customer.summary = summary
            customer.updated_at = datetime.utcnow()
            session.commit()
        finally:
            session.close()

    def increment_call_count(self, customer_id: int):
        session = self.get_session()
        try:
            customer = session.query(Customer).get(customer_id)
            if customer:
                customer.total_calls += 1
                customer.last_call_at = datetime.utcnow()
                session.commit()
        finally:
            session.close()

    def import_customers(self, campaign_id: int, phones: list[dict]) -> dict:
        """批量导入号码. phones = [{"phone": "13800138000", "name": "张三"}, ...]"""
        session = self.get_session()
        imported = 0
        duplicates = 0
        try:
            for item in phones:
                phone = item.get("phone", "").strip()
                if not phone:
                    continue
                phone_hash = self.hash_phone(phone)
                existing = session.query(Customer).filter_by(phone_hash=phone_hash).first()
                if existing:
                    duplicates += 1
                    continue
                customer = Customer(
                    phone=phone,
                    phone_hash=phone_hash,
                    campaign_id=campaign_id,
                    name=item.get("name", ""),
                    industry=item.get("industry", ""),
                    region=item.get("region", ""),
                    source="import",
                )
                session.add(customer)
                imported += 1

            session.commit()

            # update campaign total
            campaign = session.query(Campaign).get(campaign_id)
            if campaign:
                campaign.total_numbers = session.query(Customer).filter_by(
                    campaign_id=campaign_id
                ).count()
                session.commit()

            return {"imported": imported, "duplicates": duplicates, "total": imported + duplicates}
        finally:
            session.close()

    # ── Call Record ──

    def create_call_record(self, **kwargs) -> CallRecord:
        session = self.get_session()
        try:
            record = CallRecord(**kwargs)
            session.add(record)
            session.commit()
            session.refresh(record)
            return record
        finally:
            session.close()

    def update_call_record(self, call_sid: str, **kwargs):
        session = self.get_session()
        try:
            record = session.query(CallRecord).filter_by(call_sid=call_sid).first()
            if not record:
                return
            for key, value in kwargs.items():
                if hasattr(record, key):
                    setattr(record, key, value)
            session.commit()
        finally:
            session.close()

    def get_call_record(self, call_id: int) -> Optional[CallRecord]:
        session = self.get_session()
        try:
            return session.query(CallRecord).get(call_id)
        finally:
            session.close()

    def list_call_records(
        self,
        campaign_id: Optional[int] = None,
        customer_id: Optional[int] = None,
        status: Optional[str] = None,
        page: int = 1,
        page_size: int = 20,
    ) -> tuple[list[CallRecord], int]:
        session = self.get_session()
        try:
            q = session.query(CallRecord)
            if campaign_id:
                q = q.filter(CallRecord.campaign_id == campaign_id)
            if customer_id:
                q = q.filter(CallRecord.customer_id == customer_id)
            if status:
                q = q.filter(CallRecord.status == status)

            total = q.count()
            records = q.order_by(CallRecord.created_at.desc()).offset(
                (page - 1) * page_size
            ).limit(page_size).all()
            return records, total
        finally:
            session.close()

    # ── Campaign ──

    def create_campaign(self, **kwargs) -> Campaign:
        session = self.get_session()
        try:
            campaign = Campaign(**kwargs)
            session.add(campaign)
            session.commit()
            session.refresh(campaign)
            return campaign
        finally:
            session.close()

    def get_campaign(self, campaign_id: int) -> Optional[Campaign]:
        session = self.get_session()
        try:
            return session.query(Campaign).get(campaign_id)
        finally:
            session.close()

    def list_campaigns(self, status: Optional[str] = None) -> list[Campaign]:
        session = self.get_session()
        try:
            q = session.query(Campaign)
            if status:
                q = q.filter(Campaign.status == status)
            return q.order_by(Campaign.created_at.desc()).all()
        finally:
            session.close()

    def update_campaign_status(self, campaign_id: int, status: str):
        session = self.get_session()
        try:
            campaign = session.query(Campaign).get(campaign_id)
            if campaign:
                campaign.status = status
                campaign.updated_at = datetime.utcnow()
                session.commit()
        finally:
            session.close()

    def update_campaign_stats(self, campaign_id: int, called: int = 0, connected: int = 0,
                               intent_a: int = 0, intent_b: int = 0):
        session = self.get_session()
        try:
            campaign = session.query(Campaign).get(campaign_id)
            if campaign:
                campaign.called_count += called
                campaign.connected_count += connected
                campaign.intent_a_count += intent_a
                campaign.intent_b_count += intent_b
                session.commit()
        finally:
            session.close()

    # ── Script ──

    def create_script(self, **kwargs) -> Script:
        session = self.get_session()
        try:
            script = Script(**kwargs)
            session.add(script)
            session.commit()
            session.refresh(script)
            return script
        finally:
            session.close()

    def get_script(self, script_id: int) -> Optional[Script]:
        session = self.get_session()
        try:
            return session.query(Script).get(script_id)
        finally:
            session.close()

    def list_scripts(self, industry: Optional[str] = None) -> list[Script]:
        session = self.get_session()
        try:
            q = session.query(Script)
            if industry:
                q = q.filter(Script.industry == industry)
            return q.order_by(Script.created_at.desc()).all()
        finally:
            session.close()

    # ── Blacklist ──

    def add_to_blacklist(self, phone: str, reason: str = "unsubscribe"):
        session = self.get_session()
        try:
            phone_hash = self.hash_phone(phone)
            existing = session.query(Blacklist).filter_by(phone_hash=phone_hash).first()
            if existing:
                return
            bl = Blacklist(phone_hash=phone_hash, reason=reason)
            session.add(bl)

            # also mark customer as blacklisted
            customer = session.query(Customer).filter_by(phone_hash=phone_hash).first()
            if customer:
                customer.status = CustomerStatus.BLACKLISTED
                customer.updated_at = datetime.utcnow()

            session.commit()
        finally:
            session.close()

    def is_blacklisted(self, phone: str) -> bool:
        session = self.get_session()
        try:
            phone_hash = self.hash_phone(phone)
            bl = session.query(Blacklist).filter_by(phone_hash=phone_hash).first()
            if not bl:
                return False
            if bl.expires_at and bl.expires_at < datetime.utcnow():
                return False
            return True
        finally:
            session.close()

    # ── Analytics ──

    def get_overview(self, campaign_id: Optional[int] = None) -> dict:
        session = self.get_session()
        try:
            cq = session.query(Customer)
            rq = session.query(CallRecord)
            if campaign_id:
                cq = cq.filter(Customer.campaign_id == campaign_id)
                rq = rq.filter(CallRecord.campaign_id == campaign_id)

            total_customers = cq.count()
            total_calls = rq.count()
            connected_calls = rq.filter(CallRecord.status == CallStatus.CONNECTED).count()
            talking_calls = rq.filter(CallRecord.status == CallStatus.HANGUP).count()

            intent_dist = {}
            for level in IntentLevel:
                intent_dist[level.value] = cq.filter(
                    Customer.intent_level == level.value
                ).count()

            return {
                "total_customers": total_customers,
                "total_calls": total_calls,
                "connected_calls": connected_calls + talking_calls,
                "connect_rate": round(
                    (connected_calls + talking_calls) / max(total_calls, 1) * 100, 1
                ),
                "intent_distribution": intent_dist,
            }
        finally:
            session.close()

    def get_pending_calls(self, campaign_id: int, batch_size: int = 10) -> list[Customer]:
        """获取待拨打的客户列表"""
        session = self.get_session()
        try:
            today = datetime.utcnow().date()
            today_start = datetime.combine(today, datetime.min.time())

            # 已经今天拨过的客户ID
            called_today = session.query(CallRecord.customer_id).filter(
                CallRecord.campaign_id == campaign_id,
                CallRecord.created_at >= today_start,
            ).subquery()

            # 获取未拨打的、非黑名单的客户
            customers = session.query(Customer).filter(
                Customer.campaign_id == campaign_id,
                Customer.status != CustomerStatus.BLACKLISTED,
                Customer.status != CustomerStatus.CONVERTED,
                ~Customer.id.in_(called_today),
            ).order_by(Customer.id.asc()).limit(batch_size).all()

            return customers
        finally:
            session.close()
