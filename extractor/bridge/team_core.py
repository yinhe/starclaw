"""
Team Core — 团队智能体基础模型。

每个Agent拥有七元组属性:
  1. 基因 (Gene)     — 先天性格/偏好，决定决策风格，排第一
  2. 技能 (Skills)   — 可调用的工具/能力
  3. 本能 (Instincts) — 规则引擎，零LLM成本的自动触发条件
  4. MCP外接 (MCP)    — 外部数据/工具连接
  5. 工作流 (Workflows) — 步骤化执行流程
  6. 记忆 (Memory)   — 持久化知识和统计
  7. 腺体 (Glands)   — 动态状态修饰器（类似激素，影响风险偏好）

团队决策通过结构化事件总线（team_decisions表）协作。
"""

import json
import logging
import sqlite3
import os
import time as time_module
from dataclasses import dataclass, field, asdict
from datetime import datetime, date
from enum import Enum
from typing import Any, Dict, List, Optional

logger = logging.getLogger("team")

DB_PATH = os.path.join(os.path.dirname(os.path.abspath(__file__)), "trades.db")


# ═══════════════ Enums ═══════════════

class AgentRole(str, Enum):
    LEADER = "leader"
    MACRO = "macro"
    HUNTER = "hunter"
    RISK = "risk"
    COACH = "coach"


class AgentStatus(str, Enum):
    IDLE = "idle"
    SCANNING = "scanning"
    ANALYZING = "analyzing"
    APPROVING = "approving"
    VETOING = "vetoing"
    EXECUTING = "executing"
    ARBITRATING = "arbitrating"
    REVIEWING = "reviewing"
    ALERTING = "alerting"


class GeneType(str, Enum):
    BALANCED = "balanced"
    DEFENSIVE = "defensive"
    AGGRESSIVE = "aggressive"
    PARANOID = "paranoid"
    ANALYTICAL = "analytical"


class GlandState(str, Enum):
    CALM = "calm"
    FEARFUL = "fearful"
    GREEDY = "greedy"
    ALERT = "alert"
    FROZEN = "frozen"      # coach: immune to emotion


# ═══════════════ Data Models ═══════════════

@dataclass
class Gene:
    """先天基因 — 决定Agent的核心风格，难以改变。"""
    gene_type: str = "balanced"
    risk_tolerance: float = 0.5      # 0=极保守, 1=极激进
    confidence_bias: float = 0.0     # >0=偏乐观, <0=偏悲观
    veto_rate_cap: float = 1.0       # 否决率上限 (风控官用)
    min_approval_rate: float = 0.0   # 最低批准率 (风控官用)
    creativity: float = 0.5          # 创造力 (猎手用)
    patience: float = 0.5            # 耐心度 (执行用)
    objectivity: float = 0.5         # 客观性 (教练用)

    def to_dict(self) -> dict:
        return asdict(self)


@dataclass
class Gland:
    """腺体 — 动态激素状态，随盈亏实时变化。"""
    state: str = "calm"
    cortisol: float = 0.0       # 恐惧激素 (0-100), 亏损时升高
    dopamine: float = 50.0      # 满足激素 (0-100), 盈利时升高
    adrenaline: float = 0.0     # 肾上腺素 (0-100), 波动大时升高
    inertia: float = 0.5        # 惰性 (0-1), 越高越不易波动
    frozen: bool = False         # 冻结 (教练专用)

    def update(self, daily_pnl_pct: float, volatility: float = 0):
        """根据当日盈亏和波动率更新激素水平。"""
        if self.frozen:
            return

        # Cortisol: 亏损越大越恐惧
        if daily_pnl_pct < 0:
            target_cortisol = min(100, abs(daily_pnl_pct) * 50)
        else:
            target_cortisol = max(0, self.cortisol - 10)
        self.cortisol += (target_cortisol - self.cortisol) * (1 - self.inertia)

        # Dopamine: 盈利越大越满足
        if daily_pnl_pct > 0:
            target_dopamine = min(100, 50 + daily_pnl_pct * 30)
        else:
            target_dopamine = max(0, 50 + daily_pnl_pct * 20)
        self.dopamine += (target_dopamine - self.dopamine) * (1 - self.inertia)

        # Adrenaline: 波动大时升高
        self.adrenaline = min(100, volatility * 10)

        # Derive state
        if self.cortisol > 70:
            self.state = "fearful"
        elif self.dopamine > 80:
            self.state = "greedy"
        elif self.adrenaline > 60:
            self.state = "alert"
        else:
            self.state = "calm"

    def risk_modifier(self) -> float:
        """返回风险调整系数: <1.0=收紧, >1.0=放松。"""
        if self.frozen:
            return 1.0
        # High cortisol = tighter risk; high dopamine = slightly looser
        modifier = 1.0 - (self.cortisol / 200) + (self.dopamine - 50) / 400
        return max(0.3, min(1.5, modifier))

    def to_dict(self) -> dict:
        return asdict(self)


@dataclass
class AgentConfig:
    """智能体完整配置。"""
    role: str
    name: str
    avatar: str                         # emoji avatar
    gene: Gene = field(default_factory=Gene)
    gland: Gland = field(default_factory=Gland)
    skills: List[str] = field(default_factory=list)
    instincts: List[str] = field(default_factory=list)
    mcp: List[str] = field(default_factory=list)
    workflows: List[str] = field(default_factory=list)
    memory_keys: List[str] = field(default_factory=list)

    def to_dict(self) -> dict:
        return {
            "role": self.role,
            "name": self.name,
            "avatar": self.avatar,
            "gene": self.gene.to_dict(),
            "gland": self.gland.to_dict(),
            "skills": self.skills,
            "instincts": self.instincts,
            "mcp": self.mcp,
            "workflows": self.workflows,
            "memory_keys": self.memory_keys,
        }


# ═══════════════ Decision Event ═══════════════

@dataclass
class Decision:
    """一条团队决策记录。"""
    agent: str
    action: str              # propose_buy / veto / approve / stop_loss / arbitrate / ...
    target: str = ""         # stock code or 'portfolio'
    confidence: float = 0.0  # 0-100
    reasoning: str = ""
    data: str = ""           # JSON extra data
    outcome: str = "pending" # pending / profit / loss / cancelled
    pnl: float = 0.0
    ts: str = ""

    def __post_init__(self):
        if not self.ts:
            self.ts = datetime.now().strftime("%Y-%m-%d %H:%M:%S")


# ═══════════════ Decision Store ═══════════════

class DecisionStore:
    """持久化团队决策到SQLite。"""

    def __init__(self, db_path: str = DB_PATH):
        self._db = db_path
        self._init_db()

    def _init_db(self):
        with sqlite3.connect(self._db) as conn:
            conn.execute("""
                CREATE TABLE IF NOT EXISTS team_decisions (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    ts TEXT NOT NULL,
                    agent TEXT NOT NULL,
                    action TEXT NOT NULL,
                    target TEXT DEFAULT '',
                    confidence REAL DEFAULT 0,
                    reasoning TEXT DEFAULT '',
                    data TEXT DEFAULT '',
                    outcome TEXT DEFAULT 'pending',
                    pnl REAL DEFAULT 0
                )
            """)
            conn.execute("""
                CREATE INDEX IF NOT EXISTS idx_td_agent ON team_decisions(agent)
            """)
            conn.execute("""
                CREATE INDEX IF NOT EXISTS idx_td_ts ON team_decisions(ts)
            """)

    def record(self, d: Decision) -> int:
        """Record a decision, return its ID."""
        with sqlite3.connect(self._db) as conn:
            cur = conn.execute(
                "INSERT INTO team_decisions (ts, agent, action, target, confidence, reasoning, data, outcome, pnl) "
                "VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
                (d.ts, d.agent, d.action, d.target, d.confidence, d.reasoning, d.data, d.outcome, d.pnl),
            )
            return cur.lastrowid

    def update_outcome(self, decision_id: int, outcome: str, pnl: float = 0):
        with sqlite3.connect(self._db) as conn:
            conn.execute(
                "UPDATE team_decisions SET outcome=?, pnl=? WHERE id=?",
                (outcome, pnl, decision_id),
            )

    def recent(self, limit: int = 50, agent: str = None) -> List[dict]:
        with sqlite3.connect(self._db) as conn:
            conn.row_factory = sqlite3.Row
            if agent:
                rows = conn.execute(
                    "SELECT * FROM team_decisions WHERE agent=? ORDER BY id DESC LIMIT ?",
                    (agent, limit),
                ).fetchall()
            else:
                rows = conn.execute(
                    "SELECT * FROM team_decisions ORDER BY id DESC LIMIT ?", (limit,)
                ).fetchall()
            return [dict(r) for r in rows]

    def today(self) -> List[dict]:
        today_str = date.today().strftime("%Y-%m-%d")
        with sqlite3.connect(self._db) as conn:
            conn.row_factory = sqlite3.Row
            rows = conn.execute(
                "SELECT * FROM team_decisions WHERE ts >= ? ORDER BY id ASC",
                (today_str,),
            ).fetchall()
            return [dict(r) for r in rows]

    def agent_accuracy(self, agent: str, days: int = 30) -> float:
        """计算Agent过去N天的决策准确率。"""
        with sqlite3.connect(self._db) as conn:
            conn.row_factory = sqlite3.Row
            rows = conn.execute(
                "SELECT outcome FROM team_decisions "
                "WHERE agent=? AND outcome IN ('profit','loss') "
                "AND ts >= date('now', ?)",
                (agent, f"-{days} days"),
            ).fetchall()
            if not rows:
                return 0.5  # default 50% if no history
            wins = sum(1 for r in rows if r["outcome"] == "profit")
            return wins / len(rows)

    def multi_accuracy(self, agent: str) -> dict:
        """Return accuracy over 7d, 14d, 30d windows."""
        result = {}
        for days in (7, 14, 30):
            result[f"{days}d"] = round(self.agent_accuracy(agent, days), 4)
        return result

    def cleanup(self, keep_days: int = 90) -> int:
        """Delete decisions older than keep_days. Returns count deleted."""
        with sqlite3.connect(self._db) as conn:
            cur = conn.execute(
                "DELETE FROM team_decisions WHERE ts < date('now', ?)",
                (f"-{keep_days} days",),
            )
            deleted = cur.rowcount
            if deleted:
                conn.execute("VACUUM")
            return deleted

    def agent_stats(self, agent: str) -> dict:
        """Agent决策统计。"""
        with sqlite3.connect(self._db) as conn:
            conn.row_factory = sqlite3.Row
            rows = conn.execute(
                "SELECT action, COUNT(*) as cnt FROM team_decisions "
                "WHERE agent=? AND ts >= date('now', '-30 days') GROUP BY action",
                (agent,),
            ).fetchall()
            stats = {r["action"]: r["cnt"] for r in rows}

            # Veto rate for risk officer
            total_reviews = stats.get("approve", 0) + stats.get("veto", 0) + stats.get("reduce_size", 0)
            veto_count = stats.get("veto", 0)
            stats["veto_rate"] = veto_count / max(total_reviews, 1)
            stats["accuracy"] = self.agent_accuracy(agent)
            return stats


# ═══════════════ Agent Memory ═══════════════

class AgentMemory:
    """Agent持久化记忆 — SQLite存储，跨日累积经验。"""

    def __init__(self, db_path: str = DB_PATH):
        self._db = db_path
        self._init_db()
        self._cache: Dict[str, dict] = {}

    def _init_db(self):
        with sqlite3.connect(self._db) as conn:
            conn.execute("""
                CREATE TABLE IF NOT EXISTS agent_memory (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    agent TEXT NOT NULL,
                    key TEXT NOT NULL,
                    value TEXT NOT NULL,
                    updated_at TEXT NOT NULL,
                    UNIQUE(agent, key)
                )
            """)
            conn.execute("""
                CREATE INDEX IF NOT EXISTS idx_am_agent ON agent_memory(agent)
            """)

    def get(self, agent: str, key: str, default: Any = None) -> Any:
        cache_key = f"{agent}:{key}"
        if cache_key in self._cache:
            return self._cache[cache_key]
        with sqlite3.connect(self._db) as conn:
            row = conn.execute(
                "SELECT value FROM agent_memory WHERE agent=? AND key=?",
                (agent, key),
            ).fetchone()
            if row:
                try:
                    val = json.loads(row[0])
                except (json.JSONDecodeError, TypeError):
                    val = row[0]
                self._cache[cache_key] = val
                return val
        return default

    def set(self, agent: str, key: str, value: Any):
        ts = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
        val_str = json.dumps(value, ensure_ascii=False) if not isinstance(value, str) else value
        with sqlite3.connect(self._db) as conn:
            conn.execute(
                "INSERT INTO agent_memory (agent, key, value, updated_at) "
                "VALUES (?, ?, ?, ?) "
                "ON CONFLICT(agent, key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at",
                (agent, key, val_str, ts),
            )
        self._cache[f"{agent}:{key}"] = value

    def incr(self, agent: str, key: str, delta: float = 1) -> float:
        """原子自增。"""
        cur = self.get(agent, key, 0)
        new_val = cur + delta
        self.set(agent, key, new_val)
        return new_val

    def append_list(self, agent: str, key: str, item: Any, max_len: int = 100):
        """向列表记忆追加条目，保留最近max_len条。"""
        lst = self.get(agent, key, [])
        if not isinstance(lst, list):
            lst = []
        lst.append(item)
        if len(lst) > max_len:
            lst = lst[-max_len:]
        self.set(agent, key, lst)

    def get_all(self, agent: str) -> dict:
        """获取Agent全部记忆。"""
        with sqlite3.connect(self._db) as conn:
            conn.row_factory = sqlite3.Row
            rows = conn.execute(
                "SELECT key, value, updated_at FROM agent_memory WHERE agent=?",
                (agent,),
            ).fetchall()
            result = {}
            for r in rows:
                try:
                    result[r["key"]] = {"value": json.loads(r["value"]), "updated_at": r["updated_at"]}
                except (json.JSONDecodeError, TypeError):
                    result[r["key"]] = {"value": r["value"], "updated_at": r["updated_at"]}
            return result


# ═══════════════ Base Agent ═══════════════

class BaseAgent:
    """团队智能体基类。"""

    def __init__(self, config: AgentConfig, decision_store: DecisionStore,
                 memory: AgentMemory = None):
        self.config = config
        self.role = config.role
        self.name = config.name
        self.gene = config.gene
        self.gland = config.gland
        self.decisions = decision_store
        self.memory = memory or AgentMemory()
        self._status = AgentStatus.IDLE
        self._last_action = ""
        self._last_action_ts = ""

    @property
    def status(self) -> str:
        return self._status.value

    def set_status(self, s: AgentStatus):
        self._status = s

    def log_decision(self, action: str, target: str = "", confidence: float = 0,
                     reasoning: str = "", data: dict = None) -> int:
        """记录一条决策。"""
        d = Decision(
            agent=self.role,
            action=action,
            target=target,
            confidence=confidence,
            reasoning=reasoning,
            data=json.dumps(data or {}, ensure_ascii=False),
        )
        self._last_action = action
        self._last_action_ts = d.ts
        did = self.decisions.record(d)
        logger.info(f"[{self.role}] {action} {target} conf={confidence:.0f} — {reasoning}")
        return did

    def get_state(self) -> dict:
        """返回当前Agent完整状态（用于前端展示）。"""
        stats = self.decisions.agent_stats(self.role)
        return {
            **self.config.to_dict(),
            "status": self._status.value,
            "last_action": self._last_action,
            "last_action_ts": self._last_action_ts,
            "stats": stats,
        }


# ═══════════════ Default Team Configs ═══════════════

def default_team_configs() -> Dict[str, AgentConfig]:
    """返回默认的5个Agent配置。"""
    return {
        AgentRole.LEADER: AgentConfig(
            role="leader", name="组长", avatar="🧠",
            gene=Gene(gene_type="balanced", risk_tolerance=0.5, confidence_bias=0,
                      patience=0.7, objectivity=0.8),
            gland=Gland(inertia=0.8),  # 高惰性=不易波动
            skills=["portfolio_allocate", "resolve_conflict", "emergency_mode", "daily_pnl_check"],
            instincts=[
                "日亏≥0.3% → 启动保本模式",
                "盈利≥1% → 允许猎手加码",
                "Agent信心差>30 → LLM仲裁",
            ],
            mcp=["QMT账户", "SQLite决策日志", "Qwen(仅仲裁)"],
            workflows=["晨会→分配预算→审批交易→盘中巡检→签收复盘"],
            memory_keys=["agent_accuracy", "arbitration_precedents", "daily_targets"],
        ),
        AgentRole.MACRO: AgentConfig(
            role="macro", name="宏观分析师", avatar="🔭",
            gene=Gene(gene_type="defensive", risk_tolerance=0.3, confidence_bias=-0.1),
            gland=Gland(inertia=0.6),
            skills=["market_env_detect", "breadth_analysis", "sentiment_score", "index_direction"],
            instincts=[
                "涨跌比<0.5 → 一级风险警报",
                "情绪<30 → 一级风险警报",
                "宽度<15% → 禁止新开仓",
            ],
            mcp=["eastmoney宏观", "alpha_engine(L1-L3)"],
            workflows=["08:30产出市场评级→每30min更新→重大变化通知组长"],
            memory_keys=["env_accuracy", "signal_effectiveness"],
        ),
        AgentRole.HUNTER: AgentConfig(
            role="hunter", name="选股猎手", avatar="🎯",
            gene=Gene(gene_type="aggressive", risk_tolerance=0.7, confidence_bias=0.1,
                      creativity=0.9),
            gland=Gland(inertia=0.4),  # 低惰性=容易兴奋
            skills=["stock_scan", "multi_factor_score", "pattern_recognize", "sector_rotate"],
            instincts=[
                "评分≥65且多头排列 → 自动提交",
                "评分<65 → 丢弃",
                "每日最多提交5只",
            ],
            mcp=["QMT行情K线", "alpha_engine(L4)", "Qwen(AI确认top3)"],
            workflows=["09:35首扫→10:00/10:30/13:30补扫→提交→等审批"],
            memory_keys=["pick_accuracy_by_strategy", "pick_accuracy_by_sector", "missed_winners"],
        ),
        AgentRole.RISK: AgentConfig(
            role="risk", name="风控官", avatar="🛡️",
            gene=Gene(gene_type="paranoid", risk_tolerance=0.2, confidence_bias=-0.2,
                      veto_rate_cap=0.70, min_approval_rate=0.30),
            gland=Gland(inertia=0.5),
            skills=["position_monitor", "stop_loss_execute", "drawdown_check", "veto_trade"],
            instincts=[
                "单股亏≥5% → 强制止损",
                "日亏≥0.3% → 通知组长保本模式",
                "否决率不超过70%",
            ],
            mcp=["QMT持仓/委托", "SQLite风控日志"],
            workflows=["逐笔审核猎手候选→每60秒扫描持仓→止损/止盈执行"],
            memory_keys=["stop_loss_effectiveness", "veto_missed_profit", "max_drawdown"],
        ),
        AgentRole.COACH: AgentConfig(
            role="coach", name="复盘教练", avatar="📊",
            gene=Gene(gene_type="analytical", risk_tolerance=0.5, objectivity=1.0),
            gland=Gland(frozen=True, state="frozen"),  # 教练腺体冻结
            skills=["trade_attribution", "pattern_analysis", "gene_mutation_suggest", "param_optimize"],
            instincts=[
                "收盘后15min内完成复盘",
                "同类错误≥3次 → 调整对应Agent基因",
            ],
            mcp=["SQLite交易记录", "trade_tracker", "Qwen(日终总结)"],
            workflows=["15:05提取数据→盈亏归因→Agent评分→基因微调→写入记忆"],
            memory_keys=["trade_patterns", "gene_mutation_history", "param_changelog"],
        ),
    }
