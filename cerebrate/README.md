# Cerebrate — 🧠 脑虫（合伙人生态系统）

> 虫族中的脑虫（Cerebrate）是独立的指挥单位，各自统领一支虫群（Brood），向虫后（Queen）报告但拥有自主意志。

---

## 一、定位

```
脑虫 = 合伙人运营体系
  → 管作战（拓客/运营/部署），不管资源（资金由虫后管）
  → 有领地（城市/区域），有部下（城市合伙人）
  → 向虫后报告业绩，从虫后获取资源
```

**注意区分：**

| 角色 | 归属 | 核心行为 |
|------|------|---------|
| 战略合伙人 | cerebrate/partner | 管区域、Deals、节点部署、股权 |
| 城市合伙人 | cerebrate/city | 管客户、赚佣金、推物料 |
| 投资人 | queen/investor | 出资、看报表、被动回报 |

投资人由虫后直管（资金流是虫后核心事务），不在脑虫体系内。

---

## 二、模块结构

```
cerebrate/
├── partner/          战略合伙人前端 (React + Vite + Tailwind)
│   ├── src/
│   │   ├── pages/
│   │   │   ├── DashboardPage   大盘（业绩/佣金/节点）
│   │   │   ├── DealsPage       商机管理
│   │   │   ├── CitiesPage      城市合伙人管理
│   │   │   ├── NodesPage       节点部署管理
│   │   │   ├── CommissionsPage  佣金明细
│   │   │   ├── EquityPage      股权/分红
│   │   │   └── DeployPage      部署工具
│   │   ├── lib/api.ts          API 客户端
│   │   └── components/Layout   侧边栏布局
│   ├── Dockerfile
│   └── nginx.conf
│
├── city/             城市合伙人前端 (React + Vite + Tailwind)
│   ├── src/
│   │   ├── pages/
│   │   │   ├── DashboardPage   大盘（客户/佣金）
│   │   │   ├── ClientsPage     客户管理
│   │   │   ├── CommissionsPage  佣金明细
│   │   │   ├── MaterialsPage   推广物料
│   │   │   └── ClientStatsPage  客户统计
│   │   ├── lib/api.ts          API 客户端
│   │   └── components/Layout   侧边栏布局
│   ├── Dockerfile
│   └── nginx.conf
│
└── README.md         本文件
```

---

## 三、后端 API

后端 API 位于 `queen/api/`（共享虫后数据库）：

| 文件 | 内容 |
|------|------|
| `queen/api/handler/city.go` | 城市合伙人 API (14 endpoints) |
| `queen/api/model/city.go` | 数据模型 (CityPartner, CityClient, Commission, Payout, MarketingMaterial) |

**为什么不独立后端？**
- 合伙人数据与 Queen 核心数据强耦合（用户/订单/佣金/余额）
- 共享 Queen MySQL，避免跨服务数据同步
- 未来如果合伙人体量增大，可以拆出独立微服务

---

## 四、部署

两个前端作为独立 Docker 服务运行：

| 服务 | 端口 | 容器名 |
|------|:----:|--------|
| partner | 8088 | starclaw-queen-partner |
| city | 8087 | starclaw-queen-city |

编排配置在 `queen/docker-compose.yml` 中（build context 指向 `../cerebrate/`）。

---

## 五、技术栈

- **框架**: React 19 + TypeScript
- **构建**: Vite 8
- **样式**: TailwindCSS v4
- **路由**: React Router 7
- **图标**: lucide-react
- **部署**: Dockerfile → nginx 静态托管 + `/api` 反代 queen-api
