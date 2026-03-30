"""
Account Manager — manages multiple QMT test accounts.
"""

import logging
import yaml
import os
from typing import Dict, List

logger = logging.getLogger("account_manager")


class AccountManager:
    """Manages the pool of QMT trading accounts."""

    def __init__(self, qmt_client):
        self.qmt = qmt_client
        self.accounts: Dict[str, dict] = {}
        self._load_config()

    def _load_config(self):
        config_path = os.path.join(os.path.dirname(__file__), "config.yaml")
        if os.path.exists(config_path):
            with open(config_path, "r", encoding="utf-8") as f:
                cfg = yaml.safe_load(f)
            for acc in cfg.get("accounts", []):
                self.accounts[acc["id"]] = acc
            logger.info(f"Loaded {len(self.accounts)} accounts from config")
        else:
            logger.warning("config.yaml not found, using defaults")

    def get_account(self, account_id: str) -> dict:
        return self.accounts.get(account_id, {})

    def get_accounts_by_group(self, group: str) -> List[dict]:
        return [a for a in self.accounts.values() if a.get("group") == group]

    def get_all_accounts(self) -> List[dict]:
        return list(self.accounts.values())

    def refresh_balances(self):
        """Refresh balance info for all accounts from QMT."""
        for acc_id in self.accounts:
            try:
                info = self.qmt.get_account_info(acc_id)
                self.accounts[acc_id].update(info)
            except Exception as e:
                logger.error(f"Failed to refresh {acc_id}: {e}")
