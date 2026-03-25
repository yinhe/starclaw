# Pheromone MVP Runbook

本手册用于把 `PHEROMONE_ESB.md` 的架构方案落到可执行步骤，覆盖：

1. Server C 数据盘初始化
2. Nydus 批量建仓（Polyrepo）
3. Pheromone MVP（NATS JetStream）启动

---

## 0. 前置条件

- Server C 已可 sudo/root 登录
- 已安装 Docker / Docker Compose
- Nydus 机器具备 `git` 用户（默认 `git:git`）

---

## 1) 数据盘初始化（/data）

脚本：`@scripts/serverc-init-data-disk.sh`

```bash
# 在仓库根目录执行（Server C）
sudo bash scripts/serverc-init-data-disk.sh /dev/vdb /data
```

执行结果：

- 自动识别 /dev/vdb 文件系统（无则格式化 ext4）
- 挂载到 `/data`
- 写入 `/etc/fstab`
- 创建目录：
  - `/data/mysql`
  - `/data/redis`
  - `/data/nydus/repos`
  - `/data/nydus/releases`
  - `/data/pheromone/nats`
  - `/data/pheromone/logs`
  - `/data/backups`
  - `/data/logs`

---

## 2) Nydus 批量建仓（Polyrepo）

脚本：`@scripts/nydus-init-polyrepos.sh`

```bash
# 推荐以 git 用户执行，保证 owner 正确
sudo -u git bash scripts/nydus-init-polyrepos.sh /data/nydus/repos
```

会初始化以下 bare repos：

- starclaw.git
- claw.git
- queen.git
- synapse.git
- overlord.git
- forge.git
- hive.git
- pheromone.git
- cerebrate.git
- larva.git
- spore.git
- carapace.git
- nydus.git

---

## 3) 启动 Pheromone MVP（NATS + Exporter）

模板：`@scripts/docker-compose.pheromone-mvp.yml`

```bash
docker compose -f scripts/docker-compose.pheromone-mvp.yml up -d
```

验证：

```bash
# NATS 健康
curl -sS http://127.0.0.1:8222/healthz

# Prometheus exporter
curl -sS http://127.0.0.1:7777/metrics | head
```

---

## 4) 下一步建议（接入业务）

1. Queen/Synapse 增加 `PHEROMONE_NATS_URL=nats://<host>:4222`
2. 先迁移支付链路事件：
   - `pheromone.events.order.created`
   - `pheromone.events.order.paid`
3. Forge 监听 `pheromone.events.deploy.*` 作为 CI/CD 事件源

---

## 5) 回滚方案

```bash
docker compose -f scripts/docker-compose.pheromone-mvp.yml down
```

如需停用数据盘挂载，请先业务停机并迁移数据，再手动修改 `/etc/fstab`。
