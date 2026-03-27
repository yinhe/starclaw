"""
🐝 Drone Worker — 定时调度采集 + 虫茧同化 + 导入
运行方式: python -m scripts.worker
"""

import json
import os
import sys
import time
import requests
import yaml
from datetime import datetime

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from collectors.clawhub import ClawHubCollector
from collectors.skillhub import SkillHubCollector
from collectors.awesome_gpts import AwesomeGPTsCollector
from collectors.coze import CozeCollector
from collectors.dify import DifyCollector
from collectors.gpt_prompts import GPTPromptsCollector
from cocoon.auto_morph import morph_batch
from cocoon.llm_evolve import evolve_batch

# Collector registry
COLLECTORS = {
    "gpt_prompts": GPTPromptsCollector,
    "clawhub": ClawHubCollector,
    "skillhub": SkillHubCollector,
    "awesome_gpts": AwesomeGPTsCollector,
    "coze": CozeCollector,
    "dify": DifyCollector,
}


def load_config():
    config_path = os.getenv("DRONE_CONFIG", "/app/configs/sources.yaml")
    if not os.path.exists(config_path):
        config_path = os.path.join(os.path.dirname(os.path.dirname(__file__)), "configs", "sources.yaml")
    with open(config_path, "r", encoding="utf-8") as f:
        return yaml.safe_load(f)


def harvest_source(source_name: str, source_config: dict, global_config: dict):
    """采集单个数据源 → 虫茧同化 → 导入。"""
    print(f"\n{'='*60}")
    print(f"🐝 Harvesting: {source_name} ({source_config.get('name', '')})")
    print(f"{'='*60}")

    start = time.time()

    # Step 1: 采集
    collector_cls = COLLECTORS.get(source_name)
    if not collector_cls:
        print(f"[worker] unknown source: {source_name}, skipping")
        return

    collector = collector_cls(source_config)
    mode = source_config.get("mode", "incremental")
    raw_items = collector.collect(mode=mode)
    print(f"[worker] collected {len(raw_items)} raw items from {source_name}")

    if not raw_items:
        return

    # Step 2: L1 自动变态
    templates = morph_batch(raw_items, source_name, global_config)
    print(f"[worker] L1 auto-morph: {len(raw_items)} → {len(templates)} templates")

    # Step 3: L2 LLM 进化（仅对高质量的）
    l2_threshold = global_config.get("quality", {}).get("l2_threshold", 80)
    starai_api = os.getenv("DRONE_STARAI_API", global_config.get("import", {}).get("starci_api", ""))
    starai_key = os.getenv("DRONE_STARAI_KEY", "")

    if starai_api and starai_key:
        l2_count = sum(1 for t in templates if t.get("_quality_score", 0) >= l2_threshold)
        if l2_count > 0:
            print(f"[worker] L2 LLM-evolve: {l2_count} templates qualify (score >= {l2_threshold})")
            templates = evolve_batch(templates, starai_api, starai_key, l2_threshold)

    # Step 4: 导入到 Claw API
    claw_api = os.getenv("DRONE_CLAW_API", global_config.get("import", {}).get("claw_api", "http://localhost:8080"))
    imported = import_to_marketplace(templates, claw_api)

    duration = time.time() - start
    print(f"[worker] ✅ {source_name}: {len(raw_items)} collected → {len(templates)} morphed → {imported} imported ({duration:.1f}s)")


def import_to_marketplace(templates: list, claw_api: str) -> int:
    """批量导入 AgentTemplate 到 Claw 市场。"""
    if not templates:
        return 0

    batch_size = 50
    imported = 0

    for i in range(0, len(templates), batch_size):
        batch = templates[i:i + batch_size]

        # Clean internal fields before sending
        clean = []
        for t in batch:
            item = {k: v for k, v in t.items() if not k.startswith("_")}
            item["is_builtin"] = False
            item["author_id"] = "system"
            clean.append(item)

        try:
            drone_secret = os.getenv("DRONE_SECRET", "")
            resp = requests.post(
                f"{claw_api}/v1/marketplace/import",
                json={"templates": clean, "source": clean[0].get("_source", "drone")},
                headers={"Content-Type": "application/json", "X-Drone-Secret": drone_secret},
                timeout=30,
            )
            if resp.status_code in (200, 201):
                result = resp.json()
                imported += result.get("imported", len(clean))
            else:
                print(f"[worker] import batch failed: HTTP {resp.status_code} - {resp.text[:200]}")
        except requests.RequestException as e:
            print(f"[worker] import request failed: {e}")

    return imported


def run_all(config: dict):
    """运行所有已配置的采集源。"""
    sources = config.get("sources", {})
    for name, src_config in sorted(sources.items(), key=lambda x: x[1].get("priority", 99)):
        try:
            harvest_source(name, src_config, config)
        except Exception as e:
            print(f"[worker] ❌ {name} failed: {e}")
            import traceback
            traceback.print_exc()


def run_one(source_name: str, config: dict):
    """运行单个采集源。"""
    src_config = config.get("sources", {}).get(source_name)
    if not src_config:
        print(f"[worker] source '{source_name}' not found in config")
        return
    harvest_source(source_name, src_config, config)


if __name__ == "__main__":
    config = load_config()

    if len(sys.argv) > 1:
        # 指定单个源: python -m scripts.worker clawhub
        run_one(sys.argv[1], config)
    else:
        # 运行所有
        print(f"🐝 Drone Worker starting at {datetime.utcnow().isoformat()}")
        run_all(config)
        print(f"\n🐝 Drone Worker finished at {datetime.utcnow().isoformat()}")
