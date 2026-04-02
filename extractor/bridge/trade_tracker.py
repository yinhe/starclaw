"""
Trade Tracker — 记录每笔交易的完整生命周期，计算策略归因指标。

Persists to trades.json. Provides:
  - Win rate, profit factor, avg win/loss
  - Per-reason attribution (which exit reasons perform best)
  - Per-signal-strength attribution (which strength levels are profitable)
  - Holding period analysis
"""

import json
import logging
import os
import sqlite3
from datetime import datetime
from typing import Dict, List, Optional

logger = logging.getLogger("trade_tracker")

_DIR = os.path.dirname(os.path.abspath(__file__))
TRADES_FILE = os.path.join(_DIR, "trades.json")
TRADES_DB = os.path.join(_DIR, "trades.db")


class Trade:
    """A completed (closed) trade."""

    def __init__(self, code: str, direction: str,
                 entry_price: float, exit_price: float,
                 volume: int, entry_time: str, exit_time: str,
                 entry_reason: str = "", exit_reason: str = "",
                 signal_strength: int = 0, score: float = 0,
                 pnl_pct: float = 0, pnl_amount: float = 0,
                 holding_days: int = 0, name: str = "",
                 strategy_type: str = "", sector: str = ""):
        self.code = code
        self.direction = direction
        self.entry_price = entry_price
        self.exit_price = exit_price
        self.volume = volume
        self.entry_time = entry_time
        self.exit_time = exit_time
        self.entry_reason = entry_reason
        self.exit_reason = exit_reason
        self.signal_strength = signal_strength
        self.score = score
        self.pnl_pct = pnl_pct
        self.pnl_amount = pnl_amount
        self.holding_days = holding_days
        self.name = name
        self.strategy_type = strategy_type
        self.sector = sector

    def to_dict(self) -> dict:
        return self.__dict__

    @classmethod
    def from_dict(cls, d: dict) -> "Trade":
        return cls(**{k: v for k, v in d.items() if k in cls.__init__.__code__.co_varnames})


class TradeTracker:
    """Tracks and analyzes completed trades."""

    def __init__(self):
        self.trades: List[Trade] = []
        self._init_db()
        self._load()

    def _init_db(self):
        """Create SQLite trades table if not exists."""
        try:
            conn = sqlite3.connect(TRADES_DB)
            conn.execute("""CREATE TABLE IF NOT EXISTS trades (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                code TEXT NOT NULL,
                name TEXT DEFAULT '',
                direction TEXT DEFAULT 'long',
                entry_price REAL, exit_price REAL,
                volume INTEGER,
                entry_time TEXT, exit_time TEXT,
                entry_reason TEXT DEFAULT '',
                exit_reason TEXT DEFAULT '',
                signal_strength INTEGER DEFAULT 0,
                score REAL DEFAULT 0,
                pnl_pct REAL DEFAULT 0,
                pnl_amount REAL DEFAULT 0,
                holding_days INTEGER DEFAULT 0,
                created_at TEXT DEFAULT (datetime('now','localtime'))
            )""")
            conn.execute("CREATE INDEX IF NOT EXISTS idx_trades_code ON trades(code)")
            conn.execute("CREATE INDEX IF NOT EXISTS idx_trades_exit_time ON trades(exit_time)")
            # Phase 10: add strategy_type and sector columns if missing
            try:
                conn.execute("ALTER TABLE trades ADD COLUMN strategy_type TEXT DEFAULT ''")
            except Exception:
                pass  # column already exists
            try:
                conn.execute("ALTER TABLE trades ADD COLUMN sector TEXT DEFAULT ''")
            except Exception:
                pass
            conn.commit()
            conn.close()
        except Exception as e:
            logger.warning(f"Failed to init trades DB: {e}")

    def _load(self):
        """Load from SQLite first, fall back to JSON for migration."""
        try:
            conn = sqlite3.connect(TRADES_DB)
            conn.row_factory = sqlite3.Row
            rows = conn.execute("SELECT * FROM trades ORDER BY id").fetchall()
            conn.close()
            if rows:
                self.trades = [Trade(
                    code=r["code"], direction=r["direction"],
                    entry_price=r["entry_price"], exit_price=r["exit_price"],
                    volume=r["volume"], entry_time=r["entry_time"], exit_time=r["exit_time"],
                    entry_reason=r["entry_reason"] or "", exit_reason=r["exit_reason"] or "",
                    signal_strength=r["signal_strength"], score=r["score"],
                    pnl_pct=r["pnl_pct"], pnl_amount=r["pnl_amount"],
                    holding_days=r["holding_days"], name=r["name"] or "",
                    strategy_type=r["strategy_type"] if "strategy_type" in r.keys() else "",
                    sector=r["sector"] if "sector" in r.keys() else "",
                ) for r in rows]
                logger.info(f"Loaded {len(self.trades)} trades from SQLite")
                return
        except Exception as e:
            logger.warning(f"SQLite load failed: {e}")
        # Fallback: load from JSON and migrate to SQLite
        if os.path.exists(TRADES_FILE):
            try:
                with open(TRADES_FILE, "r", encoding="utf-8") as f:
                    data = json.load(f)
                self.trades = [Trade.from_dict(d) for d in data]
                logger.info(f"Loaded {len(self.trades)} trades from JSON, migrating to SQLite")
                for t in self.trades:
                    self._insert_db(t)
            except Exception as e:
                logger.warning(f"Failed to load trades: {e}")

    def _insert_db(self, t: Trade):
        """Insert a single trade into SQLite."""
        try:
            conn = sqlite3.connect(TRADES_DB)
            conn.execute(
                """INSERT INTO trades (code, name, direction, entry_price, exit_price,
                   volume, entry_time, exit_time, entry_reason, exit_reason,
                   signal_strength, score, pnl_pct, pnl_amount, holding_days,
                   strategy_type, sector)
                   VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)""",
                (t.code, t.name, t.direction, t.entry_price, t.exit_price,
                 t.volume, t.entry_time, t.exit_time, t.entry_reason, t.exit_reason,
                 t.signal_strength, t.score, t.pnl_pct, t.pnl_amount, t.holding_days,
                 getattr(t, 'strategy_type', ''), getattr(t, 'sector', ''))
            )
            conn.commit()
            conn.close()
        except Exception as e:
            logger.warning(f"Failed to insert trade to DB: {e}")

    def _save(self):
        """Save to both JSON (backward compat) and SQLite."""
        try:
            data = [t.to_dict() for t in self.trades]
            with open(TRADES_FILE, "w", encoding="utf-8") as f:
                json.dump(data, ensure_ascii=False, indent=2, fp=f)
        except Exception as e:
            logger.error(f"Failed to save trades JSON: {e}")

    def record_trade(self, code: str, entry_price: float, exit_price: float,
                     volume: int, entry_time: str, exit_time: str = "",
                     entry_reason: str = "", exit_reason: str = "",
                     signal_strength: int = 0, score: float = 0,
                     strategy_type: str = "", sector: str = ""):
        """Record a completed trade."""
        if not exit_time:
            exit_time = datetime.now().strftime("%Y-%m-%d %H:%M:%S")

        pnl_pct = (exit_price - entry_price) / entry_price * 100 if entry_price > 0 else 0
        pnl_amount = (exit_price - entry_price) * volume if entry_price > 0 else 0

        try:
            entry_dt = datetime.strptime(entry_time, "%Y-%m-%d %H:%M:%S")
            exit_dt = datetime.strptime(exit_time, "%Y-%m-%d %H:%M:%S")
            holding_days = (exit_dt - entry_dt).days
        except Exception:
            holding_days = 0

        trade = Trade(
            code=code, direction="long",
            entry_price=entry_price, exit_price=exit_price,
            volume=volume, entry_time=entry_time, exit_time=exit_time,
            entry_reason=entry_reason, exit_reason=exit_reason,
            signal_strength=signal_strength, score=score,
            pnl_pct=round(pnl_pct, 2), pnl_amount=round(pnl_amount, 2),
            holding_days=holding_days,
            strategy_type=strategy_type, sector=sector,
        )
        self.trades.append(trade)
        self._insert_db(trade)
        self._save()
        logger.info(f"[tracker] Recorded: {code} {pnl_pct:+.2f}% "
                     f"({entry_price:.2f}→{exit_price:.2f}) {exit_reason}")

    # ------------------------------------------------------------------
    # Analytics
    # ------------------------------------------------------------------

    def stats(self, last_n: int = 0) -> dict:
        """Compute overall strategy statistics.

        Args:
            last_n: if >0, only use last N trades
        """
        trades = self.trades[-last_n:] if last_n > 0 else self.trades
        if not trades:
            return {"total_trades": 0, "message": "no trades recorded"}

        wins = [t for t in trades if t.pnl_pct > 0]
        losses = [t for t in trades if t.pnl_pct <= 0]

        total = len(trades)
        win_count = len(wins)
        loss_count = len(losses)
        win_rate = win_count / total if total > 0 else 0

        avg_win = sum(t.pnl_pct for t in wins) / win_count if win_count > 0 else 0
        avg_loss = sum(t.pnl_pct for t in losses) / loss_count if loss_count > 0 else 0
        profit_factor = abs(avg_win * win_count / (avg_loss * loss_count)) if loss_count > 0 and avg_loss != 0 else 999

        total_pnl = sum(t.pnl_amount for t in trades)
        avg_holding = sum(t.holding_days for t in trades) / total if total > 0 else 0

        # Expectancy: avg_win * win_rate + avg_loss * (1 - win_rate)
        expectancy = avg_win * win_rate + avg_loss * (1 - win_rate)

        # Max consecutive losses
        max_consec_loss = 0
        cur_consec = 0
        for t in trades:
            if t.pnl_pct <= 0:
                cur_consec += 1
                max_consec_loss = max(max_consec_loss, cur_consec)
            else:
                cur_consec = 0

        return {
            "total_trades": total,
            "wins": win_count,
            "losses": loss_count,
            "win_rate": round(win_rate, 3),
            "avg_win_pct": round(avg_win, 2),
            "avg_loss_pct": round(avg_loss, 2),
            "profit_factor": round(profit_factor, 2),
            "expectancy_pct": round(expectancy, 2),
            "total_pnl": round(total_pnl, 2),
            "avg_holding_days": round(avg_holding, 1),
            "max_consecutive_losses": max_consec_loss,
        }

    def attribution_by_exit_reason(self) -> List[dict]:
        """Analyze performance grouped by exit reason."""
        groups: Dict[str, list] = {}
        for t in self.trades:
            # Normalize reason to category
            reason = t.exit_reason.split("(")[0] if t.exit_reason else "unknown"
            groups.setdefault(reason, []).append(t)

        result = []
        for reason, trades in sorted(groups.items()):
            wins = sum(1 for t in trades if t.pnl_pct > 0)
            avg_pnl = sum(t.pnl_pct for t in trades) / len(trades)
            result.append({
                "exit_reason": reason,
                "count": len(trades),
                "win_rate": round(wins / len(trades), 3),
                "avg_pnl_pct": round(avg_pnl, 2),
            })
        return sorted(result, key=lambda x: x["avg_pnl_pct"], reverse=True)

    def attribution_by_signal_strength(self) -> List[dict]:
        """Analyze performance grouped by signal strength (0-6)."""
        groups: Dict[int, list] = {}
        for t in self.trades:
            groups.setdefault(t.signal_strength, []).append(t)

        result = []
        for strength, trades in sorted(groups.items()):
            wins = sum(1 for t in trades if t.pnl_pct > 0)
            avg_pnl = sum(t.pnl_pct for t in trades) / len(trades)
            result.append({
                "signal_strength": strength,
                "count": len(trades),
                "win_rate": round(wins / len(trades), 3),
                "avg_pnl_pct": round(avg_pnl, 2),
            })
        return result

    def recent_trades(self, limit: int = 20) -> List[dict]:
        """Return most recent trades."""
        return [t.to_dict() for t in reversed(self.trades[-limit:])]

    def review(self) -> dict:
        """Comprehensive trade review: stats + attribution + time analysis + strategy suggestions."""
        trades = self.trades
        if not trades:
            return {"stats": {"total_trades": 0}, "attribution": [], "time_dist": [],
                    "holding_dist": [], "pnl_dist": [], "suggestions": ["暂无交易数据，无法生成建议"]}

        stats = self.stats()
        by_reason = self.attribution_by_exit_reason()

        # --- Time distribution (by weekday and hour) ---
        weekday_map = {0: "周一", 1: "周二", 2: "周三", 3: "周四", 4: "周五", 5: "周六", 6: "周日"}
        weekday_stats: Dict[str, list] = {}
        hour_stats: Dict[int, list] = {}
        for t in trades:
            try:
                dt = datetime.strptime(t.exit_time[:19], "%Y-%m-%d %H:%M:%S")
                wd = weekday_map.get(dt.weekday(), str(dt.weekday()))
                weekday_stats.setdefault(wd, []).append(t.pnl_pct)
                hour_stats.setdefault(dt.hour, []).append(t.pnl_pct)
            except Exception:
                pass

        time_dist = []
        for wd in ["周一", "周二", "周三", "周四", "周五"]:
            pnls = weekday_stats.get(wd, [])
            if pnls:
                wins = sum(1 for p in pnls if p > 0)
                time_dist.append({"period": wd, "count": len(pnls),
                                  "win_rate": round(wins / len(pnls), 3),
                                  "avg_pnl": round(sum(pnls) / len(pnls), 2)})

        hour_dist = []
        for h in sorted(hour_stats.keys()):
            pnls = hour_stats[h]
            wins = sum(1 for p in pnls if p > 0)
            hour_dist.append({"hour": h, "count": len(pnls),
                              "win_rate": round(wins / len(pnls), 3),
                              "avg_pnl": round(sum(pnls) / len(pnls), 2)})

        # --- Holding period distribution ---
        holding_buckets = {"当日": (0, 0), "1-2天": (1, 2), "3-5天": (3, 5),
                           "6-10天": (6, 10), "11-20天": (11, 20), "20天+": (21, 9999)}
        holding_dist = []
        for label, (lo, hi) in holding_buckets.items():
            bucket = [t for t in trades if lo <= t.holding_days <= hi]
            if bucket:
                wins = sum(1 for t in bucket if t.pnl_pct > 0)
                avg_pnl = sum(t.pnl_pct for t in bucket) / len(bucket)
                holding_dist.append({"period": label, "count": len(bucket),
                                     "win_rate": round(wins / len(bucket), 3),
                                     "avg_pnl": round(avg_pnl, 2)})

        # --- P&L distribution ---
        pnl_buckets = {"亏>10%": (-999, -10), "亏5-10%": (-10, -5), "亏0-5%": (-5, 0),
                       "赚0-5%": (0, 5), "赚5-10%": (5, 10), "赚>10%": (10, 999)}
        pnl_dist = []
        for label, (lo, hi) in pnl_buckets.items():
            count = sum(1 for t in trades if lo <= t.pnl_pct < hi)
            if count > 0:
                pnl_dist.append({"range": label, "count": count,
                                 "pct": round(count / len(trades) * 100, 1)})

        # --- Top winners & losers ---
        sorted_by_pnl = sorted(trades, key=lambda t: t.pnl_pct)
        top_losers = [t.to_dict() for t in sorted_by_pnl[:5]]
        top_winners = [t.to_dict() for t in reversed(sorted_by_pnl[-5:])]

        # --- Strategy type attribution (Phase 10) ---
        type_groups: Dict[str, list] = {}
        for t in trades:
            stype = getattr(t, 'strategy_type', '') or 'unknown'
            type_groups.setdefault(stype, []).append(t)
        strategy_type_attr = []
        for stype, group in sorted(type_groups.items()):
            wins = sum(1 for t in group if t.pnl_pct > 0)
            avg_pnl = sum(t.pnl_pct for t in group) / len(group)
            strategy_type_attr.append({
                "strategy_type": stype, "count": len(group),
                "win_rate": round(wins / len(group), 3),
                "avg_pnl_pct": round(avg_pnl, 2),
            })
        strategy_type_attr.sort(key=lambda x: x["avg_pnl_pct"], reverse=True)

        # --- Strategy improvement suggestions ---
        suggestions = self._generate_suggestions(stats, by_reason, holding_dist, time_dist, trades)

        return {
            "stats": stats,
            "attribution": by_reason,
            "strategy_type_attr": strategy_type_attr,
            "time_dist": time_dist,
            "hour_dist": hour_dist,
            "holding_dist": holding_dist,
            "pnl_dist": pnl_dist,
            "top_winners": top_winners,
            "top_losers": top_losers,
            "suggestions": suggestions,
        }

    def _generate_suggestions(self, stats: dict, attribution: list,
                              holding_dist: list, time_dist: list, trades: list) -> List[str]:
        """Auto-generate strategy improvement suggestions based on trade data."""
        suggestions = []
        total = stats.get("total_trades", 0)
        if total < 3:
            return ["交易样本不足（<3笔），建议积累更多数据后再分析"]

        win_rate = stats.get("win_rate", 0)
        avg_win = stats.get("avg_win_pct", 0)
        avg_loss = stats.get("avg_loss_pct", 0)
        pf = stats.get("profit_factor", 0)
        expectancy = stats.get("expectancy_pct", 0)
        max_consec = stats.get("max_consecutive_losses", 0)

        # 1. Win rate analysis
        if win_rate < 0.35:
            suggestions.append(f"⚠️ 胜率偏低({win_rate:.0%})，建议：① 提高入场信号质量（只做强势股） ② 加入AI确认环节 ③ 参考大盘方向过滤")
        elif win_rate < 0.45:
            suggestions.append(f"📊 胜率中等偏低({win_rate:.0%})，考虑增加趋势过滤条件（如只在MA5>MA8>MA34时入场）")

        # 2. Profit factor
        if 0 < pf < 1:
            suggestions.append(f"🔴 盈亏比不足({pf:.2f})，亏损总额大于盈利。建议：扩大止盈空间或缩小止损幅度")
        elif pf < 1.5:
            suggestions.append(f"⚠️ 盈亏比偏低({pf:.2f})，目标>1.5。建议使用阶梯止盈（先卖50%保利润，剩余追踪止盈）")

        # 3. Average win vs loss asymmetry
        if avg_win > 0 and avg_loss < 0:
            ratio = abs(avg_win / avg_loss) if avg_loss != 0 else 999
            if ratio < 1:
                suggestions.append(f"📉 平均盈利({avg_win:+.2f}%)小于平均亏损({avg_loss:.2f}%)，赚少亏多。建议：① 适当放宽止盈 ② 缩紧止损至-4%")
            elif ratio > 3:
                suggestions.append(f"✅ 盈亏比优秀({ratio:.1f}:1)，但如果胜率低可考虑适当缩小止盈以提高胜率")

        # 4. Stop loss analysis
        stop_loss_trades = [a for a in attribution if 'stop_loss' in a.get('exit_reason', '').lower()]
        if stop_loss_trades:
            sl = stop_loss_trades[0]
            if sl['count'] / total > 0.4:
                suggestions.append(f"🛑 止损占比过高({sl['count']}/{total}={sl['count']/total:.0%})，说明入场点不够精确。建议：① 等回调到MA8再入场 ② 等缩量企稳再买")

        # 5. Holding period
        short_trades = [t for t in trades if t.holding_days <= 1]
        if len(short_trades) / total > 0.3:
            short_win = sum(1 for t in short_trades if t.pnl_pct > 0)
            short_wr = short_win / len(short_trades) if short_trades else 0
            suggestions.append(f"⏱️ 当日/次日卖出占比{len(short_trades)/total:.0%}(胜率{short_wr:.0%})，频繁短线交易。建议：持仓至少2-3天让利润奔跑")

        long_trades = [t for t in trades if t.holding_days > 10]
        if long_trades:
            long_avg = sum(t.pnl_pct for t in long_trades) / len(long_trades)
            if long_avg < 0:
                suggestions.append(f"📅 持仓超10天的{len(long_trades)}笔交易平均亏损{long_avg:.2f}%，长期持有效果差。建议：加入5日时间止损")

        # 6. Consecutive losses
        if max_consec >= 4:
            suggestions.append(f"💔 最大连续亏损{max_consec}笔，建议：连亏3笔后暂停交易一天，冷静后重新评估策略")

        # 7. Time-based suggestions
        for td in time_dist:
            if td['count'] >= 3 and td['avg_pnl'] < -2:
                suggestions.append(f"📆 {td['period']}卖出表现差(均亏{td['avg_pnl']:.1f}%/笔)，考虑避开该日操作")

        # 8. Exit reason specific
        for a in attribution:
            reason = a.get('exit_reason', '')
            if 'time_stop' in reason and a['count'] >= 2 and a['avg_pnl_pct'] < -1:
                suggestions.append(f"⏰ 时间止损({a['count']}笔，均亏{a['avg_pnl_pct']:.1f}%)效果差。可能入场时机偏晚，建议选股时优先近期放量突破的票")

        # 9. Expectancy
        if expectancy < 0:
            suggestions.append(f"🔴 策略期望值为负({expectancy:.2f}%/笔)，当前策略整体亏损。需要同时提升胜率和盈亏比")
        elif expectancy > 0 and expectancy < 0.5:
            suggestions.append(f"⚠️ 策略期望值偏低({expectancy:.2f}%/笔)，佣金和滑点可能吃掉利润。建议减少交易频率，只做高置信度信号")

        if not suggestions:
            suggestions.append("✅ 策略指标整体良好，继续保持当前纪律执行")

        return suggestions
