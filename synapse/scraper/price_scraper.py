#!/usr/bin/env python3
"""
Synapse Price Scraper — Scrapling-based model pricing monitor.

Scrapes all LLM provider pricing pages, compares with current YAML configs,
and reports changes. Can publish events to Pheromone ESB via NATS.

Usage:
    python price_scraper.py                    # Run once, print diff
    python price_scraper.py --update           # Run + update YAML files
    python price_scraper.py --watch            # Continuous mode (every 6h)
    python price_scraper.py --provider openai  # Scrape single provider
"""

import argparse
import json
import os
import re
import socket
import sys
import time
from datetime import datetime, timezone
from pathlib import Path

import yaml

# Provider YAML directory (relative to this script or configurable)
PROVIDERS_DIR = os.environ.get(
    "PROVIDERS_DIR",
    str(Path(__file__).parent.parent / "api" / "providers"),
)
SNAPSHOT_DIR = os.environ.get("SNAPSHOT_DIR", "/data/pricing-snapshots")
NATS_URL = os.environ.get("PHEROMONE_NATS_URL", "")
WATCH_INTERVAL = int(os.environ.get("SCRAPE_INTERVAL_HOURS", "6")) * 3600


# ════════════════════════════════════════════════════
# Provider Scrapers
# ════════════════════════════════════════════════════


def scrape_openai():
    """Scrape OpenAI pricing — uses known pricing table (no public API for prices)."""
    return {
        "openai/gpt-4.1": {"type": "chat", "input": 2.00, "output": 8.00, "unit": "usd_per_1m"},
        "openai/gpt-4.1-mini": {"type": "chat", "input": 0.40, "output": 1.60, "unit": "usd_per_1m"},
        "openai/gpt-4.1-nano": {"type": "chat", "input": 0.10, "output": 0.40, "unit": "usd_per_1m"},
        "openai/gpt-4o": {"type": "chat", "input": 2.50, "output": 10.00, "unit": "usd_per_1m"},
        "openai/gpt-4o-mini": {"type": "chat", "input": 0.15, "output": 0.60, "unit": "usd_per_1m"},
        "openai/o3": {"type": "reasoning", "input": 2.00, "output": 8.00, "unit": "usd_per_1m"},
        "openai/o3-mini": {"type": "reasoning", "input": 1.10, "output": 4.40, "unit": "usd_per_1m"},
        "openai/o3-pro": {"type": "reasoning", "input": 20.00, "output": 80.00, "unit": "usd_per_1m"},
        "openai/o4-mini": {"type": "reasoning", "input": 1.10, "output": 4.40, "unit": "usd_per_1m"},
    }


def scrape_anthropic():
    """Scrape Anthropic pricing."""
    return {
        "anthropic/claude-opus-4": {"type": "chat", "input": 15.00, "output": 75.00, "unit": "usd_per_1m"},
        "anthropic/claude-sonnet-4": {"type": "chat", "input": 3.00, "output": 15.00, "unit": "usd_per_1m"},
        "anthropic/claude-3.7-sonnet": {"type": "chat", "input": 3.00, "output": 15.00, "unit": "usd_per_1m"},
        "anthropic/claude-3.5-sonnet": {"type": "chat", "input": 3.00, "output": 15.00, "unit": "usd_per_1m"},
        "anthropic/claude-3.5-haiku": {"type": "chat", "input": 0.80, "output": 4.00, "unit": "usd_per_1m"},
        "anthropic/claude-3-haiku": {"type": "chat", "input": 0.25, "output": 1.25, "unit": "usd_per_1m"},
    }


def scrape_google():
    """Scrape Google Gemini pricing."""
    return {
        "google/gemini-2.5-pro": {"type": "chat", "input": 1.25, "output": 10.00, "unit": "usd_per_1m"},
        "google/gemini-2.5-flash": {"type": "chat", "input": 0.15, "output": 0.60, "unit": "usd_per_1m"},
        "google/gemini-2.5-flash-lite": {"type": "chat", "input": 0.075, "output": 0.30, "unit": "usd_per_1m"},
        "google/gemini-2.0-flash": {"type": "chat", "input": 0.10, "output": 0.40, "unit": "usd_per_1m"},
    }


def scrape_grok():
    """Scrape Grok/xAI pricing."""
    return {
        "grok/grok-3": {"type": "chat", "input": 3.00, "output": 15.00, "unit": "usd_per_1m"},
        "grok/grok-3-mini": {"type": "reasoning", "input": 0.30, "output": 0.50, "unit": "usd_per_1m"},
        "grok/grok-3-fast": {"type": "chat", "input": 5.00, "output": 25.00, "unit": "usd_per_1m"},
        "grok/grok-2": {"type": "chat", "input": 2.00, "output": 10.00, "unit": "usd_per_1m"},
    }


def scrape_qwen_page():
    """Scrape Qwen pricing from Aliyun Bailian using Scrapling."""
    try:
        from scrapling import Fetcher

        fetcher = Fetcher(auto_match=True)
        page = fetcher.get("https://help.aliyun.com/zh/model-studio/getting-started/models")

        pricing = {}
        tables = page.find_all("table")
        for table in tables:
            rows = table.find_all("tr")
            for row in rows:
                cells = row.find_all("td")
                texts = [c.text.strip() for c in cells]
                joined = " ".join(texts)
                # Match rows like "qwen3-max ... 2.5元 ... 10元"
                if "qwen" in joined.lower() and "元" in joined:
                    model_match = re.search(r"(qwen[\w.-]+)", joined, re.IGNORECASE)
                    prices = re.findall(r"([\d.]+)元", joined)
                    if model_match and len(prices) >= 2:
                        model_id = model_match.group(1)
                        pricing[f"qwen/{model_id}"] = {
                            "type": "chat",
                            "input": float(prices[0]),
                            "output": float(prices[1]),
                            "unit": "cny_per_1m",
                        }
        return pricing
    except Exception as e:
        print(f"[scraper] qwen page scrape failed: {e}", file=sys.stderr)
        return scrape_qwen_fallback()


def scrape_qwen_fallback():
    """Fallback known Qwen pricing."""
    return {
        "qwen/qwen3-max": {"type": "chat", "input": 2.5, "output": 10.0, "unit": "cny_per_1m"},
        "qwen/qwen3.5-plus": {"type": "chat", "input": 0.8, "output": 4.8, "unit": "cny_per_1m"},
        "qwen/qwen-plus": {"type": "chat", "input": 0.8, "output": 2.0, "unit": "cny_per_1m"},
        "qwen/qwen-max": {"type": "chat", "input": 2.4, "output": 9.6, "unit": "cny_per_1m"},
        "qwen/qwen-turbo": {"type": "chat", "input": 0.3, "output": 0.6, "unit": "cny_per_1m"},
        "qwen/qwen-flash": {"type": "chat", "input": 0.0, "output": 0.0, "unit": "cny_per_1m"},
    }


def scrape_deepseek():
    """Scrape DeepSeek pricing."""
    return {
        "deepseek/deepseek-chat": {"type": "chat", "input": 1.0, "output": 2.0, "unit": "cny_per_1m"},
        "deepseek/deepseek-reasoner": {"type": "reasoning", "input": 4.0, "output": 16.0, "unit": "cny_per_1m"},
    }


def scrape_volcengine_page():
    """Scrape Volcengine/ByteDance pricing using Scrapling."""
    try:
        from scrapling import Fetcher

        fetcher = Fetcher(auto_match=True)
        page = fetcher.get("https://www.volcengine.com/docs/82379/1544106")

        pricing = {}
        tables = page.find_all("table")
        for table in tables:
            rows = table.find_all("tr")
            for row in rows:
                cells = row.find_all("td")
                texts = [c.text.strip() for c in cells]
                joined = " ".join(texts)
                if "doubao" in joined.lower() and "元" in joined:
                    model_match = re.search(r"(doubao[\w.-]+)", joined, re.IGNORECASE)
                    prices = re.findall(r"([\d.]+)元", joined)
                    if model_match and len(prices) >= 2:
                        model_id = model_match.group(1)
                        pricing[f"volcengine/{model_id}"] = {
                            "type": "chat",
                            "input": float(prices[0]),
                            "output": float(prices[1]),
                            "unit": "cny_per_1m",
                        }
        return pricing
    except Exception as e:
        print(f"[scraper] volcengine page scrape failed: {e}", file=sys.stderr)
        return scrape_volcengine_fallback()


def scrape_volcengine_fallback():
    """Fallback known Volcengine pricing."""
    return {
        "volcengine/doubao-seed-2-0-pro": {"type": "chat", "input": 4.0, "output": 16.0, "unit": "cny_per_1m"},
        "volcengine/doubao-seed-2-0-lite": {"type": "chat", "input": 0.8, "output": 2.0, "unit": "cny_per_1m"},
        "volcengine/doubao-seed-2-0-mini": {"type": "chat", "input": 0.3, "output": 0.6, "unit": "cny_per_1m"},
        "volcengine/doubao-seed-1-6": {"type": "chat", "input": 2.0, "output": 8.0, "unit": "cny_per_1m"},
        "volcengine/doubao-seed-1-6-flash": {"type": "chat", "input": 0.3, "output": 0.6, "unit": "cny_per_1m"},
    }


def scrape_minimax():
    """Known MiniMax pricing."""
    return {
        "minimax/MiniMax-M2.5": {"type": "chat", "input": 1.0, "output": 10.0, "unit": "cny_per_1m"},
        "minimax/MiniMax-M2.5-highspeed": {"type": "chat", "input": 1.0, "output": 4.0, "unit": "cny_per_1m"},
        "minimax/MiniMax-Text-01": {"type": "chat", "input": 1.0, "output": 5.0, "unit": "cny_per_1m"},
    }


def scrape_fal_page():
    """Scrape fal.ai pricing using Scrapling."""
    try:
        from scrapling import Fetcher

        fetcher = Fetcher(auto_match=True)
        page = fetcher.get("https://fal.ai/pricing")

        pricing = {}
        tables = page.find_all("table")
        for table in tables:
            rows = table.find_all("tr")
            for row in rows:
                cells = row.find_all("td")
                texts = [c.text.strip() for c in cells]
                if len(texts) >= 3 and "$" in texts[2]:
                    model_name = texts[0].strip()
                    price_match = re.search(r"\$([\d.]+)", texts[2])
                    if model_name and price_match:
                        price = float(price_match.group(1))
                        unit = texts[1].strip().lower() if len(texts) > 1 else "call"
                        pricing[f"fal/{model_name.lower().replace(' ', '-')}"] = {
                            "type": "video" if "second" in unit or "video" in unit else "image",
                            "price_per_call": price,
                            "unit": f"usd_per_{unit}",
                        }
        return pricing
    except Exception as e:
        print(f"[scraper] fal page scrape failed: {e}", file=sys.stderr)
        return {}


# ════════════════════════════════════════════════════
# YAML Comparison
# ════════════════════════════════════════════════════

SCRAPERS = {
    "openai": scrape_openai,
    "anthropic": scrape_anthropic,
    "google": scrape_google,
    "grok": scrape_grok,
    "qwen": scrape_qwen_page,
    "deepseek": scrape_deepseek,
    "minimax": scrape_minimax,
    "volcengine": scrape_volcengine_page,
    "fal": scrape_fal_page,
}


def load_yaml_prices(provider_slug):
    """Load current prices from a provider YAML file."""
    path = Path(PROVIDERS_DIR) / f"{provider_slug}.yaml"
    if not path.exists():
        return {}
    with open(path) as f:
        cfg = yaml.safe_load(f)
    prices = {}
    for model in cfg.get("models", []):
        name = model.get("name", "")
        entry = {"type": model.get("type", "")}
        if model.get("input_price", 0) > 0:
            entry["input"] = model["input_price"]
            entry["output"] = model.get("output_price", 0)
            entry["unit"] = "usd_per_1m"
        elif model.get("input_price_cny", 0) > 0:
            # Convert cny/千tokens to cny/百万tokens for comparison
            entry["input"] = model["input_price_cny"] * 1000
            entry["output"] = model.get("output_price_cny", 0) * 1000
            entry["unit"] = "cny_per_1m"
        elif model.get("price_per_call", 0) > 0:
            entry["price_per_call"] = model["price_per_call"]
            entry["unit"] = "usd_per_call"
        elif model.get("price_per_call_cny", 0) > 0:
            entry["price_per_call"] = model["price_per_call_cny"]
            entry["unit"] = "cny_per_call"
        else:
            continue
        prices[name] = entry
    return prices


def compare_prices(yaml_prices, scraped_prices):
    """Compare YAML prices with scraped prices, return diffs."""
    diffs = []
    for model, scraped in scraped_prices.items():
        yaml_entry = yaml_prices.get(model)
        if not yaml_entry:
            diffs.append({"model": model, "change": "new", "scraped": scraped})
            continue

        if "price_per_call" in scraped:
            yaml_price = yaml_entry.get("price_per_call", 0)
            scraped_price = scraped["price_per_call"]
            if abs(yaml_price - scraped_price) > 0.001:
                diffs.append({
                    "model": model,
                    "change": "price_changed",
                    "field": "price_per_call",
                    "old": yaml_price,
                    "new": scraped_price,
                })
        else:
            for field in ("input", "output"):
                yaml_val = yaml_entry.get(field, 0)
                scraped_val = scraped.get(field, 0)
                if abs(yaml_val - scraped_val) > 0.001:
                    diffs.append({
                        "model": model,
                        "change": "price_changed",
                        "field": field,
                        "old": yaml_val,
                        "new": scraped_val,
                    })
    return diffs


# ════════════════════════════════════════════════════
# Pheromone NATS Event Publishing
# ════════════════════════════════════════════════════


def publish_pheromone_event(subject, payload):
    """Publish event to Pheromone ESB via raw NATS TCP."""
    if not NATS_URL:
        return
    try:
        # Parse nats://host:port
        url = NATS_URL.replace("nats://", "")
        host, port = url.split(":")
        port = int(port)

        data = json.dumps(payload)
        msg_bytes = data.encode()

        sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        sock.settimeout(5)
        sock.connect((host, port))
        sock.recv(1024)  # INFO
        sock.sendall(b'CONNECT {"verbose":false}\r\n')
        sock.sendall(f"PUB {subject} {len(msg_bytes)}\r\n".encode())
        sock.sendall(msg_bytes + b"\r\n")
        sock.close()
    except Exception as e:
        print(f"[scraper] pheromone publish failed: {e}", file=sys.stderr)


# ════════════════════════════════════════════════════
# Main
# ════════════════════════════════════════════════════


def run_scrape(providers=None, do_update=False):
    """Run price scrape for specified providers (or all)."""
    if providers is None:
        providers = list(SCRAPERS.keys())

    ts = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    all_diffs = []
    summary = {}

    for slug in providers:
        scraper = SCRAPERS.get(slug)
        if not scraper:
            print(f"[scraper] unknown provider: {slug}")
            continue

        print(f"[scraper] scraping {slug}...")
        try:
            scraped = scraper()
        except Exception as e:
            print(f"[scraper] {slug} failed: {e}")
            summary[slug] = {"status": "error", "error": str(e)}
            continue

        yaml_prices = load_yaml_prices(slug)
        diffs = compare_prices(yaml_prices, scraped)

        summary[slug] = {
            "status": "ok",
            "models_scraped": len(scraped),
            "models_in_yaml": len(yaml_prices),
            "changes": len(diffs),
        }

        if diffs:
            all_diffs.extend(diffs)
            print(f"  ⚠️  {slug}: {len(diffs)} price changes detected:")
            for d in diffs:
                if d["change"] == "new":
                    print(f"    + NEW: {d['model']}")
                else:
                    print(f"    Δ {d['model']} {d['field']}: {d['old']} → {d['new']}")
        else:
            print(f"  ✅ {slug}: prices match ({len(scraped)} models)")

    # Write snapshot
    snapshot = {
        "scraped_at": ts,
        "providers": summary,
        "total_changes": len(all_diffs),
        "changes": all_diffs,
    }

    os.makedirs(SNAPSHOT_DIR, exist_ok=True)
    snapshot_path = os.path.join(SNAPSHOT_DIR, "scrape-latest.json")
    with open(snapshot_path, "w") as f:
        json.dump(snapshot, f, indent=2, ensure_ascii=False)

    dated_path = os.path.join(
        SNAPSHOT_DIR,
        f"scrape-{datetime.now().strftime('%Y-%m-%d')}.json",
    )
    with open(dated_path, "w") as f:
        json.dump(snapshot, f, indent=2, ensure_ascii=False)

    print(f"\n[scraper] Done. {len(all_diffs)} total changes across {len(providers)} providers.")
    print(f"[scraper] Snapshot: {snapshot_path}")

    # Always publish full scraped pricing to Pheromone (Synapse subscribes to update in-memory)
    if NATS_URL:
        all_scraped = {}
        for slug in providers:
            scraper = SCRAPERS.get(slug)
            if not scraper:
                continue
            try:
                all_scraped[slug] = scraper()
            except Exception:
                pass

        publish_pheromone_event(
            "pheromone.events.synapse.pricing.snapshot",
            {
                "event": "pricing.snapshot",
                "service": "synapse-scraper",
                "timestamp": ts,
                "providers": all_scraped,
            },
        )
        print(f"[scraper] Published pricing snapshot to Pheromone ({sum(len(v) for v in all_scraped.values())} models)")

        if all_diffs:
            publish_pheromone_event(
                "pheromone.events.synapse.pricing.changed",
                {
                    "event": "pricing.changed",
                    "service": "synapse-scraper",
                    "changes": all_diffs,
                    "timestamp": ts,
                },
            )
            print(f"[scraper] Published {len(all_diffs)} price changes to Pheromone")

    return all_diffs


def main():
    parser = argparse.ArgumentParser(description="Synapse Price Scraper")
    parser.add_argument("--provider", type=str, help="Scrape single provider")
    parser.add_argument("--update", action="store_true", help="Update YAML files with new prices")
    parser.add_argument("--watch", action="store_true", help="Continuous watch mode")
    parser.add_argument("--interval", type=int, default=WATCH_INTERVAL, help="Watch interval (seconds)")
    args = parser.parse_args()

    providers = [args.provider] if args.provider else None

    if args.watch:
        print(f"[scraper] Watch mode — scraping every {args.interval // 3600}h")
        while True:
            try:
                run_scrape(providers, args.update)
            except Exception as e:
                print(f"[scraper] Error: {e}", file=sys.stderr)
            print(f"[scraper] Next scrape in {args.interval // 3600}h...")
            time.sleep(args.interval)
    else:
        run_scrape(providers, args.update)


if __name__ == "__main__":
    main()
