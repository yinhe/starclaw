# Getting Started with Algorithmic Trading in US Stock Markets

> A plain-language guide for investors partnering with a quantitative trading team

---

## What We Do

We build **AI-powered quantitative trading systems** that automatically analyze thousands of stocks, identify high-probability trading opportunities, and execute trades — all without human emotion getting in the way.

Our technology combines:
- **Mathematical models** that score every stock on trend strength, momentum, volume, and volatility
- **AI confirmation** (our proprietary Claw AI) that cross-checks each candidate against news, earnings, and sector activity before placing any trade
- **Automated risk management** with multi-level circuit breakers to protect capital

---

## Opening a Brokerage Account

To run algorithmic trading on US stocks, you need a brokerage account that provides **API access** (a way for our software to place trades automatically).

### Recommended Brokers

| Broker | Best For | Minimum Deposit | Commission | API Access |
|--------|----------|----------------|------------|------------|
| **Interactive Brokers (IBKR)** | Serious quantitative trading | $0 | ~$0.005/share (min $1) | ✅ Professional-grade |
| **Alpaca** | Getting started / testing | $0 | **$0 (free)** | ✅ Easy REST API |
| **Charles Schwab** | Long-term + some algo | $0 | $0 | ✅ Via thinkorswim |

### Our Recommendation

**Start with Interactive Brokers (IBKR)** — it's what most quantitative hedge funds use worldwide. Here's why:

- ✅ Supports stocks, options, futures, forex, crypto — all in one account
- ✅ Professional API (TWS API / IB Gateway) with Python support
- ✅ **Free paper trading** (simulated trading with fake money) to test strategies
- ✅ Low commissions ($1 minimum per trade)
- ✅ Available to US residents and international investors
- ✅ Trusted — publicly traded company (Nasdaq: IBKR), founded 1978

---

## What You Need to Open an Account

### For US Residents
1. **Government-issued ID** (driver's license or passport)
2. **Social Security Number (SSN)**
3. **Bank account** for funding
4. **5 minutes** to complete the online application

### For Non-US Residents
1. **Passport**
2. **Proof of address** (utility bill or bank statement)
3. **W-8BEN tax form** (filled online — declares you're a non-US person for tax purposes)
4. **Wire transfer** capability from your bank

### Account Type
- **Individual account** — simplest, for personal trading
- **LLC / Corporate account** — if trading through a company (recommended for larger amounts)

---

## How Much Money Do You Need?

| Stage | Amount | What You Can Do |
|-------|--------|-----------------|
| **Paper Trading (Simulated)** | **$0** | Test our strategies with fake money. No risk. Full functionality. |
| **Minimum Live Trading** | **$500 – $2,000** | Run basic strategies on 3–5 stocks. Good for proof of concept. |
| **Meaningful Live Trading** | **$5,000 – $25,000** | Diversified portfolio of 8–15 stocks. Proper risk management. |
| **Day Trading** | **$25,000+** | Required by SEC rules (Pattern Day Trader rule) if you buy and sell the same stock on the same day more than 3 times per week. |
| **Professional Scale** | **$100,000+** | Full strategy suite, multiple concurrent strategies, options hedging. |

### Our Suggestion for First-Time Testing

> **Start with $0 (paper trading) for 2–4 weeks**, then fund with **$5,000–$10,000** for initial live testing.

Paper trading lets you verify that:
- Our algorithms generate profitable signals
- The AI confirmation layer filters out bad trades
- Risk management works as designed
- You're comfortable with the trading frequency and style

**You risk nothing during paper trading.** When you're satisfied with the results, you fund the account and switch to live trading with one click.

---

## What Permissions / Access Do You Need?

| Permission | Why | How to Get It |
|------------|-----|---------------|
| **Market data subscription** | Real-time stock prices | Included free with IBKR (basic) or ~$4.50/month (professional) |
| **API access** | Let our software trade automatically | Enabled by default on IBKR; just need to download TWS or IB Gateway |
| **Margin account** (optional) | Borrow money to trade more | Apply during account opening; requires $2,000+ minimum |
| **Options trading** (optional) | Hedging strategies | Apply during account opening; requires passing a short questionnaire |

For our initial strategy (stock-only, long-only), you just need:
- ✅ A funded account
- ✅ Market data (free tier is fine)
- ✅ API access enabled

**No margin, no options, no special permissions needed to start.**

---

## How It Works (Investor's View)

```
You fund the account
        ↓
Our system runs 24/5 (market hours)
        ↓
┌─────────────────────────────────────┐
│  1. Scan 4,000+ stocks              │
│  2. Score each on 4 dimensions      │
│  3. AI reviews top candidates       │
│  4. Execute confirmed trades        │
│  5. Monitor & manage risk 24/7      │
│  6. Auto stop-loss if things go bad │
└─────────────────────────────────────┘
        ↓
You see results in a real-time dashboard
        ↓
Profits are tracked transparently
```

### What You See
- **Real-time dashboard** showing all positions, P&L, and trade history
- **Daily summary** with what was bought/sold and why
- **Risk alerts** if any circuit breaker triggers
- **Monthly performance report** with benchmark comparison

### What You Don't Need to Do
- ❌ No need to watch the market
- ❌ No need to make trading decisions
- ❌ No need to understand technical analysis
- ❌ No need to install any software (dashboard is web-based)

---

## Fee Structure

| Item | Cost |
|------|------|
| Brokerage commission | ~$1 per trade (paid to IBKR) |
| Market data | $0 – $4.50/month |
| Our management fee | To be discussed (typically % of profits) |
| Account minimum | No minimum (we recommend $5,000+) |

---

## Risk Disclosure

All trading involves risk. Past performance does not guarantee future results. Our system includes multiple safety mechanisms:

- **Per-trade stop loss**: Maximum 2% loss per position
- **Daily circuit breaker**: If total portfolio drops 5% in one day, all trading stops
- **AI confirmation**: Every trade is reviewed by AI before execution
- **Diversification**: Maximum 25% in any single stock

However, losses can and do occur. Only invest money you can afford to lose.

---

## Next Steps

1. **Open an IBKR account** at [interactivebrokers.com](https://www.interactivebrokers.com)
2. **Enable paper trading** (Settings → Paper Trading Account)
3. **Share your account ID** with us (we never need your password — API uses separate credentials)
4. **We connect and start testing** with paper money
5. **Review results together** after 2–4 weeks
6. **Decision**: Fund for live trading or adjust strategy

---

*Prepared by the StarClaw Quantitative Trading Team*
*Contact: [your contact info here]*
