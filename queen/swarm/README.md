# Queen Swarm — 虫群管理服务

> 虫群通信协议的服务端实现，管理所有节点的注册、心跳、任务调度

## 计划功能

- 节点注册（POST /swarm/register）— Claw/Overlord 首次接入
- 心跳管理（POST /swarm/heartbeat）— 健康监控、负载采集
- 配置分发（GET /swarm/config）— 模型列表、策略下发
- 任务路由（POST /swarm/task/assign）— 将请求分配到最优节点
- 更新通知（POST /swarm/update/notify）— Molt 蜕皮推送
- 节点证书颁发（Spine 脊刺认证）— mTLS 证书管理
- 全网版本地图 — 追踪所有节点的版本状态
- 节点自动发现 & 拓扑维护
