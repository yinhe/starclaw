# China A-Share Quantitative Trading — Investor Guide

> How to participate in AI-powered algorithmic trading on China's stock market

---

## What We Do

We built an **AI-powered quantitative trading system (codename: Extractor)** that automatically scans 4,000+ stocks listed on the Shanghai and Shenzhen Stock Exchanges, identifies "main wave" breakout signals, and executes trades after AI confirmation.

**Our Unique Edge: Quantitative Scoring × AI Double-Check**

| Traditional Quant | Our System |
|-------------------|-----------|
| Technical indicators → Execute directly | 4-dimensional scoring → **AI analyzes fundamentals, news, sector activity** → Execute after confirmation |
| Vulnerable to earnings bombs & sudden bad news | AI pre-filters risks: earnings warnings, insider selling, policy changes |
| Fixed strategy can't adapt | Auto-detects bull/sideways/bear market, dynamically adjusts parameters |

---

## How Our System Works

```
                    ┌──────────────────┐
                    │  QMT Trading      │
                    │  Terminal          │
                    └────────┬─────────┘
                             │ Real-time data + Auto execution
                             ▼
┌───────────────────────────────────────────────────┐
│        Extractor System (Our Technology)            │
│                                                     │
│  ① Scan 4,000+ A-share stocks (daily bars)         │
│  ② Score each stock on 4 dimensions [0-1]:         │
│     · Trend + Position (50%) — Moving average       │
│       alignment, price percentile                   │
│     · Momentum (20%) — 3-day vs 10-day elasticity   │
│     · Today's Price-Volume (30%) — Breakout level   │
│       + volume surge                                │
│     · Volatility Constraint — ATR penalty           │
│  ③ AI Double-Check (Claw Agent):                    │
│     · Fundamental scan — earnings, ST/delisting     │
│     · News scan — negative news, insider selling    │
│     · Technical verification — trend confirmation   │
│     · Sector resonance — industry activity          │
│  ④ Auto-execute confirmed trades                    │
│  ⑤ 3-level circuit breaker risk management          │
│  ⑥ Daily settlement + performance report            │
└───────────────────────────────────────────────────┘
```

---

## How to Open an Account

### For Chinese Residents (Easiest)
1. **Download a broker app** — We recommend GuoJin Securities (国金证券)
2. **Online account opening** — Takes 10 minutes, need ID card + bank card
3. **Apply for QMT access** — Ask your account manager to enable quantitative trading
4. **Done** — No minimum balance required for testing

### For Foreign Investors (Including US Citizens)

Foreign individuals can invest in China A-shares through two routes:

#### Route A: Direct A-Share Account (QFII-like)
- Requires: Passport + China visa + Chinese bank account
- Must open in person at a brokerage office in China
- Restrictions apply — consult the broker in advance
- **Not recommended** for most foreign investors due to complexity

#### Route B: Stock Connect (Recommended for Foreign Investors) 🌟
Invest in A-shares through **Shanghai-Hong Kong Stock Connect** and **Shenzhen-Hong Kong Stock Connect**, without needing a Chinese account.

| Broker | Stock Connect Access | Account Opening | Language |
|--------|---------------------|-----------------|----------|
| **Interactive Brokers (IBKR)** | ✅ Shanghai + Shenzhen Connect | Online, 1-2 days | English |
| **Futu (moomoo)** | ✅ | Online | English + Chinese |
| **Tiger Brokers** | ✅ | Online | English + Chinese |
| **HSBC Hong Kong** | ✅ | In-person (HK) | English |

**Our recommendation: Open an IBKR account** → Enable "Hong Kong Stock Connect" → You can trade ~2,000 A-share stocks directly.

Stock Connect covers:
- All Shanghai Stock Exchange 180 & 380 index constituents
- All Shenzhen Stock Exchange Component & Small/Mid Cap index constituents
- Total: ~2,000 stocks (covers most liquid large & mid caps)

---

## How Much Money Do You Need

| Stage | Amount | What You Can Do |
|-------|--------|-----------------|
| **Paper Trading (Simulated)** | **¥0 / $0** | Test strategies with virtual money. Zero risk. |
| **Minimum Live Trading** | **¥5,000 – ¥20,000** (~$700 – $2,800) | Run basic strategy on 3-5 stocks |
| **Recommended Start** | **¥50,000 – ¥200,000** (~$7,000 – $28,000) | Diversified portfolio, 8-15 stocks, full risk management |
| **Standard Allocation** | **¥500,000+** (~$70,000+) | Multiple concurrent strategies |
| **Professional Scale** | **¥1,000,000+** (~$140,000+) | Full strategy suite + hedging + IPO subscription |

### For Foreign Investors via Stock Connect
- IBKR minimum: **$0** (no minimum)
- Recommended: **$5,000 – $20,000** for meaningful testing
- Stock Connect settlement in CNH (offshore RMB) or HKD

### Our Suggestion

> **Start with paper trading ($0) for 2-4 weeks**, then allocate **$10,000 – $30,000** for initial live testing via Stock Connect.

---

## Important A-Share Market Rules

| Rule | Details |
|------|---------|
| **T+1 Trading** | Stocks bought today can only be sold tomorrow (no same-day round trips) |
| **Price Limits** | Main board: ±10% daily limit; ChiNext/STAR: ±20% |
| **Minimum Lot** | 100 shares (1 lot) — most stocks are ¥5-50/share, so 1 lot = ¥500-5,000 |
| **Trading Hours** | 9:30-11:30 AM, 1:00-3:00 PM (Beijing Time, UTC+8) |
| **No Short Selling** | Retail investors generally cannot short A-shares |
| **Stamp Tax** | 0.05% on sell side only (government tax) |

---

## Risk Management — 3-Level Circuit Breaker

### Level 1: Per-Strategy
- Per-trade stop loss: **-2%** auto-close
- Daily strategy loss: **-3%** pause strategy
- Single stock max: **25%** of portfolio

### Level 2: Per-Account
- Account daily drawdown exceeds threshold → pause all strategies
- Available cash below 20% → block new positions
- 3 consecutive losing days → auto-reduce 50% + alert

### Level 3: System-Wide
- Total drawdown > **-5%** → **Full circuit break** (cancel all orders, block all trading)
- System connection failure → emergency cancel all pending orders
- Extreme market conditions → emergency liquidation protection

---

## Profit Distribution

```
After market close each trading day (3:30 PM Beijing):

Net Profit = Realized P&L - Commission - Stamp Tax

├── 60% → Reinvested (compound growth)
├── 20% → Investor returns (withdrawable per agreement)
├── 10% → Technology team management fee
└── 10% → Risk reserve fund (covers losing days)

Losing days: No distribution, covered by reserve fund.
```

---

## Fee Structure

| Item | Cost |
|------|------|
| Broker commission | ~0.015% each way (buy + sell) |
| Stamp tax | 0.05% (sell only, government tax) |
| Transfer fee | 0.001% |
| Our management fee | Profit sharing (to be discussed) |
| System fee | None |
| Account minimum | None (recommend ¥50,000+ / $7,000+) |

---

## What You See as an Investor

### Real-Time Dashboard
- 📊 All positions, P&L, and trade history at a glance
- 📋 Daily report: what was bought/sold and AI's reasoning
- 🚨 Risk alerts: immediate notification on circuit breaker triggers
- 📈 Monthly performance: return curve, max drawdown, Sharpe ratio, vs CSI 300 benchmark

### What You DON'T Need to Do
- ❌ No need to watch the market
- ❌ No need to make trading decisions
- ❌ No need to understand technical analysis
- ❌ No need to install any software

### Transparency & Security
- ✅ Your money stays in **your own brokerage account** — we cannot withdraw funds
- ✅ We only have trading permission (buy/sell), never fund transfer permission
- ✅ All trades are recorded by the broker, fully auditable
- ✅ You can revoke API access at any time to stop automated trading

---

## Next Steps

1. **Open a brokerage account** — IBKR (for Stock Connect) or GuoJin Securities (for direct A-share)
2. **Enable quantitative trading access** — Request QMT/API permission
3. **Share account credentials** — We only need API keys, never your password
4. **We connect and start paper trading** — 2-4 weeks of simulated testing
5. **Review results together** — Verify strategy performance
6. **Go live** — Fund the account and switch to real trading

---

## Risk Disclosure

⚠️ **All trading involves risk. Past performance does not guarantee future results.**

- Quantitative systems can generate losses due to extreme market volatility, technical failures, or model limitations
- AI analysis is based on historical data and public information — it cannot perfectly predict the future
- China A-share market is regulated by the CSRC (China Securities Regulatory Commission)
- Foreign investors face additional risks including currency fluctuation (RMB/USD) and regulatory changes
- **Only invest money you can afford to lose**

---

*Prepared by the StarClaw Quantitative Trading Team*
