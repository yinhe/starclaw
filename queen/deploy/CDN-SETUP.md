# StarClaw 国内 CDN 加速配置指南

## 一、架构概览

```
用户 → CDN 边缘节点 → 源站 (Nginx + Docker)
         ↓ 缓存命中
       直接返回
```

**三个域名各自接入 CDN：**

| 域名 | 用途 | 源站 | 推荐 CDN |
|------|------|------|---------|
| `starclaw.me` | 官网 + Claw 应用 | 原服务器 IP | 阿里云 CDN / Cloudflare |
| `starclaw.net` | Queen 管理 + 市场 | 原服务器 IP | 阿里云 CDN / Cloudflare |
| `star-ai.net` | AI Gateway + Dashboard | 原服务器 IP | 阿里云 CDN / Cloudflare |

## 二、缓存策略

### 2.1 静态资源（由 Vite 构建）

所有前端使用 Vite 构建，输出到 `/assets/` 目录，文件名包含内容 hash：
```
/assets/index-a1b2c3d4.js
/assets/index-e5f6g7h8.css
```

**缓存规则：**
- `/assets/*` → `Cache-Control: public, max-age=31536000, immutable` (1 年)
- `/index.html` → `Cache-Control: no-cache, no-store, must-revalidate` (不缓存)
- 图片/字体 → `expires 7d` (7 天)

### 2.2 API 请求

API 路径不应缓存：
- `/v1/*`, `/api/*`, `/auth/*`, `/admin/*`, `/pay/*` → **不缓存**
- WebSocket `/v1/ws` → **不缓存，透传**

## 三、CDN 配置步骤

### 3.1 阿里云 CDN

1. **添加加速域名**
   - 登录阿里云 CDN 控制台
   - 添加域名：`starclaw.me`、`starclaw.net`、`star-ai.net`
   - 源站类型：IP，填写服务器公网 IP
   - 端口：443 (HTTPS 回源)

2. **HTTPS 配置**
   - 上传或自动申请 SSL 证书
   - 开启 HTTP/2
   - 开启 HSTS (可选)

3. **缓存规则**
   ```
   /assets/    → 缓存 365 天
   /index.html → 不缓存
   *.js *.css  → 缓存 365 天 (命中 /assets/ 路径)
   /v1/*       → 不缓存
   /api/*      → 不缓存
   /pay/*      → 不缓存
   默认        → 遵循源站 Cache-Control
   ```

4. **性能优化**
   - 开启 Gzip 压缩
   - 开启 Brotli 压缩 (如支持)
   - 开启 HTTP/2
   - 开启智能压缩

5. **DNS 修改**
   - 将域名 CNAME 解析到阿里云 CDN 分配的 CNAME 地址

### 3.2 Cloudflare (国际 + 国内)

1. **添加站点** → 选 Pro 或 Business Plan (支持中国网络)
2. **SSL/TLS** → Full (Strict)
3. **缓存规则** (Page Rules):
   ```
   *starclaw.me/assets/*   → Cache Level: Cache Everything, Edge TTL: 1 year
   *starclaw.me/v1/*       → Cache Level: Bypass
   *starclaw.me/index.html → Cache Level: Bypass
   ```
4. **Speed 设置**:
   - Auto Minify: JS + CSS + HTML
   - Brotli: On
   - Early Hints: On
5. **DNS**: Proxy (橙色云朵) 开启

### 3.3 腾讯云 CDN

配置逻辑与阿里云相同，注意：
- 回源 Host 填写原始域名
- 缓存规则中 `/assets/` 设为 365 天
- API 路径设为不缓存

## 四、环境变量

前端构建时可通过环境变量指定 CDN 资源前缀：

```bash
# .env.production
VITE_CDN_BASE_URL=https://cdn.starclaw.me
```

在 `vite.config.ts` 中使用：
```typescript
export default defineConfig({
  base: process.env.VITE_CDN_BASE_URL || '/',
})
```

> 注意：仅当使用独立 CDN 域名时需要此配置。如果使用 Cloudflare 等全站代理 CDN，则 base 保持 `/` 即可。

## 五、验证清单

- [ ] `curl -I https://starclaw.me/assets/xxx.js` 检查 `Cache-Control: immutable`
- [ ] `curl -I https://starclaw.me/index.html` 检查 `Cache-Control: no-cache`
- [ ] `curl -I https://starclaw.me/` 检查 `Content-Encoding: gzip`
- [ ] `curl -I https://starclaw.me/` 检查 `X-Content-Type-Options: nosniff`
- [ ] CDN 控制台确认缓存命中率 > 80%
- [ ] 国内多地 ping 测试延迟 < 50ms
- [ ] WebSocket 连接正常 (app.starclaw.me)
- [ ] API 请求不被缓存 (POST/PUT/DELETE)

## 六、回滚

如需关闭 CDN：
1. DNS 解析改回源站 IP
2. TTL 等待生效 (通常 5-10 分钟)
3. 源站 Nginx 配置无需修改（已内置完整缓存头）
