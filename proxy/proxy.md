
# 调试时如何监控 `proxy` 日志（最实用的几种方式）

你现在是用 **PM2** 跑的进程名 `proxy`，对外还有 **Nginx**，调试时建议同时看这两类日志：**应用日志**（Node/PM2）+ **入口日志**（Nginx）。

---

## 1) 监控 Node/Proxy 应用日志（PM2）
### 1.0 PM2 常用运维命令（启动/重启/自启）

#### 查看状态 / 详情
```bash
pm2 status
pm2 show proxy
pm2 info proxy
```

#### 启动 / 停止 / 重启
```bash
# 启动（如果当前目录是 proxy 项目根目录）
pm2 start server.js --name proxy

# 停止 / 删除
pm2 stop proxy
pm2 delete proxy

# 重启（最常用）
pm2 restart proxy

# 平滑重载（尽量不中断，适合无状态服务）
pm2 reload proxy
```

#### 更新代码后常用流程（推荐）
```bash
pm2 restart proxy
pm2 logs proxy --lines 200
```

#### 开机自启（只需要做一次）
```bash
pm2 startup

# 执行 pm2 startup 输出的那条命令后，再保存当前进程列表
pm2 save
```

#### 恢复已保存的进程列表（机器重启后/迁移时）
```bash
pm2 resurrect
```

#### 清理日志（日志太大时）
```bash
pm2 flush
```

### 1.1 实时跟踪（最常用）
```bash
pm2 logs proxy
```

### 1.2 只看最近 N 行（适合快速回看）
```bash
pm2 logs proxy --lines 200
```

### 1.3 分开看 stdout / stderr（定位报错更快）
PM2 实际日志文件在（root 用户运行时）：
- `/root/.pm2/logs/proxy-out.log`
- `/root/.pm2/logs/proxy-error.log`

你可以直接 tail：
```bash
tail -f /root/.pm2/logs/proxy-out.log
tail -f /root/.pm2/logs/proxy-error.log
```

### 1.4 只过滤错误/关键词（快速定位）
```bash
pm2 logs proxy --lines 200 | grep -i "error"
pm2 logs proxy --lines 200 | grep -i "redis"
```

---

## 2) 监控 Nginx 日志（排查域名/HTTPS/反代）
### 2.1 实时看访问与错误
```bash
sudo tail -f /var/log/nginx/access.log
sudo tail -f /var/log/nginx/error.log
```

### 2.2 只看某个域名的请求（如果 access.log 太多）
```bash
sudo tail -f /var/log/nginx/access.log | grep "proxy.star-ai.net"
```

> 如果你想更精细（每个站点单独日志），我也可以教你把 `proxy.star-ai.net.conf` 配成单独的 `access_log`/`error_log` 文件。

---

## 3) 调试链路推荐“同时开两个窗口”
- **窗口 A**：`pm2 logs proxy`
- **窗口 B**：`sudo tail -f /var/log/nginx/error.log`

然后你从本地发请求：
```bash
curl -I https://proxy.star-ai.net/
# 或携带 X-API-KEY 调你们的接口
```

这样能立刻知道：
- 请求有没有到 Nginx
- Nginx 有没有转发到 3000
- Node 端是 401/500/超时还是正常响应

---

## 4) 常用“健康排查命令”
- **看 PM2 进程状态**
```bash
pm2 status
pm2 show proxy
```

- **看端口监听（3000 / 80 / 443）**
```bash
sudo ss -lntp | egrep ':(3000|80|443)\s'
```

---

## 5) 一个重要提醒：你现在用 root 跑 PM2
所以日志路径在 `/root/.pm2/logs/`。  
如果你后续改成普通用户运行，日志路径会变成 `/home/<user>/.pm2/logs/`。

另外注意：
- **PM2 的进程列表是按用户隔离的**。你用 root 跑，就要用 root 去 `pm2 status/logs`。
- 如果你改成普通用户跑，建议配套把 Nginx/权限/数据目录一起对齐。

---

# 状态总结
- 监控 `proxy` 本身：用 `pm2 logs proxy`（或 tail `/root/.pm2/logs/*`）。
- 监控入口/HTTPS/反代：看 `/var/log/nginx/access.log` 和 `error.log`。
- 调试时建议两个窗口同时开，最快定位问题。