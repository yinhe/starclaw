#!/bin/bash
# StarClaw 🦞 One-Click Install (China)
# Usage: curl -fsSL https://raw.githubusercontent.com/yinhe/starclaw/main/scripts/install-cn.sh | bash

set -e

# Force China mirror mode in the main installer
STARCLAW_USE_CN=true bash <(curl -fsSL https://raw.githubusercontent.com/yinhe/starclaw/main/scripts/install.sh)
