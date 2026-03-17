# Spore（孢子）— StarClaw Ultra-Lightweight Deployment System

> 让 Claw 像孢子一样，在任何设备上落地即萌发

## 概述

Spore 是 Docker 的轻量替代方案，专为 Claw 在异构设备上的部署而设计。用户可自由选择 Docker 或 Spore。

| 维度 | Docker | Spore |
|------|--------|-------|
| 运行时大小 | ~200MB | < 3MB |
| 网络要求 | 必须访问 Registry | P2P + 离线安装 |
| 支持平台 | Linux (native) | Linux/Mac/Win/Android/OpenWrt |
| 启动速度 | 秒级 | 毫秒级 |
| 更新方式 | 拉全新 Image | Delta patch（差量更新） |

## 快速开始

```bash
# 安装 Spore 运行时
go install github.com/yinhe/starclaw-spore/cmd/spore@latest

# 安装 .spore 包
spore install ./claw-v1.0.0-linux-amd64.spore

# 启动
spore start claw

# 查看状态
spore status

# 查看日志
spore logs claw
```

## 构建 Spore 包

```bash
# 安装 Hatchery 构建工具
go install github.com/yinhe/starclaw-spore/cmd/hatchery@latest

# 构建当前平台
hatchery build

# 构建所有平台
hatchery build --all

# 输出到指定目录
hatchery build --platform linux/arm64 --output release/
```

## 虫族命名体系

| 组件 | 虫族名 | 功能 |
|------|--------|------|
| **Spore** | 孢子 | 超轻量运行时 + 包格式 |
| **Hatchery** | 孵化场 | 构建工具 + 本地仓库 |
| **Creep** | 菌毯 | 设备集群管理 |
| **Nydus** | 虫洞 | P2P 分发网络 |
| **Queen** | 女王 | 中心控制台 |

## 项目结构

```
spore/
├── cmd/
│   ├── spore/       # Spore CLI (运行时)
│   └── hatchery/    # Hatchery CLI (构建工具)
├── pkg/
│   ├── manifest/    # manifest.json 解析
│   ├── archive/     # .spore 包打包/解包
│   ├── runtime/     # 进程管理 + 服务注册
│   └── platform/    # 平台检测 + 自适应
└── docs/
    └── SPORE_PLAN.md
```

## 文档

详细设计文档见 [SPORE_PLAN.md](docs/SPORE_PLAN.md)
