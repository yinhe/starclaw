# Overlord Console 👁️ — 领主管理控制台

> Web UI，企业管理员管理下属 Claw 虫群节点的可视化界面

## 已实现功能

- **总览仪表盘** — 节点总数、在线/失控/离线统计、平均 CPU/内存、运行任务、Token 消耗、团队分布
- **节点管理** — Claw 列表（按状态/团队过滤）、节点详情、配额管理（并发数/每日 Token 上限）、移除节点
- **审计日志** — 所有管理操作记录（注册/移除/配额更新/任务分配），含操作人和时间
- **地址解析** — 通过 Claw ID (Ed25519) 解析节点网络地址

## 技术栈

- React 18 + TypeScript
- Vite 6 (构建 + 开发服务器)
- TailwindCSS 3 (深色主题 + 自定义 overlord 色板)
- Lucide React (图标)
- React Router 6 (客户端路由)

## 开发

```bash
cd overlord/console
npm install
npm run dev     # http://localhost:3095
npm run build   # 输出到 dist/
```

开发模式会自动将 `/brood/*` 代理到 `http://localhost:8095`（Manager API）。

## Docker 部署

```bash
cd overlord
docker compose up -d
```

- Console: `http://localhost:3095`
- Manager API: `http://localhost:8095`
- MySQL: `localhost:3308`

## 目录结构

```
console/
├── src/
│   ├── api/brood.ts          # Brood API 客户端
│   ├── pages/
│   │   ├── DashboardPage.tsx  # 总览仪表盘
│   │   ├── ClawsPage.tsx      # 节点列表
│   │   ├── ClawDetailPage.tsx # 节点详情 + 配额
│   │   ├── AuditPage.tsx      # 审计日志
│   │   └── ResolvePage.tsx    # 地址解析
│   ├── App.tsx               # 路由 + 侧边栏布局
│   ├── main.tsx              # 入口
│   └── index.css             # TailwindCSS
├── Dockerfile                # 多阶段构建 (node → nginx)
├── nginx.conf                # 反向代理配置
└── package.json
```
